"""SWE-bench dataset loader from HuggingFace."""

import json
from pathlib import Path

import structlog
from datasets import load_dataset  # type: ignore[import-untyped]

from benchmark.swebench.instance import SWEBenchInstance

DATASET_VARIANTS = {
    "lite": "princeton-nlp/SWE-bench_Lite",
    "verified": "princeton-nlp/SWE-bench_Verified",
}


def _parse_tests(value: str | list | None) -> tuple[str, ...]:
    """Parse test list from HuggingFace format (JSON string or list)."""
    if value is None:
        return ()
    if isinstance(value, list):
        return tuple(value)
    return tuple(json.loads(value))


class DatasetLoader:
    """Loads SWE-bench datasets from HuggingFace."""

    def __init__(self, cache_dir: Path | None = None) -> None:
        """Initialize loader with optional cache directory."""
        self._cache_dir = cache_dir
        self._log = structlog.get_logger(__name__)

    def download(self, variant: str = "lite") -> list[SWEBenchInstance]:
        """Download dataset from HuggingFace and return instances."""
        dataset_name = DATASET_VARIANTS.get(variant)
        if dataset_name is None:
            available = ", ".join(DATASET_VARIANTS.keys())
            msg = f"Unknown variant '{variant}'. Available: {available}"
            raise ValueError(msg)

        self._log.info("Downloading dataset", dataset=dataset_name)

        dataset = load_dataset(
            dataset_name,
            split="test",
            cache_dir=str(self._cache_dir) if self._cache_dir else None,
        )

        instances = []
        for row in dataset:
            instance = SWEBenchInstance(
                instance_id=row["instance_id"],
                repo=row["repo"],
                base_commit=row["base_commit"],
                problem_statement=row["problem_statement"],
                hints_text=row.get("hints_text", "") or "",
                patch=row["patch"],
                test_patch=row["test_patch"],
                fail_to_pass=_parse_tests(row.get("FAIL_TO_PASS")),
                pass_to_pass=_parse_tests(row.get("PASS_TO_PASS")),
                version=row["version"],
                environment_setup_commit=row["environment_setup_commit"],
            )
            instances.append(instance)

        self._log.info("Loaded instances", count=len(instances))
        return instances

    def save(self, instances: list[SWEBenchInstance], output_path: Path) -> None:
        """Save instances to JSON file."""
        output_path.parent.mkdir(parents=True, exist_ok=True)

        data = {
            "version": "1.0",
            "count": len(instances),
            "instances": [inst.as_dict() for inst in instances],
        }

        with output_path.open("w") as f:
            json.dump(data, f, indent=2)

        self._log.info("Saved dataset", path=str(output_path), count=len(instances))

    def load(self, input_path: Path) -> list[SWEBenchInstance]:
        """Load instances from JSON file."""
        with input_path.open() as f:
            data = json.load(f)

        instances = [SWEBenchInstance.from_dict(d) for d in data["instances"]]
        self._log.info("Loaded from file", path=str(input_path), count=len(instances))
        return instances

    def find(self, input_path: Path, instance_id: str) -> SWEBenchInstance:
        """Load dataset and find a specific instance by ID."""
        instances = self.load(input_path)
        instance = next((i for i in instances if i.instance_id == instance_id), None)
        if instance is None:
            msg = f"Instance not found: {instance_id}"
            raise InstanceNotFoundError(msg)
        return instance


class InstanceNotFoundError(Exception):
    """Raised when an instance is not found in the dataset."""
