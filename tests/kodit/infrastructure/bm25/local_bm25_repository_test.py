"""Tests for the LocalBM25Repository."""

import json
import tempfile
from collections.abc import Generator
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from kodit.domain.value_objects import (
    Document,
    IndexRequest,
    SearchRequest,
)
from kodit.infrastructure.bm25.local_bm25_repository import LocalBM25Repository


class TestLocalBM25Repository:
    """Test cases for LocalBM25Repository."""

    @pytest.fixture
    def temp_dir(self) -> Generator[Path, None, None]:
        """Create a temporary directory for testing."""
        with tempfile.TemporaryDirectory() as temp_dir:
            yield Path(temp_dir)

    @pytest.fixture
    def repository(self, temp_dir: Path) -> LocalBM25Repository:
        """Create a LocalBM25Repository instance."""
        return LocalBM25Repository(temp_dir)

    @pytest.mark.asyncio
    async def test_index_documents_extends_snippet_ids_instead_of_resetting(
        self, repository: LocalBM25Repository, temp_dir: Path
    ) -> None:
        """Test that indexing new documents extends snippet_ids."""
        # Setup: Create initial documents and index them
        initial_documents = [
            Document(snippet_id=1, text="first document content"),
            Document(snippet_id=2, text="second document content"),
        ]
        initial_request = IndexRequest(documents=initial_documents)

        # Mock the BM25 retriever to avoid actual indexing
        mock_retriever = MagicMock()
        mock_retriever.index.return_value = None
        mock_retriever.save.return_value = None
        mock_retriever.scores = {"num_docs": 2}

        # Create the directory structure
        (temp_dir / "bm25s_index").mkdir(parents=True, exist_ok=True)

        with patch.object(repository, "_retriever", return_value=mock_retriever):
            # Index initial documents
            await repository.index_documents(initial_request)

        # Verify initial snippet_ids were saved
        snippet_ids_file = temp_dir / "bm25s_index" / "snippet_ids.jsonl"
        assert snippet_ids_file.exists()

        with snippet_ids_file.open() as f:
            saved_snippet_ids = json.load(f)
        assert saved_snippet_ids == [1, 2]

        # Now add new documents - this should EXTEND the existing snippet_ids
        new_documents = [
            Document(snippet_id=3, text="third document content"),
            Document(snippet_id=4, text="fourth document content"),
        ]
        new_request = IndexRequest(documents=new_documents)

        # Mock the retriever again for the second indexing
        mock_retriever.scores = {"num_docs": 4}  # Updated count

        with patch.object(repository, "_retriever", return_value=mock_retriever):
            # Index new documents
            await repository.index_documents(new_request)

        # Verify snippet_ids were extended, not reset
        with snippet_ids_file.open() as f:
            final_snippet_ids = json.load(f)

        # EXPECTED: [1, 2, 3, 4] - snippet_ids should be extended
        # ACTUAL: [3, 4] - snippet_ids are reset (this is the bug)
        assert final_snippet_ids == [1, 2, 3, 4], (
            f"Expected snippet_ids to be extended to [1, 2, 3, 4], "
            f"but got {final_snippet_ids}. This indicates the bug where "
            f"snippet_ids are reset instead of extended."
        )

    @pytest.mark.asyncio
    async def test_search_after_incremental_indexing_works_correctly(
        self,
        repository: LocalBM25Repository,
        temp_dir: Path,
    ) -> None:
        """Test that search works correctly after incremental indexing."""
        # Setup: Index initial documents
        documents = [
            Document(snippet_id=1, text="first document content"),
            Document(snippet_id=2, text="second document content"),
        ]
        request = IndexRequest(documents=documents)

        # Mock the BM25 retriever
        mock_retriever = MagicMock()
        mock_retriever.index.return_value = None
        mock_retriever.save.return_value = None
        mock_retriever.scores = {"num_docs": 2}

        # Create the directory structure
        (temp_dir / "bm25s_index").mkdir(parents=True, exist_ok=True)

        with patch.object(repository, "_retriever", return_value=mock_retriever):
            # Index initial documents
            await repository.index_documents(request)

        # Verify initial snippet_ids were saved
        snippet_ids_file = temp_dir / "bm25s_index" / "snippet_ids.jsonl"
        assert snippet_ids_file.exists()
        with snippet_ids_file.open() as f:
            saved_snippet_ids = json.load(f)
        assert saved_snippet_ids == [1, 2]

        # Now add new documents - this should EXTEND the existing snippet_ids
        new_documents = [
            Document(snippet_id=3, text="third document content"),
            Document(snippet_id=4, text="fourth document content"),
        ]
        new_request = IndexRequest(documents=new_documents)

        # Mock the retriever again for the second indexing
        mock_retriever.scores = {"num_docs": 4}  # Updated count

        with patch.object(repository, "_retriever", return_value=mock_retriever):
            # Index new documents
            await repository.index_documents(new_request)

        # Verify snippet_ids were extended, not reset
        with snippet_ids_file.open() as f:
            final_snippet_ids = json.load(f)

        assert final_snippet_ids == [1, 2, 3, 4]

    @pytest.mark.asyncio
    async def test_search_handles_actual_snippet_ids_not_indices(
        self,
        repository: LocalBM25Repository,
        temp_dir: Path,
    ) -> None:
        """Test that search correctly handles BM25 returning actual snippet IDs."""
        # Setup: Index documents with non-sequential snippet IDs
        documents = [
            Document(snippet_id=100, text="python programming language"),
            Document(snippet_id=200, text="javascript web development"),
            Document(snippet_id=300, text="java enterprise applications"),
        ]
        request = IndexRequest(documents=documents)

        # Mock the BM25 retriever
        mock_retriever = MagicMock()
        mock_retriever.index.return_value = None
        mock_retriever.save.return_value = None
        mock_retriever.scores = {"num_docs": 3}

        # IMPORTANT: Mock retrieve to return ACTUAL snippet IDs, not indices
        # This simulates what the real BM25 library does
        mock_retriever.retrieve.return_value = (
            [[200, 100]],  # Returns actual snippet IDs: 200, 100
            [[0.9, 0.8]],  # Corresponding scores
        )

        # Create the directory structure
        (temp_dir / "bm25s_index").mkdir(parents=True, exist_ok=True)

        with patch.object(repository, "_retriever", return_value=mock_retriever):
            await repository.index_documents(request)

        # Search for content
        search_request = SearchRequest(query="programming", top_k=10)

        with patch.object(repository, "_retriever", return_value=mock_retriever):
            results = await repository.search(search_request)

        # Verify the results
        assert len(results) == 2

        # The results should have the correct snippet IDs
        result_snippet_ids = [result.snippet_id for result in results]
        assert 200 in result_snippet_ids, "Should find snippet with ID 200"
        assert 100 in result_snippet_ids, "Should find snippet with ID 100"

        # The scores should match the expected order
        assert results[0].snippet_id == 200, (
            "First result should be snippet 200 (higher score)"
        )
        assert results[1].snippet_id == 100, (
            "Second result should be snippet 100 (lower score)"
        )
        assert results[0].score == 0.9, "First result should have score 0.9"
        assert results[1].score == 0.8, "Second result should have score 0.8"
