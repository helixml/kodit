"""Command line interface for kodit benchmarks."""

import json
from pathlib import Path

import click
import structlog
from dotenv import load_dotenv

from benchmark.log import configure_logging
from benchmark.minisweagent.runner import MiniSweAgentRunner, RunConfig
from benchmark.server import ServerProcess
from benchmark.swebench.evaluator import (
    EvaluationError,
    Evaluator,
)
from benchmark.swebench.loader import DatasetLoader
from benchmark.swebench.repository import (
    DEFAULT_REPOS_DIR,
)

DEFAULT_OUTPUT_DIR = Path("data")


def _extract_run_stats(output_dir: Path) -> dict:
    """Extract cost and token statistics from trajectory files."""
    stats: dict = {
        "total_cost": 0.0,
        "total_api_calls": 0,
        "total_tokens_sent": 0,
        "total_tokens_received": 0,
        "instance_count": 0,
        "instances_with_patch": 0,
        "instance_stats": {},
    }

    # Load predictions to check for patches
    predictions_path = output_dir / "preds.json"
    predictions = {}
    if predictions_path.exists():
        with predictions_path.open() as f:
            predictions = json.load(f)

    # Extract stats from trajectory files
    for trajectory_path in output_dir.glob("*/*.traj.json"):
        try:
            with trajectory_path.open() as f:
                trajectory = json.load(f)

            default_id = trajectory_path.stem.replace(".traj", "")
            instance_id = trajectory.get("instance_id", default_id)
            info = trajectory.get("info", {})
            model_stats = info.get("model_stats", {})

            cost = model_stats.get("instance_cost", 0.0)
            api_calls = model_stats.get("api_calls", 0)
            tokens_sent = model_stats.get("tokens_sent", 0)
            tokens_received = model_stats.get("tokens_received", 0)

            stats["total_cost"] += cost
            stats["total_api_calls"] += api_calls
            stats["total_tokens_sent"] += tokens_sent
            stats["total_tokens_received"] += tokens_received
            stats["instance_count"] += 1

            has_patch = bool(
                predictions.get(instance_id, {}).get("model_patch", "").strip()
            )
            if has_patch:
                stats["instances_with_patch"] += 1

            stats["instance_stats"][instance_id] = {
                "cost": cost,
                "api_calls": api_calls,
                "tokens_sent": tokens_sent,
                "tokens_received": tokens_received,
                "has_patch": has_patch,
                "exit_status": info.get("exit_status", "Unknown"),
            }
        except (json.JSONDecodeError, OSError):
            pass

    return stats


def _load_evaluation_results(
    eval_path: Path | None,
    output_dir: Path,
    run_id: str,
) -> dict:
    """Load evaluation results from JSON file."""
    log = structlog.get_logger(__name__)
    results: dict = {
        "resolved_ids": set(),
        "unresolved_ids": set(),
        "error_ids": set(),
        "empty_patch_ids": set(),
        "total": 0,
        "resolved": 0,
    }

    # Try to find evaluation file
    if eval_path and eval_path.exists():
        path = eval_path
    else:
        # Look for evaluation results in common locations
        candidates = [
            Path.cwd() / f"mini-swe-agent.{run_id}.json",
            output_dir / f"mini-swe-agent.{run_id}.json",
            Path.cwd() / f"unknown.{run_id}.json",
        ]
        candidates.extend(list(Path.cwd().glob(f"*.{run_id}.json")))

        path = None
        for candidate in candidates:
            if candidate.exists():
                path = candidate
                break

    if path and path.exists():
        log.info("Found evaluation results", path=str(path))
        with path.open() as f:
            data = json.load(f)
            results["resolved_ids"] = set(data.get("resolved_ids", []))
            results["unresolved_ids"] = set(data.get("unresolved_ids", []))
            results["error_ids"] = set(data.get("error_ids", []))
            results["empty_patch_ids"] = set(data.get("empty_patch_ids", []))
            results["total"] = data.get("total_instances", 0)
            results["resolved"] = data.get("resolved_instances", 0)
    else:
        log.warning("No evaluation results found", run_id=run_id)

    return results


def _print_section(title: str, divider: str = "-") -> None:
    """Print a section header."""
    click.echo("\n" + divider * 70)
    click.echo(title)
    click.echo(divider * 70)


