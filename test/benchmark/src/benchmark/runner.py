"""Orchestrates benchmark runs for SWE-bench instances."""

from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

import structlog

from benchmark.server import ServerProcess
from benchmark.swebench.evaluator import (
    EvaluationError,
    EvaluationResult,
    Evaluator,
)
from benchmark.swebench.generator import BenchmarkRunner, PatchGenerator
from benchmark.swebench.instance import SWEBenchInstance
from benchmark.swebench.prediction import Prediction, PredictionWriter
from benchmark.swebench.repository import RepositoryPreparer
from benchmark.swebench.retriever import KoditRetriever


@dataclass(frozen=True)
class ComparisonResult:
    """Result comparing baseline vs kodit-augmented predictions."""

    instance_id: str
    baseline_prediction: Prediction
    kodit_prediction: Prediction
    baseline_evaluation: EvaluationResult | None
    kodit_evaluation: EvaluationResult | None

    @property
    def baseline_resolved(self) -> bool:
        """Whether baseline prediction resolved the issue."""
        if self.baseline_evaluation is None:
            return False
        return self.baseline_evaluation.resolved > 0

    @property
    def kodit_resolved(self) -> bool:
        """Whether kodit prediction resolved the issue."""
        if self.kodit_evaluation is None:
            return False
        return self.kodit_evaluation.resolved > 0

    def as_dict(self) -> dict:
        """Convert to dictionary for JSON serialization."""
        return {
            "instance_id": self.instance_id,
            "baseline_resolved": self.baseline_resolved,
            "kodit_resolved": self.kodit_resolved,
            "baseline_prediction": {
                "instance_id": self.baseline_prediction.instance_id,
                "model_name_or_path": self.baseline_prediction.model_name_or_path,
                "patch_length": len(self.baseline_prediction.model_patch),
            },
            "kodit_prediction": {
                "instance_id": self.kodit_prediction.instance_id,
                "model_name_or_path": self.kodit_prediction.model_name_or_path,
                "patch_length": len(self.kodit_prediction.model_patch),
            },
            "baseline_evaluation": (
                self.baseline_evaluation.as_dict()
                if self.baseline_evaluation
                else None
            ),
            "kodit_evaluation": (
                self.kodit_evaluation.as_dict() if self.kodit_evaluation else None
            ),
        }


