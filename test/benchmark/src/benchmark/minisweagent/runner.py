"""Runner for mini-swe-agent benchmarks with Kodit comparison."""

from __future__ import annotations

import json
import os
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

import structlog

from benchmark.runner import BenchmarkOperations
from benchmark.swebench.instance import SWEBenchInstance
from benchmark.swebench.repository import DEFAULT_REPOS_DIR

if TYPE_CHECKING:
    from collections.abc import Callable

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
    cache_dir: Path | None = None


class DatabaseCache:
    """Cache for indexed Kodit databases, keyed by SWE-bench instance ID."""

    def __init__(self, cache_dir: Path) -> None:
        """Initialize with the cache directory path."""
        self._cache_dir = cache_dir
        self._log = structlog.get_logger(__name__)

    def path(self, instance: SWEBenchInstance) -> Path:
        """Return the cache file path for an instance.

        Prefers .tar.gz but falls back to legacy .tar if it exists.
        """
        gz = self._cache_dir / f"{instance.instance_id}.tar.gz"
        if gz.is_file():
            return gz
        legacy = self._cache_dir / f"{instance.instance_id}.tar"
        if legacy.is_file():
            return legacy
        return gz

    def partial_path(self, instance: SWEBenchInstance) -> Path:
        """Return the partial-dump cache file path for an instance."""
        return self._cache_dir / f"{instance.instance_id}.partial.tar.gz"

    def exists(self, instance: SWEBenchInstance) -> bool:
        """Check whether a cached dump exists for the given instance."""
        hit = self.path(instance).is_file()
        self._log.info(
            "Database cache lookup",
            instance_id=instance.instance_id,
            hit=hit,
        )
        return hit


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


