"""Command line interface for kodit benchmarks."""

import json
from pathlib import Path

import click
import structlog

from benchmark.minisweagent.runner import MiniSweAgentRunner, RunConfig
from benchmark.runner import (
    BatchConfig,
    BatchRunner,
    BenchmarkOperations,
)
from benchmark.server import (
    DEFAULT_DB_PORT,
    DEFAULT_ENRICHMENT_BASE_URL,
    DEFAULT_ENRICHMENT_MODEL,
    DEFAULT_ENRICHMENT_PARALLEL_TASKS,
    DEFAULT_ENRICHMENT_TIMEOUT,
    DEFAULT_HOST,
    DEFAULT_PORT,
    ServerProcess,
)
from benchmark.swebench.evaluator import (
    EvaluationError,
    Evaluator,
    PredictionLoader,
)
from benchmark.swebench.loader import DatasetLoader, InstanceNotFoundError
from benchmark.swebench.repository import (
    DEFAULT_REPOS_DIR,
    RepositoryCloneError,
    RepositoryIndexError,
)
from kodit.config import AppContext
from kodit.log import configure_logging

DEFAULT_OUTPUT_DIR = Path("benchmarks/data")
DEFAULT_RESULTS_DIR = Path("benchmarks/results")
DEFAULT_DATASET_FILE = DEFAULT_OUTPUT_DIR / "swebench-lite.json"
DEFAULT_MODEL = "openrouter/google/gemini-2.5-flash-lite"


class MissingApiKeyError(click.ClickException):
    """Raised when ENRICHMENT_ENDPOINT_API_KEY is not set."""

    message = (
        "ENRICHMENT_ENDPOINT_API_KEY environment variable is required.\n"
        "Set it with: export ENRICHMENT_ENDPOINT_API_KEY=your-api-key"
    )

    def __init__(self) -> None:
        """Initialize with the error message."""
        super().__init__(self.message)


def require_api_key(api_key: str | None) -> str:
    """Validate that API key is provided, raising an error if not."""
    if not api_key:
        raise MissingApiKeyError
    return api_key


@click.group(context_settings={"max_content_width": 100})
def cli() -> None:
    """kodit-benchmark CLI - Benchmark Kodit's retrieval capabilities."""
    configure_logging(AppContext())


