"""Tests for PatchExtractor."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING

import pytest

from benchmark.minisweagent.patch_extractor import PatchExtractor

if TYPE_CHECKING:
    from pathlib import Path


@pytest.fixture
def output_dir(tmp_path: Path) -> Path:
    """Create a temporary output directory with predictions and trajectories."""
    return tmp_path


@pytest.fixture
def sample_diff() -> str:
    """Return sample git diff output."""
    return """diff --git a/src/foo.py b/src/foo.py
index abc1234..def5678 100644
--- a/src/foo.py
+++ b/src/foo.py
@@ -10,6 +10,7 @@ def example():
     x = 1
+    y = 2
     return x
"""


def _make_output_message(content: str) -> dict:
    """Create a user message with output tags."""
    return {
        "role": "user",
        "content": f"<returncode>0</returncode>\n<output>\n{content}</output>",
    }


class TestPatchExtractor:
    """Tests for PatchExtractor."""

    def test_no_extraction_when_patch_exists(self, output_dir: Path) -> None:
        """Verify no extraction when model_patch is already present."""
        preds = {
            "instance_1": {
                "model_name_or_path": "test-model",
                "instance_id": "instance_1",
                "model_patch": "existing patch content",
            }
        }
        preds_path = output_dir / "preds.json"
        with preds_path.open("w") as f:
            json.dump(preds, f)

        extractor = PatchExtractor(output_dir)
        results = extractor.extract_and_update()

        assert len(results) == 0

    def test_extracts_patch_from_trajectory(
        self, output_dir: Path, sample_diff: str
    ) -> None:
        """Extract patch from trajectory when model_patch is empty."""
        preds = {
            "instance_1": {
                "model_name_or_path": "test-model",
                "instance_id": "instance_1",
                "model_patch": "",
            }
        }
        preds_path = output_dir / "preds.json"
        with preds_path.open("w") as f:
            json.dump(preds, f)

        trajectory = {
            "messages": [
                {"role": "system", "content": "You are a helpful assistant."},
                {"role": "assistant", "content": "Let me check the code."},
                _make_output_message(sample_diff),
            ]
        }
        traj_dir = output_dir / "instance_1"
        traj_dir.mkdir()
        traj_path = traj_dir / "instance_1.traj.json"
        with traj_path.open("w") as f:
            json.dump(trajectory, f)

        extractor = PatchExtractor(output_dir)
        results = extractor.extract_and_update()

        assert len(results) == 1
        assert results[0].instance_id == "instance_1"
        assert "diff --git" in results[0].patch

        with preds_path.open() as f:
            updated_preds = json.load(f)
        assert "diff --git" in updated_preds["instance_1"]["model_patch"]

    def test_extracts_last_diff_from_multiple_outputs(
        self, output_dir: Path
    ) -> None:
        """Extract the last git diff when multiple are present."""
        first_diff = """diff --git a/old.py b/old.py
--- a/old.py
+++ b/old.py
@@ -1 +1 @@
-old
+new
"""
        second_diff = """diff --git a/final.py b/final.py
--- a/final.py
+++ b/final.py
@@ -1 +1 @@
-before
+after
"""
        preds = {
            "instance_1": {
                "model_name_or_path": "test-model",
                "instance_id": "instance_1",
                "model_patch": "",
            }
        }
        preds_path = output_dir / "preds.json"
        with preds_path.open("w") as f:
            json.dump(preds, f)

        trajectory = {
            "messages": [
                _make_output_message(first_diff),
                {"role": "assistant", "content": "Making more changes."},
                _make_output_message(second_diff),
            ]
        }
        traj_dir = output_dir / "instance_1"
        traj_dir.mkdir()
        traj_path = traj_dir / "instance_1.traj.json"
        with traj_path.open("w") as f:
            json.dump(trajectory, f)

        extractor = PatchExtractor(output_dir)
        results = extractor.extract_and_update()

        assert len(results) == 1
        assert "final.py" in results[0].patch
        assert "old.py" not in results[0].patch

    def test_handles_missing_predictions_file(self, output_dir: Path) -> None:
        """Handle missing predictions file gracefully."""
        extractor = PatchExtractor(output_dir)
        results = extractor.extract_and_update()

        assert len(results) == 0

    def test_handles_missing_trajectory(self, output_dir: Path) -> None:
        """Handle missing trajectory file gracefully."""
        preds = {
            "instance_1": {
                "model_name_or_path": "test-model",
                "instance_id": "instance_1",
                "model_patch": "",
            }
        }
        preds_path = output_dir / "preds.json"
        with preds_path.open("w") as f:
            json.dump(preds, f)

        extractor = PatchExtractor(output_dir)
        results = extractor.extract_and_update()

        assert len(results) == 0

    def test_handles_trajectory_without_diff(self, output_dir: Path) -> None:
        """Handle trajectories that don't contain git diff output."""
        preds = {
            "instance_1": {
                "model_name_or_path": "test-model",
                "instance_id": "instance_1",
                "model_patch": "",
            }
        }
        preds_path = output_dir / "preds.json"
        with preds_path.open("w") as f:
            json.dump(preds, f)

        trajectory = {
            "messages": [
                _make_output_message("some other output"),
            ]
        }
        traj_dir = output_dir / "instance_1"
        traj_dir.mkdir()
        traj_path = traj_dir / "instance_1.traj.json"
        with traj_path.open("w") as f:
            json.dump(trajectory, f)

        extractor = PatchExtractor(output_dir)
        results = extractor.extract_and_update()

        assert len(results) == 0

    def test_combines_multiple_diff_files(self, output_dir: Path) -> None:
        """Combine multiple diff --git sections from one output."""
        multi_file_diff = """diff --git a/foo.py b/foo.py
--- a/foo.py
+++ b/foo.py
@@ -1 +1 @@
-old
+new
diff --git a/bar.py b/bar.py
--- a/bar.py
+++ b/bar.py
@@ -1 +1 @@
-before
+after
"""
        preds = {
            "instance_1": {
                "model_name_or_path": "test-model",
                "instance_id": "instance_1",
                "model_patch": "",
            }
        }
        preds_path = output_dir / "preds.json"
        with preds_path.open("w") as f:
            json.dump(preds, f)

        trajectory = {
            "messages": [
                _make_output_message(multi_file_diff),
            ]
        }
        traj_dir = output_dir / "instance_1"
        traj_dir.mkdir()
        traj_path = traj_dir / "instance_1.traj.json"
        with traj_path.open("w") as f:
            json.dump(trajectory, f)

        extractor = PatchExtractor(output_dir)
        results = extractor.extract_and_update()

        assert len(results) == 1
        assert "foo.py" in results[0].patch
        assert "bar.py" in results[0].patch
