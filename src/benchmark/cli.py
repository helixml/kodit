"""Command line interface for kodit benchmarks."""

import json
from pathlib import Path

import click
import structlog

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
from benchmark.swebench.generator import BenchmarkRunner, PatchGenerator
from benchmark.swebench.loader import DatasetLoader
from benchmark.swebench.prediction import PredictionWriter
from benchmark.swebench.repository import (
    DEFAULT_REPOS_DIR,
    RepositoryCloneError,
    RepositoryIndexError,
    RepositoryPreparer,
)
from benchmark.swebench.retriever import KoditRetriever
from kodit.config import AppContext
from kodit.log import configure_logging

DEFAULT_OUTPUT_DIR = Path("benchmarks/data")
DEFAULT_RESULTS_DIR = Path("benchmarks/results")
DEFAULT_DATASET_FILE = DEFAULT_OUTPUT_DIR / "swebench-lite.json"
DEFAULT_MODEL = "openrouter/qwen/qwen3-coder-flash"


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

    # Load dataset and find instance
    loader = DatasetLoader()
    instances = loader.load(dataset_file)

    instance = next((i for i in instances if i.instance_id == instance_id), None)
    if instance is None:
        log.error("Instance not found", instance_id=instance_id)
        raise SystemExit(1)

    log.info(
        "Found instance",
        instance_id=instance.instance_id,
        repo=instance.repo,
        commit=instance.base_commit[:12],
    )

    # Determine Kodit URL
    base_url = kodit_url or f"http://{host}:{port}"

    # Prepare repository
    preparer = RepositoryPreparer(
        kodit_base_url=base_url,
        repos_dir=repos_dir,
    )

    try:
        repo_id = preparer.prepare(instance)
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

    # Determine output path
    if output is None:
        output = DEFAULT_RESULTS_DIR / f"{condition}.jsonl"

    # Load dataset and find instance
    loader = DatasetLoader()
    instances = loader.load(dataset_file)

    instance = next((i for i in instances if i.instance_id == instance_id), None)
    if instance is None:
        log.error("Instance not found", instance_id=instance_id)
        raise SystemExit(1)

    log.info(
        "Generating prediction",
        instance_id=instance.instance_id,
        condition=condition,
        model=model,
    )

    # Create generator
    generator = PatchGenerator(model=model, api_key=api_key)

    # Create retriever if needed
    retriever = None
    if condition == "kodit":
        base_url = kodit_url or f"http://{host}:{port}"
        retriever = KoditRetriever(kodit_base_url=base_url)

    # Run prediction
    runner = BenchmarkRunner(generator=generator, retriever=retriever)

    if condition == "baseline":
        prediction = runner.run_baseline(instance)
    else:
        prediction = runner.run_kodit(instance, top_k=top_k)

    # Write prediction
    writer = PredictionWriter(output)
    writer.write(prediction)

    log.info(
        "Prediction written",
        output=str(output),
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


if __name__ == "__main__":
    cli()
