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
    async def test_index_documents_replaces_snippet_ids_with_new_index(
        self, repository: LocalBM25Repository, temp_dir: Path
    ) -> None:
        """Test that indexing replaces snippet_ids since BM25 index is rebuilt."""
        # Setup: Create initial documents and index them
        initial_documents = [
            Document(snippet_id="1", text="first document content"),
            Document(snippet_id="2", text="second document content"),
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
        assert saved_snippet_ids == ["1", "2"]

        # Now add new documents - this should EXTEND the existing snippet_ids
        new_documents = [
            Document(snippet_id="3", text="third document content"),
            Document(snippet_id="4", text="fourth document content"),
        ]
        new_request = IndexRequest(documents=new_documents)

        # Mock the retriever again for the second indexing
        mock_retriever.scores = {"num_docs": 4}  # Updated count

        with patch.object(repository, "_retriever", return_value=mock_retriever):
            # Index new documents
            await repository.index_documents(new_request)

        # Verify snippet_ids were replaced, not extended
        with snippet_ids_file.open() as f:
            final_snippet_ids = json.load(f)

        # Since BM25 index is rebuilt from scratch with only new documents,
        # snippet_ids should be replaced to match the index
        assert final_snippet_ids == ["3", "4"], (
            f'Expected snippet_ids to be replaced with ["3", "4"], '
            f"but got {final_snippet_ids}. The snippet_ids should match "
            f"the documents in the rebuilt BM25 index."
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
            Document(snippet_id="1", text="first document content"),
            Document(snippet_id="2", text="second document content"),
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
        assert saved_snippet_ids == ["1", "2"]

        # Now add new documents - this should EXTEND the existing snippet_ids
        new_documents = [
            Document(snippet_id="3", text="third document content"),
            Document(snippet_id="4", text="fourth document content"),
        ]
        new_request = IndexRequest(documents=new_documents)

        # Mock the retriever again for the second indexing
        mock_retriever.scores = {"num_docs": 4}  # Updated count

        with patch.object(repository, "_retriever", return_value=mock_retriever):
            # Index new documents
            await repository.index_documents(new_request)

        # Verify snippet_ids were replaced with new documents
        with snippet_ids_file.open() as f:
            final_snippet_ids = json.load(f)

        assert final_snippet_ids == ["3", "4"]

    @pytest.mark.asyncio
    async def test_search_handles_actual_snippet_ids_not_indices(
        self,
        repository: LocalBM25Repository,
        temp_dir: Path,
    ) -> None:
        """Test that search correctly handles BM25 returning actual snippet IDs."""
        # Setup: Index documents with non-sequential snippet IDs
        documents = [
            Document(snippet_id="100", text="python programming language"),
            Document(snippet_id="200", text="javascript web development"),
            Document(snippet_id="300", text="java enterprise applications"),
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

    @pytest.mark.asyncio
    async def test_search_maintains_ordering_with_different_top_k_values(
        self,
        repository: LocalBM25Repository,
        temp_dir: Path,
    ) -> None:
        """Test search results maintain consistent ordering with different top_k."""
        # Setup: Index documents
        documents = [
            Document(snippet_id="100", text="python programming tutorial advanced"),
            Document(snippet_id="200", text="python tutorial for beginners"),
            Document(snippet_id="300", text="advanced python programming guide"),
            Document(snippet_id="400", text="basic python introduction"),
        ]
        request = IndexRequest(documents=documents)

        # Mock the BM25 retriever
        mock_retriever = MagicMock()
        mock_retriever.index.return_value = None
        mock_retriever.save.return_value = None
        mock_retriever.scores = {"num_docs": 4}

        # Create the directory structure
        (temp_dir / "bm25s_index").mkdir(parents=True, exist_ok=True)

        with patch.object(repository, "_retriever", return_value=mock_retriever):
            await repository.index_documents(request)

        # Mock retrieve to return results in descending score order
        # When top_k=1, return the best match
        def mock_retrieve_top_1(
            query_tokens: list[list[str]],  # noqa: ARG001
            corpus: list[int],  # noqa: ARG001
            k: int,
        ) -> tuple[list[list[int]], list[list[float]]]:
            assert k == 1
            return [[300]], [[0.95]]  # Best match

        # When top_k=2, return top 2 matches in descending order
        def mock_retrieve_top_2(
            query_tokens: list[list[str]],  # noqa: ARG001
            corpus: list[int],  # noqa: ARG001
            k: int,
        ) -> tuple[list[list[int]], list[list[float]]]:
            assert k == 2
            return [[300, 100]], [[0.95, 0.85]]  # Best two matches

        # When top_k=3, return top 3 matches in descending order
        def mock_retrieve_top_3(
            query_tokens: list[list[str]],  # noqa: ARG001
            corpus: list[int],  # noqa: ARG001
            k: int,
        ) -> tuple[list[list[int]], list[list[float]]]:
            assert k == 3
            return [[300, 100, 200]], [[0.95, 0.85, 0.75]]  # Best three matches

        # Test with top_k=1
        mock_retriever.retrieve = mock_retrieve_top_1
        with patch.object(repository, "_retriever", return_value=mock_retriever):
            results_k1 = await repository.search(SearchRequest(query="python", top_k=1))

        # Test with top_k=2
        mock_retriever.retrieve = mock_retrieve_top_2
        with patch.object(repository, "_retriever", return_value=mock_retriever):
            results_k2 = await repository.search(SearchRequest(query="python", top_k=2))

        # Test with top_k=3
        mock_retriever.retrieve = mock_retrieve_top_3
        with patch.object(repository, "_retriever", return_value=mock_retriever):
            results_k3 = await repository.search(SearchRequest(query="python", top_k=3))

        # Verify the top result is consistent across all top_k values
        assert len(results_k1) == 1
        assert len(results_k2) == 2
        assert len(results_k3) == 3

        # The #1 result should always be snippet 300 with score 0.95
        assert results_k1[0].snippet_id == 300, "Top result with k=1 should be 300"
        assert results_k2[0].snippet_id == 300, "Top result with k=2 should be 300"
        assert results_k3[0].snippet_id == 300, "Top result with k=3 should be 300"

        assert results_k1[0].score == 0.95, "Top score should be 0.95"
        assert results_k2[0].score == 0.95, "Top score should be 0.95"
        assert results_k3[0].score == 0.95, "Top score should be 0.95"

        # The #2 result (when k>=2) should always be snippet 100 with score 0.85
        assert results_k2[1].snippet_id == 100, "Second result should be snippet 100"
        assert results_k3[1].snippet_id == 100, "Second result should be snippet 100"
        assert results_k2[1].score == 0.85, "Second score should be 0.85"
        assert results_k3[1].score == 0.85, "Second score should be 0.85"

        # The #3 result (when k>=3) should be snippet 200 with score 0.75
        assert results_k3[2].snippet_id == 200, "Third result should be snippet 200"
        assert results_k3[2].score == 0.75, "Third score should be 0.75"
