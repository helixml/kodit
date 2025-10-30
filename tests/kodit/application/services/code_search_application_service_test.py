"""Tests for CodeSearchApplicationService with real database."""

# cspell:ignore assoc

from collections.abc import AsyncGenerator
from typing import Any
from unittest.mock import MagicMock

import pytest

from kodit.application.services.code_search_application_service import (
    CodeSearchApplicationService,
)
from kodit.application.services.enrichment_query_service import EnrichmentQueryService
from kodit.domain.enrichments.development.development import (
    ENRICHMENT_TYPE_DEVELOPMENT,
)
from kodit.domain.enrichments.development.snippet.snippet import (
    ENRICHMENT_SUBTYPE_SNIPPET,
    ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY,
)
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import (
    EmbeddingDomainService,
)
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.domain.value_objects import (
    IndexRequest,
    IndexResult,
    MultiSearchRequest,
    SearchRequest,
    SearchResult,
)
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.enrichment_association_repository import (
    SQLAlchemyEnrichmentAssociationRepository,
)
from kodit.infrastructure.sqlalchemy.enrichment_v2_repository import (
    SQLAlchemyEnrichmentV2Repository,
)
from kodit.infrastructure.sqlalchemy.entities import Embedding, EmbeddingType


class MockEmbeddingProvider:
    """Mock embedding provider that returns fixed embeddings."""

    async def embed(self, data: Any) -> AsyncGenerator[list[Any], None]:
        """Generate simple embeddings based on text content."""
        results = []
        for item in data:
            text = item.text.lower()
            # Create a simple 3-dimensional embedding based on text characteristics
            emb = [
                float(len(text)) / 100.0,  # Length feature
                float(text.count("function")) / 10.0,  # Function keyword feature
                float(text.count("class")) / 10.0,  # Class keyword feature
            ]
            results.append(MagicMock(embedding=emb))
        yield results


class MockVectorSearchRepository:
    """Mock vector search repository for testing."""

    def __init__(self, embedding_repo: SqlAlchemyEmbeddingRepository) -> None:
        """Initialize with real embedding repository."""
        self.embedding_repo = embedding_repo

    async def index_documents(
        self, request: IndexRequest
    ) -> AsyncGenerator[list[IndexResult], None]:
        """Index documents by creating embeddings."""
        for doc in request.documents:
            embedding = Embedding()
            embedding.snippet_id = doc.snippet_id
            embedding.type = EmbeddingType.TEXT
            # Generate embedding based on text content
            text = doc.text.lower()
            embedding.embedding = [
                float(len(text)) / 100.0,
                float(text.count("function")) / 10.0,
                float(text.count("class")) / 10.0,
            ]
            await self.embedding_repo.create_embedding(embedding)
        yield []

    async def search(self, request: SearchRequest) -> list[SearchResult]:
        """Search using real embedding repository."""
        # Generate query embedding
        text = request.query.lower()
        query_embedding = [
            float(len(text)) / 100.0,
            float(text.count("function")) / 10.0,
            float(text.count("class")) / 10.0,
        ]

        # Use real embedding repository to search
        results = await self.embedding_repo.list_semantic_results(
            EmbeddingType.TEXT,
            query_embedding,
            top_k=request.top_k,
            snippet_ids=request.snippet_ids,
        )

        return [SearchResult(snippet_id=sid, score=score) for sid, score in results]

    async def has_embedding(
        self, snippet_id: int, embedding_type: EmbeddingType
    ) -> bool:
        """Check if embedding exists."""
        result = await self.embedding_repo.get_embedding_by_snippet_id_and_type(
            snippet_id, embedding_type
        )
        return result is not None


class MockBM25Repository:
    """Mock BM25 repository for testing."""

    async def search(self, _request: SearchRequest) -> list[SearchResult]:
        """Return empty results for BM25 search."""
        return []


@pytest.fixture
async def enrichment_v2_repo(
    session_factory: Any,
) -> SQLAlchemyEnrichmentV2Repository:
    """Create enrichment v2 repository."""
    return SQLAlchemyEnrichmentV2Repository(session_factory=session_factory)


