"""Evaluation utilities for SWE-bench predictions."""

import json
import re
import subprocess
from dataclasses import dataclass, field
from pathlib import Path

import structlog

from benchmark.swebench.instance import SWEBenchInstance
from benchmark.swebench.prediction import Prediction


@dataclass(frozen=True)
class InstanceResult:
    """Result for a single instance."""

    instance_id: str
    status: str  # "resolved", "error", "unresolved", "empty_patch"
    error_message: str | None = None


@dataclass(frozen=True)
class EvaluationResult:
    """Result of evaluating predictions against SWE-bench."""

    total_instances: int
    total_predictions: int
    valid_patches: int
    resolved: int
    file_match_count: int
    results_by_instance: dict[str, dict]
    instance_results: list[InstanceResult] = field(default_factory=list)

    @property
    def resolve_rate(self) -> float:
        """Percentage of instances resolved."""
        if self.total_predictions == 0:
            return 0.0
        return self.resolved / self.total_predictions

    @property
    def valid_patch_rate(self) -> float:
        """Percentage of predictions with valid patches."""
        if self.total_predictions == 0:
            return 0.0
        return self.valid_patches / self.total_predictions

    @property
    def file_match_rate(self) -> float:
        """Percentage of predictions that modify correct files."""
        if self.total_predictions == 0:
            return 0.0
        return self.file_match_count / self.total_predictions

    def as_dict(self) -> dict:
        """Convert to dictionary for JSON serialization."""
        return {
            "total_instances": self.total_instances,
            "total_predictions": self.total_predictions,
            "valid_patches": self.valid_patches,
            "resolved": self.resolved,
            "resolve_rate": self.resolve_rate,
            "valid_patch_rate": self.valid_patch_rate,
            "file_match_count": self.file_match_count,
            "file_match_rate": self.file_match_rate,
            "instance_results": [
                {
                    "instance_id": r.instance_id,
                    "status": r.status,
                    "error_message": r.error_message,
                }
                for r in self.instance_results
            ],
        }


class PredictionLoader:
    """Loads predictions from JSONL file."""

    def load(self, path: Path) -> list[Prediction]:
        """Load predictions from JSONL file."""
        predictions = []
        with path.open() as f:
            for raw_line in f:
                stripped = raw_line.strip()
                if not stripped:
                    continue
                data = json.loads(stripped)
                prediction = Prediction(
                    instance_id=data["instance_id"],
                    model_name_or_path=data["model_name_or_path"],
                    model_patch=data["model_patch"],
                )
                predictions.append(prediction)
        return predictions


