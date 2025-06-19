"""Tests for domain models."""

import pytest
from datetime import datetime, UTC

from kodit.domain.entities import (
    Source,
    SourceType,
    File,
    Index,
    Snippet,
    Embedding,
    EmbeddingType,
    Author,
    AuthorFileMapping,
)
from kodit.domain.value_objects import (
    BM25Document,
    BM25SearchResult,
    VectorSearchRequest,
    VectorSearchResult,
    EnrichmentRequest,
    EnrichmentResponse,
)


class TestSource:
    """Test Source domain model."""

    def test_create_source(self):
        """Test creating a source."""
        source = Source(
            uri="test_repo", cloned_path="/tmp/test_repo", source_type=SourceType.GIT
        )

        assert source.uri == "test_repo"
        assert source.cloned_path == "/tmp/test_repo"
        assert source.type == SourceType.GIT
        # SQLAlchemy models need to be added to a session to get created_at/updated_at
        # For unit tests, we'll just check the fields are set correctly
        assert hasattr(source, "created_at")
        assert hasattr(source, "updated_at")

    def test_source_types(self):
        """Test source type enum."""
        assert SourceType.GIT.value == 2
        assert SourceType.FOLDER.value == 1


class TestFile:
    """Test File domain model."""

    def test_create_file(self):
        """Test creating a file."""
        now = datetime.now(UTC)
        file = File(
            created_at=now,
            updated_at=now,
            source_id=1,
            cloned_path="/tmp/test.txt",
            mime_type="text/plain",
            uri="file:///test.txt",
            sha256="abc123",
            size_bytes=100,
        )

        assert file.source_id == 1
        assert file.mime_type == "text/plain"
        assert file.uri == "file:///test.txt"
        assert file.cloned_path == "/tmp/test.txt"
        assert file.sha256 == "abc123"
        assert file.size_bytes == 100


class TestSnippet:
    """Test Snippet domain model."""

    def test_create_snippet(self):
        """Test creating a snippet."""
        snippet = Snippet(file_id=1, index_id=1, content="def hello(): pass")

        assert snippet.file_id == 1
        assert snippet.index_id == 1
        assert snippet.content == "def hello(): pass"


class TestEmbedding:
    """Test Embedding domain model."""

    def test_create_embedding(self):
        """Test creating an embedding."""
        embedding = Embedding()
        embedding.snippet_id = 1
        embedding.type = EmbeddingType.CODE
        embedding.embedding = [0.1, 0.2, 0.3]

        assert embedding.snippet_id == 1
        assert embedding.type == EmbeddingType.CODE
        assert embedding.embedding == [0.1, 0.2, 0.3]


class TestAuthor:
    """Test Author domain model."""

    def test_create_author(self):
        """Test creating an author."""
        author = Author()
        author.name = "John Doe"
        author.email = "john@example.com"

        assert author.name == "John Doe"
        assert author.email == "john@example.com"


class TestBM25Models:
    """Test BM25 domain models."""

    def test_bm25_document(self):
        """Test BM25Document."""
        doc = BM25Document(snippet_id=1, text="test content")
        assert doc.snippet_id == 1
        assert doc.text == "test content"

    def test_bm25_search_result(self):
        """Test BM25SearchResult."""
        result = BM25SearchResult(snippet_id=1, score=0.85)
        assert result.snippet_id == 1
        assert result.score == 0.85


class TestVectorModels:
    """Test vector search domain models."""

    def test_vector_search_request(self):
        """Test VectorSearchRequest."""
        request = VectorSearchRequest(snippet_id=1, text="test content")
        assert request.snippet_id == 1
        assert request.text == "test content"

    def test_vector_search_result(self):
        """Test VectorSearchResult."""
        result = VectorSearchResult(snippet_id=1, score=0.92)
        assert result.snippet_id == 1
        assert result.score == 0.92


class TestEnrichmentModels:
    """Test enrichment domain models."""

    def test_enrichment_request(self):
        """Test EnrichmentRequest."""
        request = EnrichmentRequest(snippet_id=1, text="test content")
        assert request.snippet_id == 1
        assert request.text == "test content"

    def test_enrichment_response(self):
        """Test EnrichmentResponse."""
        response = EnrichmentResponse(snippet_id=1, text="enriched content")
        assert response.snippet_id == 1
        assert response.text == "enriched content"