def _print_metric_row(label: str, baseline_val: str, kodit_val: str) -> None:
    """Print a single metric comparison row."""
    click.echo(f"{label:<30} {baseline_val:>15} {kodit_val:>15}")


def _display_comparison_report(
    baseline_stats: dict,
    kodit_stats: dict,
    baseline_results: dict,
    kodit_results: dict,
    output: Path,
) -> None:
    """Display formatted comparison report to terminal."""
    # Compute instance sets
    b_resolved_ids = baseline_results["resolved_ids"]
    k_resolved_ids = kodit_results["resolved_ids"]
    both = b_resolved_ids & k_resolved_ids
    baseline_only = b_resolved_ids - k_resolved_ids
    kodit_only = k_resolved_ids - b_resolved_ids

    all_ids = (
        baseline_results["resolved_ids"]
        | baseline_results["unresolved_ids"]
        | baseline_results["error_ids"]
        | baseline_results["empty_patch_ids"]
        | kodit_results["resolved_ids"]
        | kodit_results["unresolved_ids"]
        | kodit_results["error_ids"]
        | kodit_results["empty_patch_ids"]
    )
    neither = all_ids - b_resolved_ids - k_resolved_ids

    # Header
    _print_section("BENCHMARK COMPARISON: BASELINE vs KODIT", "=")

    # Performance section
    _print_section("PERFORMANCE (Pass/Fail)")
    _print_metric_row("Metric", "Baseline", "Kodit")
    click.echo("-" * 70)

    b_total = baseline_results["total"]
    k_total = kodit_results["total"]
    b_res = baseline_results["resolved"]
    k_res = kodit_results["resolved"]
    b_rate = b_res / b_total if b_total > 0 else 0.0
    k_rate = k_res / k_total if k_total > 0 else 0.0

    _print_metric_row("Instances evaluated", str(b_total), str(k_total))
    _print_metric_row("Resolved (passed)", str(b_res), str(k_res))
    _print_metric_row("Resolve rate", f"{b_rate:.1%}", f"{k_rate:.1%}")

    # Instance breakdown
    _print_section("INSTANCE BREAKDOWN")
    click.echo(f"Both resolved:           {len(both):>5}")
    click.echo(f"Baseline only resolved:  {len(baseline_only):>5}")
    click.echo(f"Kodit only resolved:     {len(kodit_only):>5}")
    click.echo(f"Neither resolved:        {len(neither):>5}")

    # Cost section
    _print_section("COST & TOKEN USAGE")
    _print_metric_row("Metric", "Baseline", "Kodit")
    click.echo("-" * 70)

    b_cost = baseline_stats["total_cost"]
    k_cost = kodit_stats["total_cost"]
    _print_metric_row("Total cost", f"${b_cost:.4f}", f"${k_cost:.4f}")
    _print_metric_row(
        "Total API calls",
        f"{baseline_stats['total_api_calls']:,}",
        f"{kodit_stats['total_api_calls']:,}",
    )
    _print_metric_row(
        "Tokens sent",
        f"{baseline_stats['total_tokens_sent']:,}",
        f"{kodit_stats['total_tokens_sent']:,}",
    )
    _print_metric_row(
        "Tokens received",
        f"{baseline_stats['total_tokens_received']:,}",
        f"{kodit_stats['total_tokens_received']:,}",
    )
    b_sent = baseline_stats["total_tokens_sent"]
    b_recv = baseline_stats["total_tokens_received"]
    k_sent = kodit_stats["total_tokens_sent"]
    k_recv = kodit_stats["total_tokens_received"]
    b_tokens = b_sent + b_recv
    k_tokens = k_sent + k_recv
    _print_metric_row("Total tokens", f"{b_tokens:,}", f"{k_tokens:,}")

    # Summary
    _print_section("SUMMARY", "=")
    _print_summary(baseline_only, kodit_only, b_res, k_res)
    click.echo(f"\nDetailed results saved to: {output}")
    click.echo("=" * 70)

    # Resolution differences
    _print_resolution_differences(baseline_only, kodit_only)


