"""Prediction representation for SWE-bench evaluation."""

import json
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Prediction:
    """A single prediction for SWE-bench evaluation."""

    instance_id: str
    model_name_or_path: str
    model_patch: str

    def as_dict(self) -> dict:
        """Convert to dictionary for JSON serialization."""
        return {
            "instance_id": self.instance_id,
            "model_name_or_path": self.model_name_or_path,
            "model_patch": self.model_patch,
        }

    def to_jsonl_line(self) -> str:
        """Convert to JSONL format line."""
        return json.dumps(self.as_dict())


class PredictionWriter:
    """Writes predictions to JSONL file for SWE-bench evaluation."""

    def __init__(self, output_path: Path) -> None:
        """Initialize writer with output path."""
        self._output_path = output_path
        self._output_path.parent.mkdir(parents=True, exist_ok=True)

    def write(self, prediction: Prediction) -> None:
        """Append a single prediction to the output file."""
        with self._output_path.open("a") as f:
            f.write(prediction.to_jsonl_line() + "\n")

    def write_all(self, predictions: list[Prediction]) -> None:
        """Write all predictions to the output file (overwrites existing)."""
        with self._output_path.open("w") as f:
            for prediction in predictions:
                f.write(prediction.to_jsonl_line() + "\n")

    @property
    def path(self) -> Path:
        """Return the output path."""
        return self._output_path