@cli.command("start-kodit")
@click.option("--host", default=DEFAULT_HOST, help="Host to bind the server to")
@click.option("--port", default=DEFAULT_PORT, type=int, help="Port to bind the server")
@click.option("--db-port", default=DEFAULT_DB_PORT, type=int, help="Database port")
@click.option(
    "--enrichment-base-url",
    default=DEFAULT_ENRICHMENT_BASE_URL,
    help="Enrichment endpoint base URL",
)
@click.option(
    "--enrichment-model",
    default=DEFAULT_ENRICHMENT_MODEL,
    help="Enrichment model name",
)
@click.option(
    "--enrichment-api-key",
    envvar="ENRICHMENT_ENDPOINT_API_KEY",
    help="Enrichment API key (or set ENRICHMENT_ENDPOINT_API_KEY)",
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
def start_kodit(  # noqa: PLR0913
    host: str,
    port: int,
    db_port: int,
    enrichment_base_url: str,
    enrichment_model: str,
    enrichment_api_key: str | None,
    enrichment_parallel_tasks: int,
    enrichment_timeout: int,
) -> None:
    """Start database and Kodit server for benchmarking."""
    enrichment_api_key = require_api_key(enrichment_api_key)
    log = structlog.get_logger(__name__)

    server = ServerProcess(
        host=host,
        port=port,
        db_port=db_port,
        enrichment_base_url=enrichment_base_url,
        enrichment_model=enrichment_model,
        enrichment_api_key=enrichment_api_key,
        enrichment_parallel_tasks=enrichment_parallel_tasks,
        enrichment_timeout=enrichment_timeout,
    )

    if server.start():
        log.info("Kodit server started", url=server.base_url)
    else:
        log.error("Failed to start Kodit server")
        raise SystemExit(1)


@cli.command("stop-kodit")
def stop_kodit() -> None:
    """Stop the Kodit server and database."""
    log = structlog.get_logger(__name__)
    server = ServerProcess()

    if server.stop():
        log.info("Kodit server stopped")
    else:
        log.error("Failed to stop Kodit server")
        raise SystemExit(1)


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
    help="Output JSON file path (default: benchmarks/data/swebench-{variant}.json)",
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


@cli.command("prepare-instance")
@click.argument("instance_id")
@click.option(
    "--dataset-file",
    type=click.Path(path_type=Path, exists=True),
    default=DEFAULT_DATASET_FILE,
    help="Path to SWE-bench dataset JSON file",
)
@click.option(
    "--repos-dir",
    type=click.Path(path_type=Path),
    default=DEFAULT_REPOS_DIR,
    help="Directory to clone repositories into",
)
@click.option(
    "--kodit-url",
    default=None,
    help="Kodit server URL (default: http://{host}:{port} from start-kodit)",
)
@click.option(
    "--host",
    default=DEFAULT_HOST,
    help="Kodit server host (used if --kodit-url not provided)",
)
@click.option(
    "--port",
    default=DEFAULT_PORT,
    type=int,
    help="Kodit server port (used if --kodit-url not provided)",
)
def prepare_instance(  # noqa: PLR0913
    instance_id: str,
    dataset_file: Path,
    repos_dir: Path,
    kodit_url: str | None,
    host: str,
    port: int,
) -> None:
    """Prepare a SWE-bench instance for benchmarking.

    Clones the repository at the exact commit and indexes it with a running
    Kodit server.

    INSTANCE_ID is the SWE-bench instance identifier (e.g., django__django-11049).
    """
    log = structlog.get_logger(__name__)

    # Find instance
    loader = DatasetLoader()
    try:
        instance = loader.find(dataset_file, instance_id)
    except InstanceNotFoundError:
        log.exception("Instance not found", instance_id=instance_id)
        raise SystemExit(1) from None

    log.info(
        "Found instance",
        instance_id=instance.instance_id,
        repo=instance.repo,
        commit=instance.base_commit[:12],
    )

    # Prepare using BenchmarkOperations
    base_url = kodit_url or f"http://{host}:{port}"
    operations = BenchmarkOperations(
        kodit_base_url=base_url,
        repos_dir=repos_dir,
        results_dir=DEFAULT_RESULTS_DIR,
        model=DEFAULT_MODEL,
    )

    try:
        repo_id = operations.prepare(instance)
        log.info("Instance prepared", instance_id=instance_id, repo_id=repo_id)
    except RepositoryCloneError as e:
        log.exception("Clone failed", error=str(e))
        raise SystemExit(1) from e
    except RepositoryIndexError as e:
        log.exception("Indexing failed", error=str(e))
        raise SystemExit(1) from e


@cli.command("generate")
@click.argument("instance_id")
@click.option(
    "--condition",
    type=click.Choice(["baseline", "kodit"]),
    required=True,
    help="Retrieval condition: baseline (no retrieval) or kodit (with Kodit search)",
)
@click.option(
    "--dataset-file",
    type=click.Path(path_type=Path, exists=True),
    default=DEFAULT_DATASET_FILE,
    help="Path to SWE-bench dataset JSON file",
)
@click.option(
    "--output",
    type=click.Path(path_type=Path),
    default=None,
    help="Output JSONL file (default: benchmarks/results/{condition}.jsonl)",
)
@click.option(
    "--model",
    default=DEFAULT_MODEL,
    help="LiteLLM model identifier",
)
@click.option(
    "--top-k",
    default=10,
    type=int,
    help="Number of snippets to retrieve (for kodit condition)",
)
@click.option(
    "--api-key",
    envvar="ENRICHMENT_ENDPOINT_API_KEY",
    help="LLM API key (or set ENRICHMENT_ENDPOINT_API_KEY)",
)
@click.option(
    "--kodit-url",
    default=None,
    help="Kodit server URL (for kodit condition)",
)
@click.option(
    "--host",
    default=DEFAULT_HOST,
    help="Kodit server host (used if --kodit-url not provided)",
)
@click.option(
    "--port",
    default=DEFAULT_PORT,
    type=int,
    help="Kodit server port (used if --kodit-url not provided)",
)
def generate(  # noqa: PLR0913
    instance_id: str,
    condition: str,
    dataset_file: Path,
    output: Path | None,
    model: str,
    top_k: int,
    api_key: str | None,
    kodit_url: str | None,
    host: str,
    port: int,
) -> None:
    """Generate a patch prediction for a SWE-bench instance.

    INSTANCE_ID is the SWE-bench instance identifier (e.g., django__django-11049).

    This command generates a patch prediction using either:
    - baseline: LLM with only the problem statement
    - kodit: LLM with Kodit-retrieved code context

    Output is appended to a JSONL file compatible with SWE-bench evaluation.
    """
    log = structlog.get_logger(__name__)

    # Find instance
    loader = DatasetLoader()
    try:
        instance = loader.find(dataset_file, instance_id)
    except InstanceNotFoundError:
        log.exception("Instance not found", instance_id=instance_id)
        raise SystemExit(1) from None

    log.info(
        "Generating prediction",
        instance_id=instance.instance_id,
        condition=condition,
        model=model,
    )

    # Generate using BenchmarkOperations
    base_url = kodit_url or f"http://{host}:{port}"
    operations = BenchmarkOperations(
        kodit_base_url=base_url,
        repos_dir=DEFAULT_REPOS_DIR,
        results_dir=DEFAULT_RESULTS_DIR,
        model=model,
        api_key=api_key,
        top_k=top_k,
    )

    prediction, output_path = operations.generate_and_write(
        instance,
        condition,
        output_path=output,
    )

    log.info(
        "Prediction written",
        output=str(output_path),
        instance_id=prediction.instance_id,
        model=prediction.model_name_or_path,
    )


@cli.command("evaluate")
@click.argument("predictions_file", type=click.Path(path_type=Path, exists=True))
@click.option(
    "--output",
    type=click.Path(path_type=Path),
    default=None,
    help="Output JSON file for results (default: predictions_file with .results.json)",
)
@click.option(
    "--max-workers",
    default=4,
    type=int,
    help="Number of parallel workers for evaluation",
)
@click.option(
    "--run-id",
    default="kodit_eval",
    help="Run ID for SWE-bench evaluation",
)
def evaluate(
    predictions_file: Path,
    output: Path | None,
    max_workers: int,
    run_id: str,
) -> None:
    """Evaluate predictions against SWE-bench using the official harness.

    PREDICTIONS_FILE is the path to a JSONL file with predictions.

    Requires Docker and the swebench package to be installed.
    """
    log = structlog.get_logger(__name__)

    # Determine output path
    if output is None:
        output = predictions_file.with_suffix(".results.json")

    log.info(
        "Loading predictions",
        predictions_file=str(predictions_file),
    )

    # Load predictions
    prediction_loader = PredictionLoader()
    predictions = prediction_loader.load(predictions_file)

    log.info("Loaded predictions", count=len(predictions))

    # Run evaluation
    evaluator = Evaluator()

    log.info(
        "Running SWE-bench evaluation",
        max_workers=max_workers,
        run_id=run_id,
    )

    try:
        result = evaluator.evaluate_full(
            predictions_path=predictions_file,
            max_workers=max_workers,
            run_id=run_id,
        )
    except EvaluationError as e:
        log.error("Evaluation failed", error=str(e))  # noqa: TRY400
        raise SystemExit(1) from None

    # Write results
    with output.open("w") as f:
        json.dump(result.as_dict(), f, indent=2)

    log.info(
        "Evaluation complete",
        output=str(output),
        total_predictions=result.total_predictions,
        resolved=result.resolved,
        resolve_rate=f"{result.resolve_rate:.1%}",
    )


@cli.command("run-instance")
@click.argument("instance_id")
@click.option(
    "--dataset-file",
    type=click.Path(path_type=Path, exists=True),
    default=DEFAULT_DATASET_FILE,
    help="Path to SWE-bench dataset JSON file",
)
@click.option(
    "--repos-dir",
    type=click.Path(path_type=Path),
    default=DEFAULT_REPOS_DIR,
    help="Directory to clone repositories into",
)
@click.option(
    "--results-dir",
    type=click.Path(path_type=Path),
    default=DEFAULT_RESULTS_DIR,
    help="Directory to write results to",
)
@click.option(
    "--model",
    default=DEFAULT_MODEL,
    help="LiteLLM model identifier for patch generation",
)
@click.option(
    "--api-key",
    envvar="ENRICHMENT_ENDPOINT_API_KEY",
    help="LLM API key (or set ENRICHMENT_ENDPOINT_API_KEY)",
)
@click.option(
    "--top-k",
    default=10,
    type=int,
    help="Number of snippets to retrieve for kodit condition",
)
@click.option(
    "--skip-evaluation",
    is_flag=True,
    help="Skip SWE-bench evaluation (only generate predictions)",
)
@click.option("--host", default=DEFAULT_HOST, help="Host to bind the server to")
@click.option("--port", default=DEFAULT_PORT, type=int, help="Port to bind the server")
@click.option("--db-port", default=DEFAULT_DB_PORT, type=int, help="Database port")
@click.option(
    "--enrichment-base-url",
    default=DEFAULT_ENRICHMENT_BASE_URL,
    help="Enrichment endpoint base URL",
)
@click.option(
    "--enrichment-model",
    default=DEFAULT_ENRICHMENT_MODEL,
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
def run_instance(  # noqa: PLR0913
    instance_id: str,
    dataset_file: Path,
    repos_dir: Path,
    results_dir: Path,
    model: str,
    api_key: str | None,
    top_k: int,
    skip_evaluation: bool,  # noqa: FBT001
    host: str,
    port: int,
    db_port: int,
    enrichment_base_url: str,
    enrichment_model: str,
    enrichment_parallel_tasks: int,
    enrichment_timeout: int,
) -> None:
    """Run a complete benchmark for a single SWE-bench instance.

    This command orchestrates the full benchmark workflow:
    1. Starts the Kodit server and database
    2. Clones and indexes the repository
    3. Generates baseline prediction (no retrieval)
    4. Generates kodit prediction (with retrieval)
    5. Evaluates both predictions with SWE-bench harness
    6. Stops the server and cleans up

    INSTANCE_ID is the SWE-bench instance identifier (e.g., django__django-11049).

    Results are written to the results directory as JSONL files and a comparison
    JSON file summarizing both conditions.
    """
    api_key = require_api_key(api_key)
    log = structlog.get_logger(__name__)

    # Find instance
    loader = DatasetLoader()
    try:
        instance = loader.find(dataset_file, instance_id)
    except InstanceNotFoundError:
        log.exception("Instance not found", instance_id=instance_id)
        raise SystemExit(1) from None

    # Create batch config and runner
    config = BatchConfig(
        repos_dir=repos_dir,
        results_dir=results_dir,
        model=model,
        api_key=api_key,
        top_k=top_k,
        skip_evaluation=skip_evaluation,
        host=host,
        port=port,
        db_port=db_port,
        enrichment_base_url=enrichment_base_url,
        enrichment_model=enrichment_model,
        enrichment_parallel_tasks=enrichment_parallel_tasks,
        enrichment_timeout=enrichment_timeout,
    )

    runner = BatchRunner(config)
    result = runner.run([instance])

    # Print summary
    click.echo("\n" + "=" * 60)
    click.echo(f"Benchmark Results: {instance_id}")
    click.echo("=" * 60)
    click.echo(f"Succeeded: {result.succeeded}")
    click.echo(f"Failed: {result.failed}")
    click.echo("=" * 60)

    if result.failed > 0:
        raise SystemExit(1)


@cli.command("run-all")
@click.option(
    "--dataset-file",
    type=click.Path(path_type=Path, exists=True),
    default=DEFAULT_DATASET_FILE,
    help="Path to SWE-bench dataset JSON file",
)
@click.option(
    "--repos-dir",
    type=click.Path(path_type=Path),
    default=DEFAULT_REPOS_DIR,
    help="Directory to clone repositories into",
)
@click.option(
    "--results-dir",
    type=click.Path(path_type=Path),
    default=DEFAULT_RESULTS_DIR,
    help="Directory to write results to",
)
@click.option(
    "--model",
    default=DEFAULT_MODEL,
    help="LiteLLM model identifier for patch generation",
)
@click.option(
    "--api-key",
    envvar="ENRICHMENT_ENDPOINT_API_KEY",
    help="LLM API key (or set ENRICHMENT_ENDPOINT_API_KEY)",
)
@click.option(
    "--top-k",
    default=10,
    type=int,
    help="Number of snippets to retrieve for kodit condition",
)
@click.option(
    "--skip-evaluation",
    is_flag=True,
    help="Skip SWE-bench evaluation (only generate predictions)",
)
@click.option("--host", default=DEFAULT_HOST, help="Host to bind the server to")
@click.option("--port", default=DEFAULT_PORT, type=int, help="Port to bind the server")
@click.option("--db-port", default=DEFAULT_DB_PORT, type=int, help="Database port")
@click.option(
    "--enrichment-base-url",
    default=DEFAULT_ENRICHMENT_BASE_URL,
    help="Enrichment endpoint base URL",
)
@click.option(
    "--enrichment-model",
    default=DEFAULT_ENRICHMENT_MODEL,
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
    "--limit",
    default=None,
    type=int,
    help="Limit number of instances to run (for testing)",
)
def run_all(  # noqa: PLR0913
    dataset_file: Path,
    repos_dir: Path,
    results_dir: Path,
    model: str,
    api_key: str | None,
    top_k: int,
    skip_evaluation: bool,  # noqa: FBT001
    host: str,
    port: int,
    db_port: int,
    enrichment_base_url: str,
    enrichment_model: str,
    enrichment_parallel_tasks: int,
    enrichment_timeout: int,
    limit: int | None,
) -> None:
    """Run benchmarks for all instances in the dataset file.

    Reads the dataset JSON file and runs run-instance for each test ID
    sequentially. Results are tracked and a summary is printed at the end.
    """
    api_key = require_api_key(api_key)
    log = structlog.get_logger(__name__)

    # Load all instances from the dataset file
    loader = DatasetLoader()
    instances = loader.load(dataset_file)

    if limit:
        instances = instances[:limit]
        log.info("Limited instances", limit=limit)

    # Create batch config and runner
    config = BatchConfig(
        repos_dir=repos_dir,
        results_dir=results_dir,
        model=model,
        api_key=api_key,
        top_k=top_k,
        skip_evaluation=skip_evaluation,
        host=host,
        port=port,
        db_port=db_port,
        enrichment_base_url=enrichment_base_url,
        enrichment_model=enrichment_model,
        enrichment_parallel_tasks=enrichment_parallel_tasks,
        enrichment_timeout=enrichment_timeout,
    )

    def on_progress(instance_id: str, success: bool, current: int, total: int) -> None:  # noqa: FBT001
        status = "✓" if success else "✗"
        click.echo(f"[{current}/{total}] {status} {instance_id}")

    runner = BatchRunner(config)
    result = runner.run(instances, on_instance_complete=on_progress)

    # Print final summary
    click.echo("\n" + "=" * 60)
    click.echo("BENCHMARK RUN COMPLETE")
    click.echo("=" * 60)
    click.echo(f"Total instances: {result.total}")
    click.echo(f"Succeeded: {result.succeeded}")
    click.echo(f"Failed: {result.failed}")
    if result.failures:
        click.echo(f"Failed instances: {', '.join(result.failures)}")
    click.echo("=" * 60)

    if result.failed > 0:
        raise SystemExit(1)


# ============================================================================
# Mini-swe-agent commands
# ============================================================================

MINI_SWE_AGENT_OUTPUT_DIR = Path("benchmarks/minisweagent")
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
@click.option(
    "--workers",
    default=1,
    type=int,
    help="Number of parallel workers",
)
@click.option(
    "--top-k",
    default=10,
    type=int,
    help="Number of snippets to retrieve per instance",
)
@click.option("--host", default=DEFAULT_HOST, help="Kodit server host")
@click.option("--port", default=DEFAULT_PORT, type=int, help="Kodit server port")
@click.option("--db-port", default=DEFAULT_DB_PORT, type=int, help="Database port")
@click.option(
    "--enrichment-base-url",
    default=DEFAULT_ENRICHMENT_BASE_URL,
    help="Enrichment endpoint base URL",
)
@click.option(
    "--enrichment-model",
    default=DEFAULT_ENRICHMENT_MODEL,
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
    "--stream/--no-stream",
    default=True,
    help="Stream mini-swe-agent output to terminal instead of capturing",
)
@click.option(
    "--evaluate/--no-evaluate",
    default=True,
    help="Run SWE-bench evaluation after completion",
)
def mini_run_kodit(  # noqa: PLR0913, PLR0915, C901
    dataset_file: Path,
    output_dir: Path,
    repos_dir: Path,
    workers: int,
    top_k: int,
    host: str,
    port: int,
    db_port: int,
    enrichment_base_url: str,
    enrichment_model: str,
    enrichment_parallel_tasks: int,
    enrichment_timeout: int,
    limit: int | None,
    instance_id: str | None,
    api_key: str | None,
    stream: bool,  # noqa: FBT001
    evaluate: bool,  # noqa: FBT001
) -> None:
    """Run mini-swe-agent with Kodit retrieval.

    This runs mini-swe-agent against SWE-bench instances with problem
    statements augmented with Kodit-retrieved code context.

    For each instance, this command:
    1. Starts the Kodit server and database
    2. Clones the repository at the exact commit
    3. Indexes it with Kodit and waits for completion
    4. Retrieves relevant code snippets
    5. Augments the problem statement with the context
    6. Runs mini-swe-agent with the augmented problem statement
    7. Stops the Kodit server
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

    # Start Kodit server
    server = ServerProcess(
        host=host,
        port=port,
        db_port=db_port,
        enrichment_base_url=enrichment_base_url,
        enrichment_model=enrichment_model,
        enrichment_api_key=api_key,
        enrichment_parallel_tasks=enrichment_parallel_tasks,
        enrichment_timeout=enrichment_timeout,
    )

    log.info("Starting Kodit server")
    if not server.start():
        log.error("Failed to start Kodit server")
        raise SystemExit(1)

    base_url = f"http://{host}:{port}"

    try:
        log.info(
            "Running mini-swe-agent with Kodit",
            instance_count=len(instances),
            workers=workers,
            kodit_url=base_url,
            top_k=top_k,
            repos_dir=str(repos_dir),
        )

        # Create runner and config
        runner = MiniSweAgentRunner(kodit_base_url=base_url, top_k=top_k)
        config = RunConfig(
            config_path=MINI_SWE_AGENT_CONFIG_DIR / "kodit.yaml",
            output_dir=output_dir,
            repos_dir=repos_dir,
            workers=workers,
            api_key=api_key,
            stream_output=stream,
        )

        result = runner.run_with_kodit(config, instances)
    finally:
        log.info("Stopping Kodit server")
        server.stop()

    click.echo("\n" + "=" * 60)
    click.echo("MINI-SWE-AGENT WITH KODIT COMPLETE")
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
    "--baseline-preds",
    type=click.Path(path_type=Path, exists=True),
    default=MINI_SWE_AGENT_OUTPUT_DIR / "baseline" / "preds.json",
    help="Path to baseline predictions JSON",
)
@click.option(
    "--kodit-preds",
    type=click.Path(path_type=Path, exists=True),
    default=MINI_SWE_AGENT_OUTPUT_DIR / "kodit" / "preds.json",
    help="Path to Kodit predictions JSON",
)
@click.option(
    "--output",
    type=click.Path(path_type=Path),
    default=MINI_SWE_AGENT_OUTPUT_DIR / "comparison.json",
    help="Output JSON file for comparison results",
)
def mini_compare(
    baseline_preds: Path,
    kodit_preds: Path,
    output: Path,
) -> None:
    """Compare baseline and Kodit mini-swe-agent results.

    Generates a comparison report showing which instances were resolved
    by each condition.
    """
    log = structlog.get_logger(__name__)

    log.info(
        "Comparing results",
        baseline=str(baseline_preds),
        kodit=str(kodit_preds),
    )

    # Load predictions
    with baseline_preds.open() as f:
        baseline_data = json.load(f)

    with kodit_preds.open() as f:
        kodit_data = json.load(f)

    # Basic comparison (patch presence)
    baseline_instances = set(baseline_data.keys())
    kodit_instances = set(kodit_data.keys())

    both = baseline_instances & kodit_instances
    baseline_only = baseline_instances - kodit_instances
    kodit_only = kodit_instances - baseline_instances

    comparison = {
        "summary": {
            "baseline_total": len(baseline_instances),
            "kodit_total": len(kodit_instances),
            "both": len(both),
            "baseline_only": len(baseline_only),
            "kodit_only": len(kodit_only),
        },
        "instances": {
            "both": list(both),
            "baseline_only": list(baseline_only),
            "kodit_only": list(kodit_only),
        },
    }

    # Write comparison
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("w") as f:
        json.dump(comparison, f, indent=2)

    click.echo("\n" + "=" * 60)
    click.echo("COMPARISON RESULTS")
    click.echo("=" * 60)
    click.echo(f"Baseline instances: {len(baseline_instances)}")
    click.echo(f"Kodit instances: {len(kodit_instances)}")
    click.echo(f"Both completed: {len(both)}")
    click.echo(f"Output: {output}")
    click.echo("=" * 60)
    click.echo("\nNote: Run 'evaluate' on each predictions file to get")
    click.echo("resolution rates before final comparison.")


if __name__ == "__main__":
    cli()
