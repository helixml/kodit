"""Local vector search."""

from collections.abc import AsyncGenerator

import structlog
import tiktoken

from kodit.embedding.embedding_models import Embedding, EmbeddingType
from kodit.embedding.embedding_provider.embedding_provider import (
    EmbeddingProvider,
    EmbeddingRequest,
)
from kodit.embedding.embedding_repository import EmbeddingRepository
from kodit.embedding.vector_search_service import (
    IndexResult,
    VectorSearchRequest,
    VectorSearchResponse,
    VectorSearchService,
)


class LocalVectorSearchService(VectorSearchService):
    """Local vector search."""

    def __init__(
        self,
        embedding_repository: EmbeddingRepository,
        embedding_provider: EmbeddingProvider,
    ) -> None:
        """Initialize the local embedder."""
        self.log = structlog.get_logger(__name__)
        self.embedding_repository = embedding_repository
        self.embedding_provider = embedding_provider
        self.encoding = tiktoken.encoding_for_model("text-embedding-3-small")

    async def index(
        self, data: list[VectorSearchRequest]
    ) -> AsyncGenerator[list[IndexResult], None]:
        """Embed a list of documents."""
        if not data or len(data) == 0:
            self.log.warning("Embedding data is empty, skipping embedding")
            return

        requests = [EmbeddingRequest(id=doc.snippet_id, text=doc.text) for doc in data]

        async for batch in self.embedding_provider.embed(requests):
            for result in batch:
                await self.embedding_repository.create_embedding(
                    Embedding(
                        snippet_id=result.id,
                        embedding=result.embedding,
                        type=EmbeddingType.CODE,
                    )
                )
                yield [IndexResult(snippet_id=result.id)]

    async def retrieve(self, query: str, top_k: int = 10) -> list[VectorSearchResponse]:
        """Query the embedding model."""
        # Build a single-item request and collect its embedding.
        req = EmbeddingRequest(id=0, text=query)
        embedding_vec: list[float] | None = None
        async for batch in self.embedding_provider.embed([req]):
            if batch:
                embedding_vec = [float(v) for v in batch[0].embedding]
                break

        if not embedding_vec:
            return []

        results = await self.embedding_repository.list_semantic_results(
            EmbeddingType.CODE, embedding_vec, top_k
        )
        return [
            VectorSearchResponse(snippet_id, score) for snippet_id, score in results
        ]