def _print_summary(
    baseline_only: set, kodit_only: set, b_resolved: int, k_resolved: int
) -> None:
    """Print the summary section."""
    improvement = len(kodit_only) - len(baseline_only)
    if improvement > 0:
        click.echo(f"Kodit resolved {improvement} more instance(s) than baseline")
    elif improvement < 0:
        click.echo(f"Baseline resolved {-improvement} more instance(s) than Kodit")
    else:
        click.echo("Both approaches resolved the same number of unique instances")

    if k_resolved > b_resolved:
        diff = k_resolved - b_resolved
        click.echo(f"Overall: Kodit has {diff} more total resolutions")
    elif b_resolved > k_resolved:
        diff = b_resolved - k_resolved
        click.echo(f"Overall: Baseline has {diff} more total resolutions")


def _print_resolution_differences(baseline_only: set, kodit_only: set) -> None:
    """Print the resolution differences section."""
    if not baseline_only and not kodit_only:
        return

    _print_section("RESOLUTION DIFFERENCES")

    if baseline_only:
        click.echo("\nBaseline resolved but Kodit did not:")
        for instance_id in sorted(baseline_only):
            click.echo(f"  - {instance_id}")

    if kodit_only:
        click.echo("\nKodit resolved but Baseline did not:")
        for instance_id in sorted(kodit_only):
            click.echo(f"  + {instance_id}")


# Server defaults
DEFAULT_HOST = "127.0.0.1"
DEFAULT_PORT = 8765

# Enrichment defaults (OpenRouter base URL for the Kodit Go server)
DEFAULT_ENRICHMENT_BASE_URL = "https://openrouter.ai/api/v1"
DEFAULT_ENRICHMENT_PARALLEL_TASKS = 50
DEFAULT_ENRICHMENT_TIMEOUT = 60

# Embedding defaults (OpenRouter base URL for the Kodit Go server)
DEFAULT_EMBEDDING_BASE_URL = "https://openrouter.ai/api/v1"
DEFAULT_EMBEDDING_PARALLEL_TASKS = 50
DEFAULT_EMBEDDING_TIMEOUT = 60
DEFAULT_EMBEDDING_MODEL = "thenlper/gte-base"

# Model defaults
# Kodit server models (no litellm prefix — the Go server talks to OpenRouter directly)
DEFAULT_KODIT_ENRICHMENT_MODEL = "mistralai/mistral-nemo"
# SWE-agent model (uses litellm, needs the openrouter/ prefix)
DEFAULT_SWE_AGENT_MODEL = "openrouter/minimax/minimax-m2.5"


class MissingApiKeyError(click.ClickException):
    """Raised when ENRICHMENT_ENDPOINT_API_KEY is not set."""

    message = (
        "ENRICHMENT_ENDPOINT_API_KEY environment variable is required.\n"
        "Set it with: export ENRICHMENT_ENDPOINT_API_KEY=your-api-key"
    )

    def __init__(self) -> None:
        """Initialize with the error message."""
        super().__init__(self.message)


class MissingEmbeddingApiKeyError(click.ClickException):
    """Raised when EMBEDDING_ENDPOINT_API_KEY is not set."""

    message = (
        "EMBEDDING_ENDPOINT_API_KEY environment variable is required.\n"
        "Set it with: export EMBEDDING_ENDPOINT_API_KEY=your-api-key"
    )

    def __init__(self) -> None:
        """Initialize with the error message."""
        super().__init__(self.message)


def require_api_key(api_key: str | None) -> str:
    """Validate that API key is provided, raising an error if not."""
    if not api_key:
        raise MissingApiKeyError
    return api_key


def require_embedding_api_key(api_key: str | None) -> str:
    """Validate that embedding API key is provided, raising an error if not."""
    if not api_key:
        raise MissingEmbeddingApiKeyError
    return api_key


# Project root .env (two levels up from test/benchmark/)
_PROJECT_ROOT = Path(__file__).resolve().parents[4]


@click.group(context_settings={"max_content_width": 100})
def cli() -> None:
    """kodit-benchmark CLI - Benchmark Kodit's retrieval capabilities."""
    load_dotenv(_PROJECT_ROOT / ".env")
    configure_logging()


