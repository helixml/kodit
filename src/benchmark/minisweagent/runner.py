"""Runner for mini-swe-agent benchmarks with Kodit comparison."""

from __future__ import annotations

import json
import subprocess
from dataclasses import dataclass
from typing import TYPE_CHECKING

import structlog

from benchmark.minisweagent.retrieval import KoditContextProvider
from benchmark.runner import BenchmarkOperations
from benchmark.swebench.instance import SWEBenchInstance
from benchmark.swebench.repository import DEFAULT_REPOS_DIR

if TYPE_CHECKING:
    from collections.abc import Callable
    from pathlib import Path

    from benchmark.server import ServerProcess


@dataclass(frozen=True)
class RunConfig:
    """Configuration for a mini-swe-agent run."""

    config_path: Path
    output_dir: Path
    model: str
    repos_dir: Path = DEFAULT_REPOS_DIR
    workers: int = 4
    subset: str = "verified"
    api_key: str | None = None
    stream_output: bool = True
    force_reindex: bool = False


@dataclass
class InstanceStats:
    """Stats for a single instance run."""

    instance_id: str
    exit_status: str
    cost: float
    api_calls: int
    has_patch: bool


@dataclass
class RunResult:
    """Result of a mini-swe-agent benchmark run."""

    predictions_path: Path
    trajectories_dir: Path
    total_instances: int
    completed_instances: int
    condition: str
    instance_stats: list[InstanceStats] | None = None

    @property
    def total_cost(self) -> float:
        """Sum of costs across all instances."""
        if not self.instance_stats:
            return 0.0
        return sum(s.cost for s in self.instance_stats)

    @property
    def total_api_calls(self) -> int:
        """Sum of API calls across all instances."""
        if not self.instance_stats:
            return 0
        return sum(s.api_calls for s in self.instance_stats)

    @property
    def instances_with_patch(self) -> int:
        """Count of instances that produced a non-empty patch."""
        if not self.instance_stats:
            return 0
        return sum(1 for s in self.instance_stats if s.has_patch)


@dataclass
class ComparisonResult:
    """Comparison between baseline and kodit runs."""

    baseline: RunResult
    kodit: RunResult
    baseline_resolved: list[str]
    kodit_resolved: list[str]
    both_resolved: list[str]
    neither_resolved: list[str]


class PreparedInstanceCache:
    """Cache for prepared (augmented) instances with incremental persistence."""

    def __init__(self, cache_path: Path) -> None:
        """Initialize with path to cached JSONL file."""
        self._path = cache_path
        self._log = structlog.get_logger(__name__)

    @property
    def path(self) -> Path:
        """Return cache file path."""
        return self._path

    def prepared_ids(self) -> frozenset[str]:
        """Return set of instance IDs that have been prepared."""
        if not self._path.exists():
            return frozenset()

        ids: set[str] = set()
        with self._path.open() as f:
            for line in f:
                stripped = line.strip()
                if not stripped:
                    continue
                try:
                    record = json.loads(stripped)
                    ids.add(record["instance_id"])
                except (json.JSONDecodeError, KeyError):
                    continue
        return frozenset(ids)

    def load(self) -> list[SWEBenchInstance]:
        """Load all prepared instances from cache."""
        if not self._path.exists():
            return []

        instances = []
        with self._path.open() as f:
            for line in f:
                stripped = line.strip()
                if not stripped:
                    continue
                try:
                    record = json.loads(stripped)
                    fail_to_pass = json.loads(record.get("FAIL_TO_PASS", "[]"))
                    pass_to_pass = json.loads(record.get("PASS_TO_PASS", "[]"))

                    instances.append(
                        SWEBenchInstance(
                            instance_id=record["instance_id"],
                            repo=record["repo"],
                            base_commit=record["base_commit"],
                            problem_statement=record["problem_statement"],
                            patch=record.get("patch", ""),
                            test_patch=record.get("test_patch", ""),
                            fail_to_pass=fail_to_pass,
                            pass_to_pass=pass_to_pass,
                            version=record.get("version", ""),
                            environment_setup_commit=record.get(
                                "environment_setup_commit", ""
                            ),
                            hints_text=record.get("hints_text", ""),
                        )
                    )
                except (json.JSONDecodeError, KeyError) as e:
                    self._log.warning("Failed to parse cached instance", error=str(e))
                    continue

        self._log.info(
            "Loaded cached instances", path=str(self._path), count=len(instances)
        )
        return instances

    def filter_unprepared(
        self, instances: list[SWEBenchInstance]
    ) -> list[SWEBenchInstance]:
        """Return instances that are not yet prepared."""
        prepared = self.prepared_ids()
        return [i for i in instances if i.instance_id not in prepared]

    def append(self, instance: SWEBenchInstance) -> None:
        """Append a single prepared instance to the cache file."""
        self._path.parent.mkdir(parents=True, exist_ok=True)

        record = {
            "instance_id": instance.instance_id,
            "repo": instance.repo,
            "base_commit": instance.base_commit,
            "problem_statement": instance.problem_statement,
            "patch": instance.patch,
            "test_patch": instance.test_patch,
            "FAIL_TO_PASS": json.dumps(instance.fail_to_pass),
            "PASS_TO_PASS": json.dumps(instance.pass_to_pass),
            "version": instance.version,
            "environment_setup_commit": instance.environment_setup_commit,
            "hints_text": instance.hints_text,
        }

        with self._path.open("a") as f:
            f.write(json.dumps(record) + "\n")

        self._log.debug("Appended instance to cache", instance_id=instance.instance_id)

    def clear(self) -> None:
        """Remove the cache file."""
        if self._path.exists():
            self._path.unlink()
            self._log.info("Cleared cache", path=str(self._path))


