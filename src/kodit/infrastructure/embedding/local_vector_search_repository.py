"""Local vector search repository implementation."""

from collections.abc import AsyncGenerator

import structlog

from kodit.domain.services.embedding_service import (
    EmbeddingProvider,
    VectorSearchRepository,
)
from kodit.domain.value_objects import (
    EmbeddingRequest,
    IndexRequest,
    IndexResult,
    SearchRequest,
    SearchResult,
)
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import Embedding, EmbeddingType


class LocalVectorSearchRepository(VectorSearchRepository):
    """Local vector search repository implementation."""

    def __init__(
        self,
        embedding_provider: EmbeddingProvider,
        embedding_repository: SqlAlchemyEmbeddingRepository,
        embedding_type: EmbeddingType,
    ) -> None:
        """Initialize the local vector search repository.

        Args:
            embedding_provider: The embedding provider for generating embeddings
            embedding_repository: The embedding repository for persistence
            embedding_type: The type of embedding to use

        """
        self.embedding_provider = embedding_provider
        self.embedding_repository = embedding_repository
        self.embedding_type = embedding_type
        self.log = structlog.get_logger(__name__)

    async def index_documents(
        self, request: IndexRequest
    ) -> AsyncGenerator[list[IndexResult], None]:
        """Index documents for vector search."""
        if not request.documents or len(request.documents) == 0:
            yield []

        # Convert to embedding requests
        requests = [
            EmbeddingRequest(snippet_id=doc.snippet_id, text=doc.text)
            for doc in request.documents
        ]

        async for batch in self.embedding_provider.embed(requests):
            results = []
            for result in batch:
                await self.embedding_repository.create_embedding(
                    Embedding(
                        snippet_id=result.snippet_id,
                        embedding=result.embedding,
                        type=self.embedding_type,
                    )
                )
                results.append(IndexResult(snippet_id=result.snippet_id))
            yield results

    async def search(self, request: SearchRequest) -> list[SearchResult]:
        """Search documents using vector similarity."""
        # Build a single-item request and collect its embedding
        req = EmbeddingRequest(snippet_id=0, text=request.query)
        embedding_vec: list[float] | None = None
        async for batch in self.embedding_provider.embed([req]):
            if batch:
                embedding_vec = [float(v) for v in batch[0].embedding]
                break

        if not embedding_vec:
            return []

        results = await self.embedding_repository.list_semantic_results(
            self.embedding_type, embedding_vec, request.top_k, request.snippet_ids
        )
        return [
            SearchResult(snippet_id=snippet_id, score=score)
            for snippet_id, score in results
        ]

    async def has_embedding(
        self, snippet_id: int, embedding_type: EmbeddingType
    ) -> bool:
        """Check if a snippet has an embedding."""
        return (
            await self.embedding_repository.get_embedding_by_snippet_id_and_type(
                snippet_id, embedding_type
            )
            is not None
        )