class BenchmarkOperations:
    """Reusable benchmark operations for an already-running Kodit server."""

    def __init__(  # noqa: PLR0913
        self,
        kodit_base_url: str,
        repos_dir: Path,
        results_dir: Path,
        model: str,
        api_key: str | None = None,
        top_k: int = 10,
        llm_timeout: int = 60,
    ) -> None:
        """Initialize operations with server URL and paths."""
        self._base_url = kodit_base_url
        self._repos_dir = repos_dir
        self._results_dir = results_dir
        self._model = model
        self._api_key = api_key
        self._top_k = top_k
        self._llm_timeout = llm_timeout
        self._log = structlog.get_logger(__name__)

    def prepare(self, instance: SWEBenchInstance) -> int:
        """Prepare an instance by cloning and indexing."""
        self._log.info("Preparing instance", instance_id=instance.instance_id)
        preparer = RepositoryPreparer(
            kodit_base_url=self._base_url,
            repos_dir=self._repos_dir,
        )
        return preparer.prepare(instance)

    def generate(
        self,
        instance: SWEBenchInstance,
        condition: str,
    ) -> Prediction:
        """Generate a prediction for baseline or kodit condition."""
        generator = PatchGenerator(
            model=self._model,
            api_key=self._api_key,
            timeout=self._llm_timeout,
        )

        if condition == "baseline":
            self._log.info("Generating baseline prediction")
            return generator.generate_baseline(instance)

        retriever = KoditRetriever(kodit_base_url=self._base_url)
        runner = BenchmarkRunner(generator=generator, retriever=retriever)

        self._log.info("Generating kodit prediction", top_k=self._top_k)
        return runner.run_kodit(instance, top_k=self._top_k)

    def generate_and_write(
        self,
        instance: SWEBenchInstance,
        condition: str,
        output_path: Path | None = None,
    ) -> tuple[Prediction, Path]:
        """Generate a prediction and write to file."""
        prediction = self.generate(instance, condition)

        if output_path is None:
            self._results_dir.mkdir(parents=True, exist_ok=True)
            output_path = self._results_dir / f"{condition}.jsonl"

        PredictionWriter(output_path).write(prediction)
        self._log.info("Prediction written", output=str(output_path))

        return prediction, output_path

    def evaluate(
        self,
        predictions_path: Path,
        run_id: str,
        max_workers: int = 4,
    ) -> EvaluationResult:
        """Evaluate predictions using SWE-bench harness."""
        self._log.info("Evaluating predictions", path=str(predictions_path))
        evaluator = Evaluator()
        return evaluator.evaluate_full(
            predictions_path=predictions_path,
            run_id=run_id,
            max_workers=max_workers,
        )

    def run_comparison(
        self,
        instance: SWEBenchInstance,
        *,
        skip_evaluation: bool = False,
    ) -> ComparisonResult:
        """Run both conditions and compare results."""
        # Generate predictions
        baseline_prediction = self.generate(instance, "baseline")
        kodit_prediction = self.generate(instance, "kodit")

        # Write predictions
        self._results_dir.mkdir(parents=True, exist_ok=True)
        baseline_path = self._results_dir / f"{instance.instance_id}.baseline.jsonl"
        kodit_path = self._results_dir / f"{instance.instance_id}.kodit.jsonl"

        PredictionWriter(baseline_path).write(baseline_prediction)
        PredictionWriter(kodit_path).write(kodit_prediction)

        self._log.info(
            "Predictions written",
            baseline=str(baseline_path),
            kodit=str(kodit_path),
        )

        # Evaluate if requested
        baseline_evaluation = None
        kodit_evaluation = None

        if not skip_evaluation:
            try:
                baseline_evaluation = self.evaluate(
                    baseline_path,
                    run_id=f"{instance.instance_id}_baseline",
                )
            except EvaluationError as e:
                self._log.warning("Baseline evaluation failed", error=str(e))

            try:
                kodit_evaluation = self.evaluate(
                    kodit_path,
                    run_id=f"{instance.instance_id}_kodit",
                )
            except EvaluationError as e:
                self._log.warning("Kodit evaluation failed", error=str(e))

        return ComparisonResult(
            instance_id=instance.instance_id,
            baseline_prediction=baseline_prediction,
            kodit_prediction=kodit_prediction,
            baseline_evaluation=baseline_evaluation,
            kodit_evaluation=kodit_evaluation,
        )


class InstanceRunner:
    """Runs a complete benchmark with server lifecycle management."""

    def __init__(
        self,
        server: ServerProcess,
        operations: BenchmarkOperations,
    ) -> None:
        """Initialize runner with server and operations."""
        self._server = server
        self._operations = operations
        self._log = structlog.get_logger(__name__)

    def run(
        self,
        instance: SWEBenchInstance,
        *,
        skip_evaluation: bool = False,
    ) -> ComparisonResult:
        """Run the full benchmark workflow for an instance."""
        self._log.info(
            "Starting benchmark run",
            instance_id=instance.instance_id,
            repo=instance.repo,
        )

        # Start server
        self._log.info("Starting kodit server")
        if not self._server.start():
            msg = "Failed to start kodit server"
            raise InstanceRunError(msg)

        try:
            # Prepare and run comparison
            self._operations.prepare(instance)
            return self._operations.run_comparison(
                instance,
                skip_evaluation=skip_evaluation,
            )
        finally:
            # Stop server
            self._log.info("Stopping kodit server")
            self._server.stop()


class InstanceRunError(Exception):
    """Raised when an instance run fails."""


@dataclass(frozen=True)
class BatchResult:
    """Result of running benchmarks for multiple instances."""

    total: int
    succeeded: int
    failed: int
    failures: list[str]

    def as_dict(self) -> dict:
        """Convert to dictionary for JSON serialization."""
        return {
            "total": self.total,
            "succeeded": self.succeeded,
            "failed": self.failed,
            "failures": self.failures,
        }