@cli.command("download")
@click.option(
    "--dataset",
    type=click.Choice(["lite", "verified"]),
    default="lite",
    help="SWE-bench dataset variant",
)
@click.option(
    "--output",
    type=click.Path(path_type=Path),
    default=None,
    help="Output JSON file path (default: data/swebench-{variant}.json)",
)
def download(dataset: str, output: Path | None) -> None:
    """Download SWE-bench dataset from HuggingFace and save as JSON."""
    log = structlog.get_logger(__name__)

    if output is None:
        output = DEFAULT_OUTPUT_DIR / f"swebench-{dataset}.json"

    log.info("Downloading SWE-bench dataset", variant=dataset, output=str(output))

    loader = DatasetLoader()
    instances = loader.download(dataset)
    loader.save(instances, output)

    log.info("Download complete", count=len(instances), output=str(output))


# ============================================================================
# Mini-swe-agent commands
# ============================================================================

MINI_SWE_AGENT_OUTPUT_DIR = Path("output/minisweagent")
MINI_SWE_AGENT_CONFIG_DIR = Path(__file__).parent / "minisweagent" / "configs"


@cli.group("mini-swe-agent")
def mini_swe_agent_group() -> None:
    """Mini-swe-agent benchmark commands for Kodit comparison."""


@mini_swe_agent_group.command("run-baseline")
@click.option(
    "--dataset-file",
    type=click.Path(path_type=Path, exists=True),
    default=DEFAULT_OUTPUT_DIR / "swebench-verified.json",
    help="Path to SWE-bench dataset JSON file",
)
@click.option(
    "--output-dir",
    type=click.Path(path_type=Path),
    default=MINI_SWE_AGENT_OUTPUT_DIR,
    help="Output directory for predictions and trajectories",
)
@click.option(
    "--workers",
    default=1,
    type=int,
    help="Number of parallel workers",
)
@click.option(
    "--limit",
    default=None,
    type=int,
    help="Limit number of instances to run (for testing)",
)
@click.option(
    "--instance-id",
    default=None,
    type=str,
    help="Run only a specific instance by ID",
)
@click.option(
    "--api-key",
    envvar="ENRICHMENT_ENDPOINT_API_KEY",
    help="LLM API key (or set ENRICHMENT_ENDPOINT_API_KEY)",
)
@click.option(
    "--swe-agent-model",
    default=DEFAULT_SWE_AGENT_MODEL,
    help="LiteLLM model identifier for mini-swe-agent",
)
@click.option(
    "--evaluate/--no-evaluate",
    default=True,
    help="Run SWE-bench evaluation after completion",
)
@click.option(
    "--stream/--no-stream",
    default=True,
    help="Stream mini-swe-agent output to terminal instead of capturing",
)
def mini_run_baseline(  # noqa: PLR0913, PLR0915, C901
    dataset_file: Path,
    output_dir: Path,
    workers: int,
    limit: int | None,
    instance_id: str | None,
    api_key: str | None,
    swe_agent_model: str,
    evaluate: bool,  # noqa: FBT001
    stream: bool,  # noqa: FBT001
) -> None:
    """Run mini-swe-agent baseline (without Kodit retrieval).

    This runs mini-swe-agent against SWE-bench instances with only the
    problem statement, providing a baseline for comparison.
    """
    api_key = require_api_key(api_key)
    log = structlog.get_logger(__name__)

    # Load instances
    loader = DatasetLoader()
    instances = loader.load(dataset_file)

    if instance_id:
        instances = [i for i in instances if i.instance_id == instance_id]
        if not instances:
            click.echo(f"Instance not found: {instance_id}", err=True)
            raise SystemExit(1)
        log.info("Running single instance", instance_id=instance_id)
    elif limit:
        instances = instances[:limit]
        log.info("Limited instances", limit=limit)

    log.info(
        "Running mini-swe-agent baseline",
        instance_count=len(instances),
        workers=workers,
    )

    # Create runner and config
    runner = MiniSweAgentRunner()
    config = RunConfig(
        config_path=MINI_SWE_AGENT_CONFIG_DIR / "baseline.yaml",
        output_dir=output_dir,
        model=swe_agent_model,
        workers=workers,
        api_key=api_key,
        stream_output=stream,
    )

    result = runner.run_baseline(config, instances)

    click.echo("\n" + "=" * 60)
    click.echo("MINI-SWE-AGENT BASELINE COMPLETE")
    click.echo("=" * 60)
    click.echo(f"Total instances: {result.total_instances}")
    click.echo(f"Completed: {result.completed_instances}")
    click.echo(f"With patches: {result.instances_with_patch}")
    click.echo("-" * 60)
    click.echo(f"Total cost: ${result.total_cost:.4f}")
    click.echo(f"Total API calls: {result.total_api_calls}")
    click.echo("-" * 60)

    # Show per-instance stats
    if result.instance_stats:
        click.echo("\nPer-instance results:")
        for stat in result.instance_stats:
            patch_indicator = "✓" if stat.has_patch else "✗"
            click.echo(
                f"  {patch_indicator} {stat.instance_id}: "
                f"{stat.exit_status} (${stat.cost:.4f}, {stat.api_calls} calls)"
            )

    click.echo("-" * 60)
    click.echo(f"Predictions: {result.predictions_path}")
    click.echo("=" * 60)

    # Run evaluation if requested
    if evaluate and result.instances_with_patch > 0:
        click.echo("\n" + "=" * 60)
        click.echo("RUNNING SWE-BENCH EVALUATION")
        click.echo("=" * 60)

        # Convert predictions to JSONL format for SWE-bench
        jsonl_path = runner.convert_preds_to_jsonl(result.predictions_path)
        log.info("Converted predictions for evaluation", jsonl_path=str(jsonl_path))

        # Run evaluation
        evaluator = Evaluator()
        try:
            eval_result = evaluator.evaluate_full(
                predictions_path=jsonl_path,
                dataset_name="princeton-nlp/SWE-bench_Verified",
                max_workers=workers,
                run_id="mini_swe_agent_baseline",
            )

            click.echo("\n" + "-" * 60)
            click.echo("EVALUATION RESULTS")
            click.echo("-" * 60)
            click.echo(f"Total predictions: {eval_result.total_predictions}")
            click.echo(f"Resolved: {eval_result.resolved}")
            click.echo(f"Resolve rate: {eval_result.resolve_rate:.1%}")

            # Show per-instance evaluation results
            if eval_result.instance_results:
                click.echo("\nPer-instance evaluation:")
                for ir in eval_result.instance_results:
                    status_indicator = "✓" if ir.status == "resolved" else "✗"
                    click.echo(f"  {status_indicator} {ir.instance_id}: {ir.status}")

            click.echo("=" * 60)

        except EvaluationError as e:
            log.exception("Evaluation failed", error=str(e))
            click.echo(f"Evaluation failed: {e}", err=True)
    elif evaluate and result.instances_with_patch == 0:
        click.echo("\nSkipping evaluation: no instances produced patches", err=True)


