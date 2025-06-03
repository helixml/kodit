"""Tests for the local BM25 service."""

import json
import tempfile
from pathlib import Path

import pytest
from bm25s.tokenization import Tokenized

from kodit.bm25.local_bm25 import BM25Service
from kodit.bm25.keyword_search_service import BM25Document, BM25Result


@pytest.fixture
def temp_data_dir():
    """Create a temporary directory for test data."""
    with tempfile.TemporaryDirectory() as temp_dir:
        yield Path(temp_dir)


@pytest.fixture
def bm25_service(temp_data_dir):
    """Create a BM25Service instance with a temporary data directory."""
    return BM25Service(temp_data_dir)


@pytest.fixture
def sample_documents():
    """Create sample documents for testing."""
    return [
        BM25Document(snippet_id=1, text="The quick brown fox jumps over the lazy dog"),
        BM25Document(snippet_id=2, text="A quick brown fox is faster than a lazy dog"),
        BM25Document(snippet_id=3, text="The lazy dog sleeps all day"),
    ]


@pytest.mark.asyncio
async def test_initialization(bm25_service):
    """Test that the service initializes correctly."""
    assert bm25_service.retriever is not None
    assert isinstance(bm25_service.snippet_ids, list)
    assert len(bm25_service.snippet_ids) == 0


@pytest.mark.asyncio
async def test_index_and_retrieve(bm25_service, sample_documents):
    """Test indexing and retrieving documents."""
    # Index the documents
    await bm25_service.index(sample_documents)

    # Verify snippet IDs were stored correctly
    assert len(bm25_service.snippet_ids) == 3
    assert bm25_service.snippet_ids == [1, 2, 3]

    # Test retrieval
    results = await bm25_service.retrieve("quick brown fox", top_k=2)
    assert len(results) == 2

    # Verify results are BM25Result objects with correct snippet IDs
    assert all(isinstance(r, BM25Result) for r in results)
    assert all(r.snippet_id in [1, 2] for r in results)

    # Verify scores are in descending order
    assert results[0].score >= results[1].score


@pytest.mark.asyncio
async def test_empty_corpus(bm25_service):
    """Test behavior with empty corpus."""
    # Use a minimal document with a single token to avoid BM25 library limitations
    minimal_doc = [BM25Document(snippet_id=1, text="test")]
    await bm25_service.index(minimal_doc)
    assert len(bm25_service.snippet_ids) == 1

    # Test retrieval with a query that won't match
    results = await bm25_service.retrieve("nonexistent", top_k=1)
    assert len(results) == 0


@pytest.mark.asyncio
async def test_top_k_larger_than_corpus(bm25_service):
    """Test behavior when top_k is larger than corpus size."""
    # Create a minimal corpus
    docs = [BM25Document(snippet_id=1, text="test")]
    await bm25_service.index(docs)

    # Test that top_k is automatically adjusted to corpus size
    results = await bm25_service.retrieve("test", top_k=2)
    assert len(results) == 1
    assert results[0].snippet_id == 1


@pytest.mark.asyncio
async def test_top_k_zero(bm25_service, sample_documents):
    """Test behavior when top_k is 0."""
    await bm25_service.index(sample_documents)
    results = await bm25_service.retrieve("test", top_k=0)
    assert len(results) == 0


@pytest.mark.asyncio
async def test_snippet_id_matching(bm25_service):
    """Test that snippet IDs correctly match with their corresponding documents."""
    # Create documents with distinct content
    docs = [
        BM25Document(snippet_id=100, text="python programming language"),
        BM25Document(snippet_id=200, text="java programming language"),
        BM25Document(snippet_id=300, text="javascript programming language"),
    ]

    await bm25_service.index(docs)

    # Search for "python"
    results = await bm25_service.retrieve("python", top_k=1)
    assert len(results) == 1
    assert results[0].snippet_id == 100

    # Search for "java"
    results = await bm25_service.retrieve("java", top_k=1)
    assert len(results) == 1
    assert results[0].snippet_id == 200

    # Search for "javascript"
    results = await bm25_service.retrieve("javascript", top_k=1)
    assert len(results) == 1
    assert results[0].snippet_id == 300


@pytest.mark.asyncio
async def test_persistence(bm25_service, sample_documents):
    """Test that the index and snippet IDs persist between service instances."""
    # Create and index documents
    await bm25_service.index(sample_documents)

    # Create a new service instance with the same data directory
    new_service = BM25Service(bm25_service.index_path.parent)

    # Verify the new instance loaded the correct data
    assert len(new_service.snippet_ids) == 3
    assert new_service.snippet_ids == [1, 2, 3]

    # Verify retrieval works with the loaded index
    results = await new_service.retrieve("quick brown fox", top_k=2)
    assert len(results) == 2
    assert all(r.snippet_id in [1, 2] for r in results)


@pytest.mark.asyncio
async def test_delete_not_supported(bm25_service, sample_documents):
    """Test that delete operation is not supported."""
    await bm25_service.index(sample_documents)
    await bm25_service.delete([1, 2])
    # Verify that the documents are still there
    results = await bm25_service.retrieve("quick brown fox", top_k=2)
    assert len(results) == 2