@pytest.fixture
async def enrichment_association_repo(
    session_factory: Any,
) -> SQLAlchemyEnrichmentAssociationRepository:
    """Create enrichment association repository."""
    return SQLAlchemyEnrichmentAssociationRepository(session_factory=session_factory)


@pytest.fixture
async def embedding_repo(session_factory: Any) -> SqlAlchemyEmbeddingRepository:
    """Create embedding repository."""
    return SqlAlchemyEmbeddingRepository(session_factory=session_factory)


@pytest.fixture
async def code_search_service(
    embedding_repo: SqlAlchemyEmbeddingRepository,
) -> EmbeddingDomainService:
    """Create code search service with real database."""
    # Create real services
    vector_search_repo = MockVectorSearchRepository(embedding_repo)
    embedding_provider = MockEmbeddingProvider()

    return EmbeddingDomainService(
        embedding_provider=embedding_provider,  # type: ignore[arg-type]
        vector_search_repository=vector_search_repo,  # type: ignore[arg-type]
    )


@pytest.fixture
async def test_data(
    enrichment_v2_repo: SQLAlchemyEnrichmentV2Repository,
    enrichment_association_repo: SQLAlchemyEnrichmentAssociationRepository,
    embedding_repo: SqlAlchemyEmbeddingRepository,
) -> dict[str, Any]:
    """Create test data: snippets with summaries and embeddings."""
    # Create snippet enrichments
    snippet1 = db_entities.EnrichmentV2(
        type=ENRICHMENT_TYPE_DEVELOPMENT,
        subtype=ENRICHMENT_SUBTYPE_SNIPPET,
        content="def calculate_sum(a, b):\n    return a + b",
    )
    snippet2 = db_entities.EnrichmentV2(
        type=ENRICHMENT_TYPE_DEVELOPMENT,
        subtype=ENRICHMENT_SUBTYPE_SNIPPET,
        content="class Calculator:\n    def add(self, x, y):\n        return x + y",
    )
    snippet3 = db_entities.EnrichmentV2(
        type=ENRICHMENT_TYPE_DEVELOPMENT,
        subtype=ENRICHMENT_SUBTYPE_SNIPPET,
        content="def process_data(data):\n    return data.strip()",
    )

    # Save snippets and capture returned entities with IDs
    snippet1 = await enrichment_v2_repo.save(snippet1)  # type: ignore[arg-type,assignment]
    snippet2 = await enrichment_v2_repo.save(snippet2)  # type: ignore[arg-type,assignment]
    snippet3 = await enrichment_v2_repo.save(snippet3)  # type: ignore[arg-type,assignment]

    # Create summary enrichments
    summary1 = db_entities.EnrichmentV2(
        type=ENRICHMENT_TYPE_DEVELOPMENT,
        subtype=ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY,
        content="This function calculates the sum of two numbers",
    )
    summary2 = db_entities.EnrichmentV2(
        type=ENRICHMENT_TYPE_DEVELOPMENT,
        subtype=ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY,
        content="This class provides calculator functionality with addition method",
    )
    summary3 = db_entities.EnrichmentV2(
        type=ENRICHMENT_TYPE_DEVELOPMENT,
        subtype=ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY,
        content="This function processes data by removing whitespace",
    )

    # Save summaries and capture returned entities with IDs
    summary1 = await enrichment_v2_repo.save(summary1)  # type: ignore[arg-type,assignment]
    summary2 = await enrichment_v2_repo.save(summary2)  # type: ignore[arg-type,assignment]
    summary3 = await enrichment_v2_repo.save(summary3)  # type: ignore[arg-type,assignment]

    # Create associations between summaries and snippets
    assoc1 = db_entities.EnrichmentAssociation(
        enrichment_id=summary1.id,
        entity_type=db_entities.EnrichmentV2.__tablename__,
        entity_id=str(snippet1.id),
    )
    assoc2 = db_entities.EnrichmentAssociation(
        enrichment_id=summary2.id,
        entity_type=db_entities.EnrichmentV2.__tablename__,
        entity_id=str(snippet2.id),
    )
    assoc3 = db_entities.EnrichmentAssociation(
        enrichment_id=summary3.id,
        entity_type=db_entities.EnrichmentV2.__tablename__,
        entity_id=str(snippet3.id),
    )

    assoc1 = await enrichment_association_repo.save(assoc1)  # type: ignore[arg-type,assignment]
    assoc2 = await enrichment_association_repo.save(assoc2)  # type: ignore[arg-type,assignment]
    assoc3 = await enrichment_association_repo.save(assoc3)  # type: ignore[arg-type,assignment]

    # Add a commit-pointing association to summary1 to test entity_type filtering
    # This association points to a commit SHA, which cannot be converted to int
    commit_assoc = db_entities.EnrichmentAssociation(
        enrichment_id=summary1.id,
        entity_type=db_entities.GitCommit.__tablename__,
        entity_id="abc123def456",  # Commit SHA - would fail int() conversion
    )
    await enrichment_association_repo.save(commit_assoc)  # type: ignore[arg-type,assignment]

    # Create embeddings for summaries
    emb1 = Embedding()
    emb1.snippet_id = str(summary1.id)
    emb1.type = EmbeddingType.TEXT
    # Embedding that matches "function" query well
    emb1.embedding = [0.5, 1.0, 0.0]
    await embedding_repo.create_embedding(emb1)

    emb2 = Embedding()
    emb2.snippet_id = str(summary2.id)
    emb2.type = EmbeddingType.TEXT
    # Embedding that matches "class" query well
    emb2.embedding = [0.6, 0.5, 1.0]
    await embedding_repo.create_embedding(emb2)

    emb3 = Embedding()
    emb3.snippet_id = str(summary3.id)
    emb3.type = EmbeddingType.TEXT
    # Embedding that matches "function" query moderately
    emb3.embedding = [0.4, 0.7, 0.0]
    await embedding_repo.create_embedding(emb3)

    return {
        "snippets": [snippet1, snippet2, snippet3],
        "summaries": [summary1, summary2, summary3],
        "associations": [assoc1, assoc2, assoc3],
    }


