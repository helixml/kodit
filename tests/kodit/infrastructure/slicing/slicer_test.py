"""Tests for the Slicer."""

from datetime import UTC, datetime
from pathlib import Path

import pytest

from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.slicer import Slicer


class TestSlicer:
    """Test the Slicer functionality."""

    @pytest.mark.parametrize(
        ("language", "extension"),
        [
            ("c", ".c"),
            ("cpp", ".cpp"),
            ("csharp", ".cs"),
            ("go", ".go"),
            ("java", ".java"),
            ("javascript", ".js"),
            ("python", ".py"),
            ("rust", ".rs"),
        ],
    )
    def test_extract_snippets_from_language(
        self, language: str, extension: str
    ) -> None:
        """Test extracting snippets from supported languages."""
        data_dir = Path(__file__).parent / "data" / language
        files = [f for f in data_dir.glob(f"*{extension}") if f.is_file()]

        git_files = [
            GitFile(
                created_at=datetime.now(tz=UTC),
                blob_sha=f"sha_{f.name}",
                commit_sha="test_commit",
                path=str(f),
                mime_type="text/plain",
                size=f.stat().st_size,
                extension=extension,
            )
            for f in files
        ]

        slicer = Slicer()
        snippets = slicer.extract_snippets_from_git_files(git_files, language)

        assert len(snippets) > 0
        assert all(snippet.extension == language for snippet in snippets)