@mini_swe_agent_group.command("run-kodit")
@click.option(
    "--dataset-file",
    type=click.Path(path_type=Path, exists=True),
    default=DEFAULT_OUTPUT_DIR / "swebench-verified.json",
    help="Path to SWE-bench dataset JSON file",
)
@click.option(
    "--output-dir",
    type=click.Path(path_type=Path),
    default=MINI_SWE_AGENT_OUTPUT_DIR,
    help="Output directory for predictions and trajectories",
)
@click.option(
    "--repos-dir",
    type=click.Path(path_type=Path),
    default=DEFAULT_REPOS_DIR,
    help="Directory to clone repositories into",
)
@click.option("--host", default="0.0.0.0", help="Kodit server host")  # noqa: S104
@click.option("--port", default=DEFAULT_PORT, type=int, help="Kodit server port")
@click.option(
    "--enrichment-base-url",
    default=DEFAULT_ENRICHMENT_BASE_URL,
    help="Enrichment endpoint base URL",
)
@click.option(
    "--kodit-enrichment-model",
    default=DEFAULT_KODIT_ENRICHMENT_MODEL,
    help="Enrichment model name",
)
@click.option(
    "--enrichment-parallel-tasks",
    default=DEFAULT_ENRICHMENT_PARALLEL_TASKS,
    type=int,
    help="Number of parallel enrichment tasks",
)
@click.option(
    "--enrichment-timeout",
    default=DEFAULT_ENRICHMENT_TIMEOUT,
    type=int,
    help="Enrichment request timeout in seconds",
)
@click.option(
    "--embedding-base-url",
    default=DEFAULT_EMBEDDING_BASE_URL,
    help="Embedding endpoint base URL",
)
@click.option(
    "--embedding-model",
    default=DEFAULT_EMBEDDING_MODEL,
    help="Embedding model name",
)
@click.option(
    "--embedding-api-key",
    envvar="EMBEDDING_ENDPOINT_API_KEY",
    help="Embedding API key (or set EMBEDDING_ENDPOINT_API_KEY)",
)
@click.option(
    "--embedding-parallel-tasks",
    default=DEFAULT_EMBEDDING_PARALLEL_TASKS,
    type=int,
    help="Number of parallel embedding tasks",
)
@click.option(
    "--embedding-timeout",
    default=DEFAULT_EMBEDDING_TIMEOUT,
    type=int,
    help="Embedding request timeout in seconds",
)
@click.option(
    "--limit",
    default=None,
    type=int,
    help="Limit number of instances to run (for testing)",
)
@click.option(
    "--instance-id",
    default=None,
    type=str,
    help="Run only a specific instance by ID",
)
@click.option(
    "--api-key",
    envvar="ENRICHMENT_ENDPOINT_API_KEY",
    help="LLM API key (or set ENRICHMENT_ENDPOINT_API_KEY)",
)
@click.option(
    "--swe-agent-model",
    default=DEFAULT_SWE_AGENT_MODEL,
    help="LiteLLM model identifier for mini-swe-agent",
)
@click.option(
    "--stream/--no-stream",
    default=True,
    help="Stream mini-swe-agent output to terminal instead of capturing",
)
@click.option(
    "--evaluate/--no-evaluate",
    default=True,
    help="Run SWE-bench evaluation after completion",
)
@click.option(
    "--cache-dir",
    type=click.Path(path_type=Path),
    default=Path(__file__).resolve().parents[2] / ".db_cache",
    help="Directory for caching indexed database dumps (set empty to disable)",
)
def mini_run_kodit(  # noqa: PLR0913, PLR0915, C901
    dataset_file: Path,
    output_dir: Path,
    repos_dir: Path,
    host: str,
    port: int,
    enrichment_base_url: str,
    kodit_enrichment_model: str,
    enrichment_parallel_tasks: int,
    enrichment_timeout: int,
    embedding_base_url: str,
    embedding_model: str,
    embedding_api_key: str | None,
    embedding_parallel_tasks: int,
    embedding_timeout: int,
    limit: int | None,
    instance_id: str | None,
    api_key: str | None,
    swe_agent_model: str,
    stream: bool,  # noqa: FBT001
    evaluate: bool,  # noqa: FBT001
    cache_dir: Path | None,
) -> None:
    """Run mini-swe-agent with live Kodit MCP access.

    Gives the agent a CLI tool (kodit_mcp_cli.py) volume-mounted into the
    Docker container so it can query Kodit throughout its execution.

    For each instance, this command:
    1. Starts a fresh Kodit server and database
    2. Clones the repository at the exact commit and indexes it
    3. Runs mini-swe-agent with the MCP CLI mounted at /kodit-cli.py
    4. Collects the prediction
    5. Stops the Kodit server
    """
    api_key = require_api_key(api_key)
    if not embedding_api_key:
        embedding_api_key = api_key
    log = structlog.get_logger(__name__)

    # Load instances
    loader = DatasetLoader()
    instances = loader.load(dataset_file)

    if instance_id:
        instances = [i for i in instances if i.instance_id == instance_id]
        if not instances:
            click.echo(f"Instance not found: {instance_id}", err=True)
            raise SystemExit(1)
        log.info("Running single instance", instance_id=instance_id)
    elif limit:
        instances = instances[:limit]
        log.info("Limited instances", limit=limit)

    log.info(
        "Running mini-swe-agent with Kodit MCP",
        instance_count=len(instances),
        repos_dir=str(repos_dir),
    )

    # Helper to create a fresh server for each instance
    def create_server() -> ServerProcess:
        return ServerProcess(
            host=host,
            port=port,
            enrichment_base_url=enrichment_base_url,
            enrichment_model=kodit_enrichment_model,
            enrichment_api_key=api_key,
            enrichment_parallel_tasks=enrichment_parallel_tasks,
            enrichment_timeout=enrichment_timeout,
            embedding_base_url=embedding_base_url,
            embedding_model=embedding_model,
            embedding_api_key=embedding_api_key,
            embedding_parallel_tasks=embedding_parallel_tasks,
            embedding_timeout=embedding_timeout,
        )

    base_url = f"http://{host}:{port}"

    # Create runner and config
    runner = MiniSweAgentRunner(kodit_base_url=base_url)
    config = RunConfig(
        config_path=MINI_SWE_AGENT_CONFIG_DIR / "kodit.yaml",
        output_dir=output_dir,
        model=swe_agent_model,
        repos_dir=repos_dir,
        workers=1,
        api_key=api_key,
        stream_output=stream,
        cache_dir=cache_dir,
    )

    # Process each instance with fresh server start/stop and MCP access
    result = runner.run_with_kodit_mcp(
        config=config,
        instances=instances,
        server_factory=create_server,
        port=port,
    )

    click.echo("\n" + "=" * 60)
    click.echo("MINI-SWE-AGENT WITH KODIT MCP COMPLETE")
    click.echo("=" * 60)
    click.echo(f"Total instances: {result.total_instances}")
    click.echo(f"Completed: {result.completed_instances}")
    click.echo(f"With patches: {result.instances_with_patch}")
    click.echo("-" * 60)
    click.echo(f"Total cost: ${result.total_cost:.4f}")
    click.echo(f"Total API calls: {result.total_api_calls}")
    click.echo("-" * 60)

    # Show per-instance stats
    if result.instance_stats:
        click.echo("\nPer-instance results:")
        for stat in result.instance_stats:
            patch_indicator = "✓" if stat.has_patch else "✗"
            click.echo(
                f"  {patch_indicator} {stat.instance_id}: "
                f"{stat.exit_status} (${stat.cost:.4f}, {stat.api_calls} calls)"
            )

    click.echo("-" * 60)
    click.echo(f"Predictions: {result.predictions_path}")
    click.echo("=" * 60)

    # Run evaluation if requested
    if evaluate and result.instances_with_patch > 0:
        click.echo("\n" + "=" * 60)
        click.echo("RUNNING SWE-BENCH EVALUATION")
        click.echo("=" * 60)

        # Convert predictions to JSONL format for SWE-bench
        jsonl_path = runner.convert_preds_to_jsonl(result.predictions_path)
        log.info("Converted predictions for evaluation", jsonl_path=str(jsonl_path))

        # Run evaluation
        evaluator = Evaluator()
        try:
            eval_result = evaluator.evaluate_full(
                predictions_path=jsonl_path,
                dataset_name="princeton-nlp/SWE-bench_Verified",
                max_workers=1,
                run_id="mini_swe_agent_kodit",
            )

            click.echo("\n" + "-" * 60)
            click.echo("EVALUATION RESULTS")
            click.echo("-" * 60)
            click.echo(f"Total predictions: {eval_result.total_predictions}")
            click.echo(f"Resolved: {eval_result.resolved}")
            click.echo(f"Resolve rate: {eval_result.resolve_rate:.1%}")

            # Show per-instance evaluation results
            if eval_result.instance_results:
                click.echo("\nPer-instance evaluation:")
                for ir in eval_result.instance_results:
                    status_indicator = "✓" if ir.status == "resolved" else "✗"
                    click.echo(f"  {status_indicator} {ir.instance_id}: {ir.status}")

            click.echo("=" * 60)

        except EvaluationError as e:
            log.exception("Evaluation failed", error=str(e))
            click.echo(f"Evaluation failed: {e}", err=True)
    elif evaluate and result.instances_with_patch == 0:
        click.echo("\nSkipping evaluation: no instances produced patches", err=True)