class TestCodeSearchApplicationServiceTextQuery:
    """Test the text_query parameter in CodeSearchApplicationService.search."""

    @pytest.mark.asyncio
    async def test_search_with_text_query_finds_matching_snippets(
        self,
        code_search_service: EmbeddingDomainService,
        enrichment_v2_repo: SQLAlchemyEnrichmentV2Repository,
        enrichment_association_repo: SQLAlchemyEnrichmentAssociationRepository,
        test_data: dict[str, Any],  # noqa: ARG002
    ) -> None:
        """Test that text_query searches summaries and returns associated snippets."""
        # Create the full service
        mock_bm25_service = BM25DomainService(
            repository=MockBM25Repository()  # type: ignore[arg-type]
        )
        mock_progress_tracker = MagicMock()
        fusion_service = ReciprocalRankFusionService()

        trackable_resolution = MagicMock(spec=TrackableResolutionService)
        enrichment_query_service = EnrichmentQueryService(
            trackable_resolution=trackable_resolution,
            enrichment_repo=enrichment_v2_repo,
            enrichment_association_repository=enrichment_association_repo,
        )

        service = CodeSearchApplicationService(
            bm25_service=mock_bm25_service,
            code_search_service=code_search_service,
            text_search_service=code_search_service,
            progress_tracker=mock_progress_tracker,
            fusion_service=fusion_service,
            enrichment_query_service=enrichment_query_service,
        )

        # Search for "function" - should match snippets 1 and 3
        request = MultiSearchRequest(
            text_query="function calculations",
            top_k=10,
        )

        results = await service.search(request)

        # Verify we got results
        assert len(results) > 0, "Should return at least one result"

        # Verify results contain snippets (not summaries)
        for result in results:
            assert result.snippet.content is not None
            # Verify the content is from a snippet, not a summary
            assert (
                "def " in result.snippet.content or "class " in result.snippet.content
            ), "Results should contain actual code snippets"

    @pytest.mark.asyncio
    async def test_search_with_text_query_returns_correct_snippet_for_class_query(
        self,
        code_search_service: EmbeddingDomainService,
        enrichment_v2_repo: SQLAlchemyEnrichmentV2Repository,
        enrichment_association_repo: SQLAlchemyEnrichmentAssociationRepository,
        test_data: dict[str, Any],  # noqa: ARG002
    ) -> None:
        """Test that searching for 'class' returns the Calculator snippet."""
        # Create the full service
        mock_bm25_service = BM25DomainService(
            repository=MockBM25Repository()  # type: ignore[arg-type]
        )
        mock_progress_tracker = MagicMock()
        fusion_service = ReciprocalRankFusionService()

        trackable_resolution = MagicMock(spec=TrackableResolutionService)
        enrichment_query_service = EnrichmentQueryService(
            trackable_resolution=trackable_resolution,
            enrichment_repo=enrichment_v2_repo,
            enrichment_association_repository=enrichment_association_repo,
        )

        service = CodeSearchApplicationService(
            bm25_service=mock_bm25_service,
            code_search_service=code_search_service,
            text_search_service=code_search_service,
            progress_tracker=mock_progress_tracker,
            fusion_service=fusion_service,
            enrichment_query_service=enrichment_query_service,
        )

        # Search for "class" - should match snippet 2 best
        request = MultiSearchRequest(
            text_query="class calculator functionality",
            top_k=10,
        )

        results = await service.search(request)

        # Verify we got results
        assert len(results) > 0, "Should return at least one result"

        # The top result should contain the Calculator class
        top_result = results[0]
        assert "class Calculator" in top_result.snippet.content

    @pytest.mark.asyncio
    async def test_search_with_empty_text_query_returns_empty(
        self,
        code_search_service: EmbeddingDomainService,
        enrichment_v2_repo: SQLAlchemyEnrichmentV2Repository,
        enrichment_association_repo: SQLAlchemyEnrichmentAssociationRepository,
        test_data: dict[str, Any],  # noqa: ARG002
    ) -> None:
        """Test that empty text_query returns no results."""
        mock_bm25_service = BM25DomainService(
            repository=MockBM25Repository()  # type: ignore[arg-type]
        )
        mock_progress_tracker = MagicMock()
        fusion_service = ReciprocalRankFusionService()

        trackable_resolution = MagicMock(spec=TrackableResolutionService)
        enrichment_query_service = EnrichmentQueryService(
            trackable_resolution=trackable_resolution,
            enrichment_repo=enrichment_v2_repo,
            enrichment_association_repository=enrichment_association_repo,
        )

        service = CodeSearchApplicationService(
            bm25_service=mock_bm25_service,
            code_search_service=code_search_service,
            text_search_service=code_search_service,
            progress_tracker=mock_progress_tracker,
            fusion_service=fusion_service,
            enrichment_query_service=enrichment_query_service,
        )

        # Search with no query parameters
        request = MultiSearchRequest(
            text_query=None,
            top_k=10,
        )

        results = await service.search(request)

        # Should return empty results
        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_search_text_query_respects_top_k(
        self,
        code_search_service: EmbeddingDomainService,
        enrichment_v2_repo: SQLAlchemyEnrichmentV2Repository,
        enrichment_association_repo: SQLAlchemyEnrichmentAssociationRepository,
        test_data: dict[str, Any],  # noqa: ARG002
    ) -> None:
        """Test that top_k limits the number of results."""
        mock_bm25_service = BM25DomainService(
            repository=MockBM25Repository()  # type: ignore[arg-type]
        )
        mock_progress_tracker = MagicMock()
        fusion_service = ReciprocalRankFusionService()

        trackable_resolution = MagicMock(spec=TrackableResolutionService)
        enrichment_query_service = EnrichmentQueryService(
            trackable_resolution=trackable_resolution,
            enrichment_repo=enrichment_v2_repo,
            enrichment_association_repository=enrichment_association_repo,
        )

        service = CodeSearchApplicationService(
            bm25_service=mock_bm25_service,
            code_search_service=code_search_service,
            text_search_service=code_search_service,
            progress_tracker=mock_progress_tracker,
            fusion_service=fusion_service,
            enrichment_query_service=enrichment_query_service,
        )

        # Search with top_k=1
        request = MultiSearchRequest(
            text_query="function or class",
            top_k=1,
        )

        results = await service.search(request)

        # Should return at most 1 result
        assert len(results) <= 1
