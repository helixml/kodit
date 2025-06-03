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
        embeddings = await self.embedding_provider.embed([i.text for i in data])
        for i, x in zip(data, embeddings, strict=False):
            await self.embedding_repository.create_embedding(
                Embedding(
                    snippet_id=i.snippet_id,
                    embedding=[float(y) for y in x],
                    type=EmbeddingType.CODE,
                )
            )

    async def retrieve(self, query: str, top_k: int = 10) -> list[VectorSearchResponse]:
        """Query the embedding model."""
        embedding = (await self.embedding_provider.embed([query]))[0]
        results = await self.embedding_repository.list_semantic_results(
            EmbeddingType.CODE, [float(x) for x in embedding], top_k
        )
        return [
            VectorSearchResponse(snippet_id, score) for snippet_id, score in results
        ]