@mini_swe_agent_group.command("compare")
@click.option(
    "--baseline-dir",
    type=click.Path(path_type=Path, exists=True),
    default=MINI_SWE_AGENT_OUTPUT_DIR / "baseline",
    help="Path to baseline output directory",
)
@click.option(
    "--kodit-dir",
    type=click.Path(path_type=Path, exists=True),
    default=MINI_SWE_AGENT_OUTPUT_DIR / "kodit",
    help="Path to Kodit output directory",
)
@click.option(
    "--baseline-eval",
    type=click.Path(path_type=Path, exists=True),
    default=None,
    help="Path to baseline evaluation JSON (auto-detected if not specified)",
)
@click.option(
    "--kodit-eval",
    type=click.Path(path_type=Path, exists=True),
    default=None,
    help="Path to Kodit evaluation JSON (auto-detected if not specified)",
)
@click.option(
    "--output",
    type=click.Path(path_type=Path),
    default=MINI_SWE_AGENT_OUTPUT_DIR / "comparison.json",
    help="Output JSON file for comparison results",
)
def mini_compare(
    baseline_dir: Path,
    kodit_dir: Path,
    baseline_eval: Path | None,
    kodit_eval: Path | None,
    output: Path,
) -> None:
    """Compare baseline and Kodit mini-swe-agent results.

    Compares pass/fail rates, total costs, and token usage between
    the baseline and Kodit-augmented approaches.

    Requires evaluation results to have been generated (run with --evaluate).
    """
    log = structlog.get_logger(__name__)
    log.info(
        "Comparing results",
        baseline_dir=str(baseline_dir),
        kodit_dir=str(kodit_dir),
    )

    # Extract stats from both directories
    baseline_stats = _extract_run_stats(baseline_dir)
    kodit_stats = _extract_run_stats(kodit_dir)

    # Load evaluation results
    baseline_results = _load_evaluation_results(
        baseline_eval, baseline_dir, "mini_swe_agent_baseline"
    )
    kodit_results = _load_evaluation_results(
        kodit_eval, kodit_dir, "mini_swe_agent_kodit"
    )

    # Compute comparison metrics
    baseline_resolved = baseline_results["resolved_ids"]
    kodit_resolved = kodit_results["resolved_ids"]
    both_resolved = baseline_resolved & kodit_resolved
    baseline_only = baseline_resolved - kodit_resolved
    kodit_only = kodit_resolved - baseline_resolved

    all_instances = (
        baseline_results["resolved_ids"]
        | baseline_results["unresolved_ids"]
        | baseline_results["error_ids"]
        | baseline_results["empty_patch_ids"]
        | kodit_results["resolved_ids"]
        | kodit_results["unresolved_ids"]
        | kodit_results["error_ids"]
        | kodit_results["empty_patch_ids"]
    )
    neither = all_instances - baseline_resolved - kodit_resolved

    instance_sets = {
        "both": both_resolved,
        "baseline_only": baseline_only,
        "kodit_only": kodit_only,
        "neither": neither,
    }

    # Build comparison data for JSON output
    comparison = _build_comparison_dict(
        baseline_stats,
        kodit_stats,
        baseline_results,
        kodit_results,
        instance_sets,
    )

    # Write comparison JSON
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("w") as f:
        json.dump(comparison, f, indent=2)

    # Display formatted report
    _display_comparison_report(
        baseline_stats,
        kodit_stats,
        baseline_results,
        kodit_results,
        output,
    )


