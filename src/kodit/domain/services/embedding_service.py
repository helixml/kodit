"""Domain services for embedding operations."""

from abc import ABC, abstractmethod
from collections.abc import AsyncGenerator, Sequence

from kodit.domain.value_objects import (
    EmbeddingRequest,
    EmbeddingResponse,
    IndexRequest,
    IndexResult,
    SearchRequest,
    SearchResult,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType


class EmbeddingProvider(ABC):
    """Abstract interface for embedding provider."""

    @abstractmethod
    def embed(
        self, data: list[EmbeddingRequest]
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed a list of strings."""


class VectorSearchRepository(ABC):
    """Abstract interface for vector search repository."""

    @abstractmethod
    def index_documents(
        self, request: IndexRequest
    ) -> AsyncGenerator[list[IndexResult], None]:
        """Index documents for vector search."""

    @abstractmethod
    async def search(self, request: SearchRequest) -> Sequence[SearchResult]:
        """Search documents using vector similarity."""

    @abstractmethod
    async def has_embedding(
        self, snippet_id: int, embedding_type: EmbeddingType
    ) -> bool:
        """Check if a snippet has an embedding."""


class EmbeddingDomainService:
    """Domain service for embedding operations."""

    def __init__(
        self,
        embedding_provider: EmbeddingProvider,
        vector_search_repository: VectorSearchRepository,
    ) -> None:
        """Initialize the embedding domain service.

        Args:
            embedding_provider: The embedding provider for generating embeddings
            vector_search_repository: The vector search repository for persistence

        """
        self.embedding_provider = embedding_provider
        self.vector_search_repository = vector_search_repository

    async def index_documents(
        self, request: IndexRequest
    ) -> AsyncGenerator[list[IndexResult], None]:
        """Index documents using domain business rules.

        Args:
            request: The indexing request containing documents to index

        Yields:
            Lists of IndexResult for each batch processed

        Raises:
            ValueError: If the request is invalid

        """
        # Domain logic: validate request
        if not request.documents:
            return

        # Domain logic: filter out invalid documents
        valid_documents = [
            doc
            for doc in request.documents
            if doc.snippet_id is not None and doc.text and doc.text.strip()
        ]

        if not valid_documents:
            return

        # Domain logic: create new request with validated documents
        validated_request = IndexRequest(documents=valid_documents)
        async for result in self.vector_search_repository.index_documents(
            validated_request
        ):
            yield result

    async def search(self, request: SearchRequest) -> Sequence[SearchResult]:
        """Search documents using domain business rules.

        Args:
            request: The search request

        Returns:
            Sequence of search results

        Raises:
            ValueError: If the request is invalid

        """
        # Domain logic: validate request
        if not request.query or not request.query.strip():
            raise ValueError("Search query cannot be empty")

        if request.top_k <= 0:
            raise ValueError("Top-k must be positive")

        # Domain logic: normalize query
        normalized_query = request.query.strip()
        normalized_request = SearchRequest(
            query=normalized_query, top_k=request.top_k, snippet_ids=request.snippet_ids
        )

        return await self.vector_search_repository.search(normalized_request)

    async def has_embedding(
        self, snippet_id: int, embedding_type: EmbeddingType
    ) -> bool:
        """Check if a snippet has an embedding using domain business rules.

        Args:
            snippet_id: The snippet ID to check
            embedding_type: The type of embedding to check

        Returns:
            True if the snippet has an embedding, False otherwise

        Raises:
            ValueError: If the snippet_id is invalid

        """
        # Domain logic: validate snippet_id
        if snippet_id is None or snippet_id <= 0:
            raise ValueError("Snippet ID must be positive")

        return await self.vector_search_repository.has_embedding(
            snippet_id, embedding_type
        )