class InstancePreparer:
    """Prepares instances for mini-swe-agent with Kodit context."""

    def __init__(
        self,
        kodit_url: str,
        repos_dir: Path,
        top_k: int = 10,
    ) -> None:
        """Initialize with Kodit configuration."""
        self._kodit_url = kodit_url
        self._repos_dir = repos_dir
        self._top_k = top_k
        self._log = structlog.get_logger(__name__)

    def prepare(
        self,
        instance: SWEBenchInstance,
        output_dir: Path,
    ) -> SWEBenchInstance | None:
        """Prepare a single instance by indexing and augmenting with context.

        Returns the augmented instance, or None if preparation failed.
        """
        self._log.info(
            "Preparing instance",
            instance_id=instance.instance_id,
            repo=instance.repo,
        )

        # Create operations for cloning and indexing
        operations = BenchmarkOperations(
            kodit_base_url=self._kodit_url,
            repos_dir=self._repos_dir,
            results_dir=output_dir,
            model="",  # Not used for preparation
            top_k=self._top_k,
        )

        # Create context provider for retrieval
        context_provider = KoditContextProvider(
            kodit_base_url=self._kodit_url,
            top_k=self._top_k,
        )

        try:
            # Clone and index the repository
            repo_id = operations.prepare(instance)
            self._log.info(
                "Repository prepared",
                instance_id=instance.instance_id,
                repo_id=repo_id,
            )

            # Retrieve snippets and augment problem statement
            augmented_statement = context_provider.augment_problem_statement(instance)

            # Create augmented instance
            return SWEBenchInstance(
                instance_id=instance.instance_id,
                repo=instance.repo,
                base_commit=instance.base_commit,
                problem_statement=augmented_statement,
                patch=instance.patch,
                test_patch=instance.test_patch,
                fail_to_pass=instance.fail_to_pass,
                pass_to_pass=instance.pass_to_pass,
                version=instance.version,
                environment_setup_commit=instance.environment_setup_commit,
                hints_text=instance.hints_text,
            )
        except Exception as e:  # noqa: BLE001
            self._log.exception(
                "Failed to prepare instance",
                instance_id=instance.instance_id,
                error=str(e),
            )
            return None