def _build_run_summary(stats: dict, results: dict) -> dict:
    """Build summary dict for a single run."""
    total = results["total"]
    return {
        "instances_evaluated": total,
        "resolved": results["resolved"],
        "resolve_rate": results["resolved"] / total if total > 0 else 0.0,
        "total_cost": stats["total_cost"],
        "total_api_calls": stats["total_api_calls"],
        "total_tokens_sent": stats["total_tokens_sent"],
        "total_tokens_received": stats["total_tokens_received"],
        "total_tokens": stats["total_tokens_sent"] + stats["total_tokens_received"],
    }


def _build_comparison_dict(
    baseline_stats: dict,
    kodit_stats: dict,
    baseline_results: dict,
    kodit_results: dict,
    instance_sets: dict,
) -> dict:
    """Build comparison dictionary for JSON output."""
    both = instance_sets["both"]
    baseline_only = instance_sets["baseline_only"]
    kodit_only = instance_sets["kodit_only"]
    neither = instance_sets["neither"]

    return {
        "summary": {
            "baseline": _build_run_summary(baseline_stats, baseline_results),
            "kodit": _build_run_summary(kodit_stats, kodit_results),
            "comparison": {
                "both_resolved": len(both),
                "baseline_only_resolved": len(baseline_only),
                "kodit_only_resolved": len(kodit_only),
                "neither_resolved": len(neither),
                "kodit_improvement": len(kodit_only) - len(baseline_only),
            },
        },
        "instances": {
            "both_resolved": sorted(both),
            "baseline_only_resolved": sorted(baseline_only),
            "kodit_only_resolved": sorted(kodit_only),
            "neither_resolved": sorted(neither),
        },
    }


if __name__ == "__main__":
    cli()
