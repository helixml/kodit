"""Extract patches from mini-swe-agent trajectories when model_patch is empty."""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from typing import TYPE_CHECKING

import structlog

if TYPE_CHECKING:
    from pathlib import Path


@dataclass(frozen=True)
class ExtractionResult:
    """Result of extracting a patch from a trajectory."""

    instance_id: str
    patch: str
    source: str


class PatchExtractor:
    """Extracts patches from trajectory messages when submission detection fails."""

    def __init__(self, output_dir: Path) -> None:
        """Initialize with the mini-swe-agent output directory."""
        self._output_dir = output_dir
        self._log = structlog.get_logger(__name__)
        self._diff_pattern = re.compile(r"diff --git.*?(?=diff --git|\Z)", re.DOTALL)

    def extract_and_update(self) -> list[ExtractionResult]:
        """Extract patches for empty predictions and update preds.json."""
        preds_path = self._output_dir / "preds.json"
        if not preds_path.exists():
            self._log.warning("Predictions file not found", path=str(preds_path))
            return []

        predictions = self._load_predictions(preds_path)
        results: list[ExtractionResult] = []

        for instance_id, pred in predictions.items():
            model_patch = pred.get("model_patch", "")
            if model_patch and model_patch.strip():
                continue

            trajectory = self._load_trajectory(instance_id)
            if not trajectory:
                continue

            patch = self._extract_from_trajectory(trajectory)
            if patch:
                pred["model_patch"] = patch
                results.append(
                    ExtractionResult(
                        instance_id=instance_id,
                        patch=patch,
                        source="trajectory",
                    )
                )
                self._log.info(
                    "Extracted patch from trajectory",
                    instance_id=instance_id,
                    patch_length=len(patch),
                )

        if results:
            self._save_predictions(preds_path, predictions)
            self._log.info(
                "Updated predictions with extracted patches",
                count=len(results),
            )

        return results

    def _load_predictions(self, path: Path) -> dict:
        """Load predictions from preds.json."""
        with path.open() as f:
            return json.load(f)

    def _save_predictions(self, path: Path, predictions: dict) -> None:
        """Save predictions to preds.json."""
        with path.open("w") as f:
            json.dump(predictions, f, indent=2)

    def _load_trajectory(self, instance_id: str) -> dict | None:
        """Load trajectory for a specific instance."""
        traj_path = self._output_dir / instance_id / f"{instance_id}.traj.json"
        if not traj_path.exists():
            self._log.debug("Trajectory not found", instance_id=instance_id)
            return None

        try:
            with traj_path.open() as f:
                return json.load(f)
        except json.JSONDecodeError as e:
            self._log.warning(
                "Failed to parse trajectory",
                instance_id=instance_id,
                error=str(e),
            )
            return None

    def _extract_from_trajectory(self, trajectory: dict) -> str:
        """Extract the last git diff from trajectory messages."""
        messages = trajectory.get("messages", [])
        diffs: list[str] = []

        for message in messages:
            if message.get("role") != "user":
                continue

            content = message.get("content", "")
            extracted = self._extract_diff_from_output(content)
            if extracted:
                diffs.append(extracted)

        if diffs:
            return diffs[-1]
        return ""

    def _extract_diff_from_output(self, content: str) -> str:
        """Extract git diff content from an output message."""
        output_match = re.search(r"<output>(.*?)</output>", content, re.DOTALL)
        if not output_match:
            return ""

        output = output_match.group(1)
        if "diff --git" not in output:
            return ""

        matches = self._diff_pattern.findall(output)
        if not matches:
            return ""

        return "".join(matches).strip()
