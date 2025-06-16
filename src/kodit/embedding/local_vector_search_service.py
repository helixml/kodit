"""Local vector search."""

import structlog
import tiktoken

from kodit.embedding.embedding_models import Embedding, EmbeddingType
from kodit.embedding.embedding_provider.embedding_provider import EmbeddingProvider
from kodit.embedding.embedding_repository import EmbeddingRepository
from kodit.embedding.vector_search_service import (
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

    async def index(self, data: list[VectorSearchRequest]) -> None:
        """Embed a list of documents."""
        if not data or len(data) == 0:
            self.log.warning("Embedding data is empty, skipping embedding")
            return

        # Prepare requests for the embedding provider.
        from kodit.embedding.embedding_provider.embedding_provider import (
            EmbeddingRequest,
        )

        requests = [
            EmbeddingRequest(id=idx, text=item.text) for idx, item in enumerate(data)
        ]

        # Collect embeddings from the async generator while preserving order.
        embeddings_map: dict[int, list[float]] = {}
        async for batch in self.embedding_provider.embed(requests):
            for resp in batch:
                embeddings_map[resp.id] = [float(v) for v in resp.embedding]

        # Persist embeddings following the original data order.
        for idx, item in enumerate(data):
            embedding_vec = embeddings_map.get(idx)
            if embedding_vec is None:
                # Skip if the provider returned no embedding (e.g., empty text)
                continue
            await self.embedding_repository.create_embedding(
                Embedding(
                    snippet_id=item.snippet_id,
                    embedding=embedding_vec,
                    type=EmbeddingType.CODE,
                )
            )

    async def retrieve(self, query: str, top_k: int = 10) -> list[VectorSearchResponse]:
        """Query the embedding model."""
        from kodit.embedding.embedding_provider.embedding_provider import (
            EmbeddingRequest,
        )

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