class Evaluator:
    """Evaluates SWE-bench predictions."""

    def __init__(self) -> None:
        """Initialize evaluator."""
        self._log = structlog.get_logger(__name__)

    def evaluate_quick(
        self,
        predictions: list[Prediction],
        instances: list[SWEBenchInstance],
    ) -> EvaluationResult:
        """Quick evaluation without running tests.

        Computes metrics that don't require Docker:
        - Valid patch rate (predictions with parseable diffs)
        - File match rate (predictions that modify correct files)
        """
        instance_map = {i.instance_id: i for i in instances}
        results_by_instance: dict[str, dict] = {}

        valid_patches = 0
        file_match_count = 0

        for prediction in predictions:
            instance = instance_map.get(prediction.instance_id)
            if instance is None:
                self._log.warning(
                    "Instance not found for prediction",
                    instance_id=prediction.instance_id,
                )
                continue

            # Check if patch is valid
            is_valid = self._is_valid_patch(prediction.model_patch)
            if is_valid:
                valid_patches += 1

            # Check if patch modifies correct files
            predicted_files = self._extract_files_from_patch(prediction.model_patch)
            gold_files = self._extract_files_from_patch(instance.patch)
            files_match = bool(predicted_files & gold_files)
            if files_match:
                file_match_count += 1

            results_by_instance[prediction.instance_id] = {
                "valid_patch": is_valid,
                "predicted_files": list(predicted_files),
                "gold_files": list(gold_files),
                "files_match": files_match,
                "resolved": False,  # Unknown without running tests
            }

        return EvaluationResult(
            total_instances=len(instances),
            total_predictions=len(predictions),
            valid_patches=valid_patches,
            resolved=0,  # Unknown without running tests
            file_match_count=file_match_count,
            results_by_instance=results_by_instance,
        )

    def evaluate_full(
        self,
        predictions_path: Path,
        dataset_name: str = "princeton-nlp/SWE-bench_Lite",
        max_workers: int = 4,
        run_id: str = "kodit_eval",
    ) -> EvaluationResult:
        """Full evaluation using SWE-bench harness.

        Requires Docker and the swebench package installed.
        """
        # Check if swebench is installed
        if not self._is_swebench_available():
            msg = "swebench package not installed. Install with: pip install swebench"
            raise EvaluationError(msg)

        # Pull Docker images for all instances in predictions
        self._pull_docker_images(predictions_path, dataset_name)

        self._log.info(
            "Running SWE-bench evaluation",
            predictions_path=str(predictions_path),
            dataset_name=dataset_name,
            max_workers=max_workers,
            run_id=run_id,
        )

        # Run the SWE-bench harness with streaming output
        cmd = [
            "python",
            "-m",
            "swebench.harness.run_evaluation",
            "--dataset_name",
            dataset_name,
            "--predictions_path",
            str(predictions_path),
            "--max_workers",
            str(max_workers),
            "--run_id",
            run_id,
        ]

        process = subprocess.Popen(  # noqa: S603
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
        )

        # Stream output line by line
        output_lines = []
        if process.stdout:
            for raw_line in process.stdout:
                stripped = raw_line.rstrip()
                output_lines.append(stripped)
                self._log.info("swebench", output=stripped)

        returncode = process.wait()

        if returncode != 0:
            self._log.error(
                "SWE-bench evaluation failed",
                returncode=returncode,
            )
            msg = f"SWE-bench evaluation failed with code {returncode}"
            raise EvaluationError(msg)

        # Parse results from output
        return self._parse_harness_output(run_id)

    def _is_swebench_available(self) -> bool:
        """Check if swebench package is installed."""
        result = subprocess.run(  # noqa: S603
            ["python", "-c", "import swebench"],  # noqa: S607
            capture_output=True,
            check=False,
        )
        return result.returncode == 0

    def _pull_docker_images(
        self,
        predictions_path: Path,
        dataset_name: str,
    ) -> None:
        """Pull Docker images for instances in predictions file."""
        # Load instance IDs from predictions
        instance_ids = []
        with predictions_path.open() as f:
            for raw_line in f:
                stripped = raw_line.strip()
                if not stripped:
                    continue
                data = json.loads(stripped)
                instance_ids.append(data["instance_id"])

        if not instance_ids:
            return

        self._log.info(
            "Pulling Docker images for evaluation",
            instance_count=len(instance_ids),
        )

        # Use swebench's docker_build to pull/build images
        cmd = [
            "python",
            "-m",
            "swebench.harness.docker_build",
            "--dataset_name",
            dataset_name,
            "--instance_ids",
            *instance_ids,
        ]

        process = subprocess.Popen(  # noqa: S603
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
        )

        if process.stdout:
            for raw_line in process.stdout:
                stripped = raw_line.rstrip()
                self._log.info("docker_build", output=stripped)

        returncode = process.wait()

        if returncode != 0:
            self._log.warning(
                "Docker image pull had non-zero exit code",
                returncode=returncode,
            )

    def _is_valid_patch(self, patch: str) -> bool:
        """Check if patch is a valid unified diff."""
        if not patch or not patch.strip():
            return False
        # Must contain diff header
        return "diff --git" in patch or patch.startswith("---")

    def _extract_files_from_patch(self, patch: str) -> set[str]:
        """Extract file paths from a unified diff."""
        files = set()
        # Match 'diff --git a/path b/path' or '--- a/path'
        for match in re.finditer(r"diff --git a/(.+?) b/", patch):
            files.add(match.group(1))
        for match in re.finditer(r"^--- a/(.+)$", patch, re.MULTILINE):
            files.add(match.group(1))
        return files

    def _parse_harness_output(
        self,
        run_id: str,
    ) -> EvaluationResult:
        """Parse SWE-bench harness output to extract results."""
        # SWE-bench outputs results to {model_name}.{run_id}.json in cwd
        # Find the results file by matching the run_id
        results_file = None
        for path in Path.cwd().glob(f"*.{run_id}.json"):
            results_file = path
            break

        resolved = 0
        submitted = 0
        total = 0
        instance_results: list[InstanceResult] = []

        if results_file and results_file.exists():
            self._log.info("Found results file", path=str(results_file))
            with results_file.open() as f:
                data = json.load(f)
                resolved = data.get("resolved_instances", 0)
                submitted = data.get("submitted_instances", 0)
                total = data.get("total_instances", 0)

                # Extract per-instance results
                resolved_ids = set(data.get("resolved_ids", []))
                error_ids = set(data.get("error_ids", []))
                unresolved_ids = set(data.get("unresolved_ids", []))
                empty_patch_ids = set(data.get("empty_patch_ids", []))

                # Log resolved instances
                for instance_id in resolved_ids:
                    self._log.info(
                        "Instance PASSED",
                        instance_id=instance_id,
                        status="resolved",
                    )
                    instance_results.append(
                        InstanceResult(instance_id=instance_id, status="resolved")
                    )

                # Log error instances
                for instance_id in error_ids:
                    self._log.warning(
                        "Instance FAILED",
                        instance_id=instance_id,
                        status="error",
                        reason="evaluation error (check Docker logs)",
                    )
                    instance_results.append(
                        InstanceResult(
                            instance_id=instance_id,
                            status="error",
                            error_message="evaluation error (check Docker logs)",
                        )
                    )

                # Log unresolved instances
                for instance_id in unresolved_ids:
                    self._log.warning(
                        "Instance FAILED",
                        instance_id=instance_id,
                        status="unresolved",
                        reason="tests did not pass",
                    )
                    instance_results.append(
                        InstanceResult(
                            instance_id=instance_id,
                            status="unresolved",
                            error_message="tests did not pass",
                        )
                    )

                # Log empty patch instances
                for instance_id in empty_patch_ids:
                    self._log.warning(
                        "Instance FAILED",
                        instance_id=instance_id,
                        status="empty_patch",
                        reason="no patch generated",
                    )
                    instance_results.append(
                        InstanceResult(
                            instance_id=instance_id,
                            status="empty_patch",
                            error_message="no patch generated",
                        )
                    )
        else:
            self._log.warning("Results file not found", run_id=run_id)

        return EvaluationResult(
            total_instances=total,
            total_predictions=submitted,
            valid_patches=submitted,  # Assume all valid if harness ran
            resolved=resolved,
            file_match_count=0,  # Not computed in full eval
            results_by_instance={},
            instance_results=instance_results,
        )


class EvaluationError(Exception):
    """Raised when evaluation fails."""