# Path to kodit_mcp_cli.py relative to this module
_MCP_CLI_PATH = Path(__file__).resolve().parent / "kodit_mcp_cli.py"


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

    def run_with_kodit_mcp(
        self,
        config: RunConfig,
        instances: list[SWEBenchInstance],
        server_factory: Callable[[], ServerProcess],
        port: int,
    ) -> RunResult:
        """Run mini-swe-agent with live MCP access to Kodit.

        For each instance:
        1. Start a fresh Kodit server (clean database)
        2. Clone repo and index with Kodit
        3. Run mini-swe-agent with the MCP CLI mounted into Docker
        4. Collect the prediction
        5. Stop the Kodit server

        Predictions from all instances are merged into a combined preds.json.
        """
        if not self._kodit_url:
            msg = "Kodit URL required for MCP runs"
            raise ValueError(msg)

        self._log.info(
            "Running mini-swe-agent with Kodit MCP",
            config=str(config.config_path),
            output_dir=str(config.output_dir),
            instance_count=len(instances),
        )

        output_dir = config.output_dir / "kodit"
        output_dir.mkdir(parents=True, exist_ok=True)

        merged_predictions: dict = {}
        all_stats: list[InstanceStats] = []
        total = len(instances)

        for i, instance in enumerate(instances, start=1):
            self._log.info(
                "Processing instance with Kodit MCP",
                progress=f"{i}/{total}",
                instance_id=instance.instance_id,
                repo=instance.repo,
            )

            result = self._run_single_instance_with_mcp(
                config=config,
                instance=instance,
                output_dir=output_dir,
                server_factory=server_factory,
                port=port,
            )
            if result is None:
                continue

            preds, stats = result
            merged_predictions.update(preds)
            all_stats.extend(stats)

        # Write merged predictions
        predictions_path = output_dir / "preds.json"
        with predictions_path.open("w") as f:
            json.dump(merged_predictions, f, indent=2)

        self._log.info(
            "All instances complete",
            total=total,
            completed=len(merged_predictions),
        )

        return RunResult(
            predictions_path=predictions_path,
            trajectories_dir=output_dir,
            total_instances=total,
            completed_instances=len(merged_predictions),
            condition="kodit",
            instance_stats=all_stats,
        )

    def _run_single_instance_with_mcp(
        self,
        config: RunConfig,
        instance: SWEBenchInstance,
        output_dir: Path,
        server_factory: Callable[[], ServerProcess],
        port: int,
    ) -> tuple[dict, list[InstanceStats]] | None:
        """Run a single instance with a fresh Kodit server and MCP CLI.

        Returns (predictions_dict, stats_list) or None on failure.
        """
        kodit_url = self._kodit_url
        if not kodit_url:
            return None

        cache = DatabaseCache(config.cache_dir) if config.cache_dir else None
        cache_hit = cache is not None and cache.exists(instance)

        server = server_factory()
        self._log.info(
            "Starting Kodit server for instance",
            instance_id=instance.instance_id,
            cache_hit=cache_hit,
        )

        restore_dump = cache.path(instance) if cache is not None and cache_hit else None
        if not server.start(restore_dump=restore_dump):
            self._log.error(
                "Failed to start Kodit server",
                instance_id=instance.instance_id,
            )
            return None

        try:
            if not cache_hit:
                # Clone and index the repository
                operations = BenchmarkOperations(
                    kodit_base_url=kodit_url,
                    repos_dir=config.repos_dir,
                    results_dir=output_dir,
                    model="",
                )
                operations.prepare(instance)
                self._log.info(
                    "Repository indexed",
                    instance_id=instance.instance_id,
                )

                # Save the indexed database to cache
                if cache is not None:
                    cache_path = cache.path(instance)
                    if server.db.dump(cache_path):
                        self._log.info(
                            "Database cached",
                            instance_id=instance.instance_id,
                            path=str(cache_path),
                        )
            else:
                self._log.info(
                    "Skipping indexing (cache hit)",
                    instance_id=instance.instance_id,
                )

            # Run mini-swe-agent for this single instance with MCP config
            instance_output = output_dir / instance.instance_id
            instance_output.mkdir(parents=True, exist_ok=True)

            result = self._run_single_mcp_instance(
                config=config,
                instance=instance,
                output_dir=instance_output,
                port=port,
            )
            return result
        except Exception as exc:  # noqa: BLE001
            self._log.exception(
                "Failed to process instance",
                instance_id=instance.instance_id,
                error=str(exc),
            )
            # Still dump the database so partial work is preserved
            if cache is not None and not cache_hit:
                partial = cache.partial_path(instance)
                if server.db.dump(partial):
                    self._log.info(
                        "Partial database cached",
                        instance_id=instance.instance_id,
                        path=str(partial),
                    )
            return None
        finally:
            self._log.info(
                "Stopping Kodit server after instance",
                instance_id=instance.instance_id,
            )
            server.stop()

    def _run_single_mcp_instance(
        self,
        config: RunConfig,
        instance: SWEBenchInstance,
        output_dir: Path,
        port: int,
    ) -> tuple[dict, list[InstanceStats]] | None:
        """Run mini-swe-agent for a single instance with MCP overrides."""
        # Create a single-instance dataset
        dataset_dir = output_dir / "dataset"
        dataset_dir.mkdir(parents=True, exist_ok=True)
        instances_path = dataset_dir / "test.jsonl"
        self._save_instances_as_dataset([instance], instances_path)

        mcp_url = f"http://host.docker.internal:{port}/mcp"
        cli_path = str(_MCP_CLI_PATH)

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
            "1",
            # Dynamic config overrides for Docker networking and MCP CLI
            "-c",
            (
                "environment.run_args="
                '["--rm",'
                '"--add-host=host.docker.internal:host-gateway",'
                f'"-v","{cli_path}:/kodit-cli.py:ro"]'
            ),
            "-c",
            f"environment.env.KODIT_MCP_URL={mcp_url}",
        ]

        self._log.info(
            "Running mini-swe-agent with MCP",
            instance_id=instance.instance_id,
            mcp_url=mcp_url,
        )

        env = self._build_env(config.api_key)
        returncode = self._execute_subprocess(
            cmd, env, stream_output=config.stream_output
        )
        if returncode != 0:
            self._log.error(
                "mini-swe-agent failed for instance",
                instance_id=instance.instance_id,
                returncode=returncode,
            )

        # Read predictions and stats from this instance run
        preds_path = output_dir / "preds.json"
        predictions = {}
        if preds_path.exists():
            try:
                with preds_path.open() as f:
                    predictions = json.load(f)
            except (json.JSONDecodeError, OSError):
                pass

        stats = self._extract_stats(output_dir)
        return predictions, stats

    def _build_env(self, api_key: str | None) -> dict[str, str]:
        """Build environment with API key for mini-swe-agent."""
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

        if instances is not None:
            if not instances:
                msg = "No instances to run (all preparation failed?)"
                raise ValueError(msg)
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
        """Run mini-swe-agent with custom instances."""
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