@dataclass(frozen=True)
class BatchConfig:
    """Configuration for batch benchmark runs."""

    repos_dir: Path
    results_dir: Path
    model: str
    api_key: str
    top_k: int
    skip_evaluation: bool
    host: str
    port: int
    enrichment_base_url: str
    enrichment_model: str
    enrichment_parallel_tasks: int
    enrichment_timeout: int
    embedding_base_url: str
    embedding_model: str
    embedding_parallel_tasks: int
    embedding_timeout: int
    llm_timeout: int = 60


class BatchRunner:
    """Runs benchmarks for multiple SWE-bench instances sequentially."""

    def __init__(self, config: BatchConfig) -> None:
        """Initialize with configuration."""
        self._config = config
        self._log = structlog.get_logger(__name__)

    def run(
        self,
        instances: list[SWEBenchInstance],
        on_instance_complete: Callable[[str, bool, int, int], None] | None = None,
    ) -> BatchResult:
        """Run benchmarks for all instances sequentially."""
        total = len(instances)
        self._log.info("Starting batch benchmark run", total=total)

        succeeded = 0
        failed = 0
        failures: list[str] = []

        for i, instance in enumerate(instances, start=1):
            instance_id = instance.instance_id
            self._log.info(
                "Running instance",
                progress=f"{i}/{total}",
                instance_id=instance_id,
            )

            success = self._run_single(instance)

            if success:
                succeeded += 1
            else:
                failed += 1
                failures.append(instance_id)

            if on_instance_complete:
                on_instance_complete(instance_id, success, i, total)

        self._log.info(
            "Batch benchmark complete",
            total=total,
            succeeded=succeeded,
            failed=failed,
        )

        return BatchResult(
            total=total,
            succeeded=succeeded,
            failed=failed,
            failures=failures,
        )

    def _run_single(self, instance: SWEBenchInstance) -> bool:
        """Run benchmark for a single instance. Returns True on success."""
        server = ServerProcess(
            host=self._config.host,
            port=self._config.port,
            enrichment_base_url=self._config.enrichment_base_url,
            enrichment_model=self._config.enrichment_model,
            enrichment_api_key=self._config.api_key,
            enrichment_parallel_tasks=self._config.enrichment_parallel_tasks,
            enrichment_timeout=self._config.enrichment_timeout,
            embedding_base_url=self._config.embedding_base_url,
            embedding_model=self._config.embedding_model,
            embedding_api_key=self._config.api_key,
            embedding_parallel_tasks=self._config.embedding_parallel_tasks,
            embedding_timeout=self._config.embedding_timeout,
        )

        operations = BenchmarkOperations(
            kodit_base_url=server.base_url,
            repos_dir=self._config.repos_dir,
            results_dir=self._config.results_dir,
            model=self._config.model,
            api_key=self._config.api_key,
            top_k=self._config.top_k,
            llm_timeout=self._config.llm_timeout,
        )

        runner = InstanceRunner(server=server, operations=operations)

        try:
            result = runner.run(instance, skip_evaluation=self._config.skip_evaluation)
        except InstanceRunError as e:
            self._log.error(
                "Benchmark run failed",
                instance_id=instance.instance_id,
                error=str(e),
            )
            return False
        except TimeoutError as e:
            self._log.error(
                "Benchmark run timed out",
                instance_id=instance.instance_id,
                error=str(e),
            )
            return False
        except Exception as e:  # noqa: BLE001
            self._log.error(
                "Unexpected error during benchmark run",
                instance_id=instance.instance_id,
                error=str(e),
                error_type=type(e).__name__,
            )
            return False

        self._write_comparison(instance.instance_id, result)
        return True

    def _write_comparison(self, instance_id: str, result: ComparisonResult) -> None:
        """Write comparison result to file."""
        import json

        comparison_path = self._config.results_dir / f"{instance_id}.comparison.json"
        comparison_path.parent.mkdir(parents=True, exist_ok=True)
        with comparison_path.open("w") as f:
            json.dump(result.as_dict(), f, indent=2)

        self._log.info(
            "Comparison written",
            instance_id=instance_id,
            path=str(comparison_path),
        )