class MiniSweAgentRunner:
    """Runs mini-swe-agent for Kodit benchmark comparisons."""

    def __init__(
        self,
        kodit_base_url: str | None = None,
        top_k: int = 10,
    ) -> None:
        """Initialize runner with optional Kodit URL for augmented runs."""
        self._kodit_url = kodit_base_url
        self._top_k = top_k
        self._log = structlog.get_logger(__name__)

    def run_baseline(
        self,
        config: RunConfig,
        instances: list[SWEBenchInstance] | None = None,
    ) -> RunResult:
        """Run mini-swe-agent without Kodit retrieval."""
        self._log.info(
            "Running baseline mini-swe-agent",
            config=str(config.config_path),
            output_dir=str(config.output_dir),
            workers=config.workers,
        )

        return self._run_mini_swe_agent(
            config=config,
            condition="baseline",
            instances=instances,
        )

    def run_with_kodit_per_instance(
        self,
        config: RunConfig,
        instances: list[SWEBenchInstance],
        server_factory: Callable[[], ServerProcess],
    ) -> RunResult:
        """Run mini-swe-agent with Kodit, starting/stopping server per instance.

        Uses incremental saving - each prepared instance is immediately saved
        to the cache file. If preparation fails partway through, previously
        prepared instances are preserved.

        Supports resumption - already-prepared instances are skipped.
        """
        if not self._kodit_url:
            msg = "Kodit URL required for augmented runs"
            raise ValueError(msg)

        self._log.info(
            "Running mini-swe-agent with Kodit (per-instance server lifecycle)",
            config=str(config.config_path),
            output_dir=str(config.output_dir),
            workers=config.workers,
            top_k=self._top_k,
        )

        # Set up cache for incremental saving
        output_dir = config.output_dir / "kodit"
        cache_path = output_dir / "dataset" / "test.jsonl"
        cache = PreparedInstanceCache(cache_path)

        # Ensure cache directory exists
        cache_path.parent.mkdir(parents=True, exist_ok=True)

        # Clear cache if force_reindex is set
        if config.force_reindex:
            cache.clear()

        # Filter to only unprepared instances
        unprepared = cache.filter_unprepared(instances)
        already_prepared = len(instances) - len(unprepared)

        if already_prepared > 0:
            self._log.info(
                "Resuming preparation",
                already_prepared=already_prepared,
                remaining=len(unprepared),
            )

        # Prepare remaining instances with incremental saving
        self._prepare_instances_incrementally(
            config=config,
            instances=unprepared,
            server_factory=server_factory,
            cache=cache,
        )

        # Load all prepared instances (old + new)
        augmented_instances = cache.load()

        return self._run_mini_swe_agent(
            config=config,
            condition="kodit",
            instances=augmented_instances,
        )

    def _prepare_instances_incrementally(
        self,
        config: RunConfig,
        instances: list[SWEBenchInstance],
        server_factory: Callable[[], ServerProcess],
        cache: PreparedInstanceCache,
    ) -> None:
        """Prepare instances with incremental saving after each success."""
        if not instances:
            return

        total = len(instances)
        kodit_url = self._kodit_url
        if not kodit_url:
            return

        preparer = InstancePreparer(
            kodit_url=kodit_url,
            repos_dir=config.repos_dir,
            top_k=self._top_k,
        )

        for i, instance in enumerate(instances, start=1):
            self._log.info(
                "Preparing instance for Kodit retrieval",
                progress=f"{i}/{total}",
                instance_id=instance.instance_id,
                repo=instance.repo,
            )

            # Start fresh server for this instance
            server = server_factory()
            self._log.info("Starting Kodit server for instance")
            if not server.start():
                self._log.error(
                    "Failed to start Kodit server",
                    instance_id=instance.instance_id,
                )
                continue

            try:
                # Prepare and get augmented instance
                augmented = preparer.prepare(instance, config.output_dir)

                if augmented:
                    # Save immediately to cache
                    cache.append(augmented)
                    self._log.info(
                        "Instance prepared and saved",
                        instance_id=instance.instance_id,
                    )
            finally:
                # Stop server to ensure clean state for next instance
                self._log.info(
                    "Stopping Kodit server after instance",
                    instance_id=instance.instance_id,
                )
                server.stop()

    def _build_env(self, api_key: str | None) -> dict[str, str]:
        """Build environment with API key for mini-swe-agent."""
        import os

        env = os.environ.copy()
        if api_key:
            env["MSWEA_MODEL_API_KEY"] = api_key
            env["OPENROUTER_API_KEY"] = api_key
        return env

    def _execute_subprocess(
        self,
        cmd: list[str],
        env: dict[str, str],
        *,
        stream_output: bool,
    ) -> int:
        """Execute subprocess with optional output streaming."""
        if stream_output:
            streamed = subprocess.run(cmd, check=False, env=env)  # noqa: S603
            return streamed.returncode

        captured = subprocess.run(  # noqa: S603
            cmd,
            capture_output=True,
            text=True,
            check=False,
            env=env,
        )
        if captured.returncode != 0:
            output = captured.stdout or captured.stderr or ""
            self._log.error(
                "mini-swe-agent output",
                output=output[:2000] if output else "",
            )
        return captured.returncode

    def _run_mini_swe_agent(
        self,
        config: RunConfig,
        condition: str,
        instances: list[SWEBenchInstance] | None = None,
    ) -> RunResult:
        """Execute mini-swe-agent via subprocess."""
        output_dir = config.output_dir / condition
        output_dir.mkdir(parents=True, exist_ok=True)

        if instances:
            # Run with custom instances (for Kodit-augmented condition)
            return self._run_with_custom_instances(
                config=config,
                instances=instances,
                output_dir=output_dir,
                condition=condition,
            )

        # Run via CLI for baseline
        return self._run_via_cli(
            config=config,
            output_dir=output_dir,
            condition=condition,
        )

    def _run_via_cli(
        self,
        config: RunConfig,
        output_dir: Path,
        condition: str,
    ) -> RunResult:
        """Run mini-swe-agent via CLI subprocess."""
        cmd = [
            "mini-extra",
            "swebench",
            "--subset",
            config.subset,
            "--config",
            str(config.config_path),
            "--output",
            str(output_dir),
            "--workers",
            str(config.workers),
        ]

        self._log.info("Running mini-swe-agent CLI", cmd=" ".join(cmd))

        env = self._build_env(config.api_key)
        returncode = self._execute_subprocess(
            cmd, env, stream_output=config.stream_output
        )
        if returncode != 0:
            self._log.error("mini-swe-agent failed", returncode=returncode)

        predictions_path = output_dir / "preds.json"
        trajectories_dir = output_dir / "trajectories"

        return RunResult(
            predictions_path=predictions_path,
            trajectories_dir=trajectories_dir,
            total_instances=self._count_predictions(predictions_path),
            completed_instances=self._count_predictions(predictions_path),
            condition=condition,
        )

    def _run_with_custom_instances(
        self,
        config: RunConfig,
        instances: list[SWEBenchInstance],
        output_dir: Path,
        condition: str,
    ) -> RunResult:
        """Run mini-swe-agent with custom (augmented) instances."""
        # Create dataset directory with test.jsonl for HuggingFace auto-detection
        dataset_dir = output_dir / "dataset"
        dataset_dir.mkdir(parents=True, exist_ok=True)
        instances_path = dataset_dir / "test.jsonl"
        self._save_instances_as_dataset(instances, instances_path)

        # Run mini-swe-agent with custom dataset directory
        cmd = [
            "mini-extra",
            "swebench",
            "--subset",
            str(dataset_dir),
            "--split",
            "test",
            "--config",
            str(config.config_path),
            "--output",
            str(output_dir),
            "--workers",
            str(config.workers),
        ]

        self._log.info(
            "Running mini-swe-agent with custom instances",
            cmd=" ".join(cmd),
            instance_count=len(instances),
        )

        env = self._build_env(config.api_key)
        returncode = self._execute_subprocess(
            cmd, env, stream_output=config.stream_output
        )
        if returncode != 0:
            self._log.error("mini-swe-agent failed", returncode=returncode)

        predictions_path = output_dir / "preds.json"
        trajectories_dir = output_dir / "trajectories"

        # Extract stats from trajectory files
        instance_stats = self._extract_stats(output_dir)

        return RunResult(
            predictions_path=predictions_path,
            trajectories_dir=trajectories_dir,
            total_instances=len(instances),
            completed_instances=self._count_predictions(predictions_path),
            condition=condition,
            instance_stats=instance_stats,
        )

    def _save_instances_as_dataset(
        self,
        instances: list[SWEBenchInstance],
        path: Path,
    ) -> None:
        """Save instances in HuggingFace-compatible JSON Lines format."""
        with path.open("w") as f:
            for inst in instances:
                record = {
                    "instance_id": inst.instance_id,
                    "repo": inst.repo,
                    "base_commit": inst.base_commit,
                    "problem_statement": inst.problem_statement,
                    "patch": inst.patch,
                    "test_patch": inst.test_patch,
                    "FAIL_TO_PASS": json.dumps(inst.fail_to_pass),
                    "PASS_TO_PASS": json.dumps(inst.pass_to_pass),
                    "version": inst.version,
                    "environment_setup_commit": inst.environment_setup_commit,
                    "hints_text": inst.hints_text,
                }
                f.write(json.dumps(record) + "\n")

        self._log.info(
            "Saved instances as dataset",
            path=str(path),
            count=len(instances),
        )

    def _count_predictions(self, predictions_path: Path) -> int:
        """Count predictions in the output file."""
        if not predictions_path.exists():
            return 0

        try:
            with predictions_path.open() as f:
                data = json.load(f)
            return len(data)
        except (json.JSONDecodeError, OSError):
            return 0

    def convert_preds_to_jsonl(self, preds_path: Path) -> Path:
        """Convert mini-swe-agent JSON predictions to JSONL format for SWE-bench."""
        jsonl_path = preds_path.with_suffix(".jsonl")

        with preds_path.open() as f:
            preds = json.load(f)

        with jsonl_path.open("w") as f:
            for instance_id, data in preds.items():
                record = {
                    "instance_id": data.get("instance_id", instance_id),
                    "model_name_or_path": data.get("model_name_or_path", "unknown"),
                    "model_patch": data.get("model_patch", ""),
                }
                f.write(json.dumps(record) + "\n")

        self._log.info(
            "Converted predictions to JSONL",
            input=str(preds_path),
            output=str(jsonl_path),
        )
        return jsonl_path

    def _extract_stats(self, output_dir: Path) -> list[InstanceStats]:
        """Extract stats from trajectory files in output directory."""
        stats = []
        predictions = {}

        # Load predictions to check for patches
        preds_path = output_dir / "preds.json"
        if preds_path.exists():
            try:
                with preds_path.open() as f:
                    predictions = json.load(f)
            except (json.JSONDecodeError, OSError):
                pass

        # Find all trajectory files
        for traj_path in output_dir.glob("*/*.traj.json"):
            try:
                with traj_path.open() as f:
                    traj = json.load(f)

                default_id = traj_path.stem.replace(".traj", "")
                instance_id = traj.get("instance_id", default_id)
                info = traj.get("info", {})
                model_stats = info.get("model_stats", {})

                # Check if this instance has a non-empty patch
                has_patch = bool(
                    predictions.get(instance_id, {}).get("model_patch", "").strip()
                )

                stats.append(
                    InstanceStats(
                        instance_id=instance_id,
                        exit_status=info.get("exit_status", "Unknown"),
                        cost=model_stats.get("instance_cost", 0.0),
                        api_calls=model_stats.get("api_calls", 0),
                        has_patch=has_patch,
                    )
                )
            except (json.JSONDecodeError, OSError) as e:
                self._log.warning(
                    "Failed to parse trajectory",
                    path=str(traj_path),
                    error=str(e),
                )

        return stats

    def compare_results(
        self,
        baseline: RunResult,
        kodit: RunResult,
        evaluation_results: dict[str, dict] | None = None,
    ) -> ComparisonResult:
        """Compare baseline and kodit run results."""
        baseline_resolved: list[str] = []
        kodit_resolved: list[str] = []
        both_resolved: list[str] = []
        neither_resolved: list[str] = []

        if evaluation_results:
            baseline_eval = evaluation_results.get("baseline", {})
            kodit_eval = evaluation_results.get("kodit", {})

            all_instances = set(baseline_eval.keys()) | set(kodit_eval.keys())

            for instance_id in all_instances:
                baseline_success = baseline_eval.get(
                    instance_id, {}
                ).get("resolved", False)
                kodit_success = kodit_eval.get(
                    instance_id, {}
                ).get("resolved", False)

                if baseline_success and kodit_success:
                    both_resolved.append(instance_id)
                elif baseline_success:
                    baseline_resolved.append(instance_id)
                elif kodit_success:
                    kodit_resolved.append(instance_id)
                else:
                    neither_resolved.append(instance_id)

        return ComparisonResult(
            baseline=baseline,
            kodit=kodit,
            baseline_resolved=baseline_resolved,
            kodit_resolved=kodit_resolved,
            both_resolved=both_resolved,
            neither_resolved=neither_resolved,
        )
