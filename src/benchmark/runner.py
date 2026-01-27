"""Orchestrates benchmark runs for SWE-bench instances."""

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
    ) -> None:
        """Initialize operations with server URL and paths."""
        self._base_url = kodit_base_url
        self._repos_dir = repos_dir
        self._results_dir = results_dir
        self._model = model
        self._api_key = api_key
        self._top_k = top_k
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
        generator = PatchGenerator(model=self._model, api_key=self._api_key)

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
