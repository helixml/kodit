"""Domain service for indexing operations."""

from abc import ABC, abstractmethod

from kodit.domain.entities import Snippet
from kodit.domain.value_objects import (
    FusionRequest,
    FusionResult,
    IndexCreateRequest,
    IndexView,
    SnippetWithFile,
)


class IndexRepository(ABC):
    """Abstract index repository interface."""

    @abstractmethod
    async def create_index(self, source_id: int) -> IndexView:
        """Create a new index for a source."""

    @abstractmethod
    async def get_index_by_id(self, index_id: int) -> IndexView | None:
        """Get an index by its ID."""

    @abstractmethod
    async def get_index_by_source_id(self, source_id: int) -> IndexView | None:
        """Get an index by its source ID."""

    @abstractmethod
    async def list_indexes(self) -> list[IndexView]:
        """List all indexes."""

    @abstractmethod
    async def update_index_timestamp(self, index_id: int) -> None:
        """Update the timestamp of an index."""

    @abstractmethod
    async def delete_all_snippets(self, index_id: int) -> None:
        """Delete all snippets for an index."""

    @abstractmethod
    async def get_snippets_for_index(self, index_id: int) -> list[Snippet]:
        """Get all snippets for an index."""

    @abstractmethod
    async def add_snippet(self, snippet: dict) -> None:
        """Add a snippet to the database."""

    @abstractmethod
    async def update_snippet_content(self, snippet_id: int, content: str) -> None:
        """Update the content of an existing snippet."""

    @abstractmethod
    async def list_snippets_by_ids(self, ids: list[int]) -> list[SnippetWithFile]:
        """List snippets by IDs."""


class FusionService(ABC):
    """Abstract fusion service interface."""

    @abstractmethod
    def reciprocal_rank_fusion(
        self, rankings: list[list[FusionRequest]], k: float = 60
    ) -> list[FusionResult]:
        """Perform reciprocal rank fusion on search results."""


class IndexingDomainService:
    """Domain service for indexing operations."""

    def __init__(
        self, index_repository: IndexRepository, fusion_service: FusionService
    ) -> None:
        """Initialize the indexing domain service.

        Args:
            index_repository: Repository for index operations
            fusion_service: Service for result fusion

        """
        self.index_repository = index_repository
        self.fusion_service = fusion_service

    async def create_index(self, request: IndexCreateRequest) -> IndexView:
        """Create a new index.

        Args:
            request: The index create request.

        Returns:
            The created index view.

        """
        return await self.index_repository.create_index(request.source_id)

    async def get_index(self, index_id: int) -> IndexView | None:
        """Get an index by its ID.

        Args:
            index_id: The ID of the index to retrieve.

        Returns:
            The index view if found, None otherwise.

        """
        return await self.index_repository.get_index_by_id(index_id)

    async def get_index_by_source_id(self, source_id: int) -> IndexView | None:
        """Get an index by its source ID.

        Args:
            source_id: The ID of the source to retrieve an index for.

        Returns:
            The index view if found, None otherwise.

        """
        return await self.index_repository.get_index_by_source_id(source_id)

    async def list_indexes(self) -> list[IndexView]:
        """List all indexes.

        Returns:
            A list of index views.

        """
        return await self.index_repository.list_indexes()

    async def update_index_timestamp(self, index_id: int) -> None:
        """Update the timestamp of an index.

        Args:
            index_id: The ID of the index to update.

        """
        await self.index_repository.update_index_timestamp(index_id)

    async def delete_all_snippets(self, index_id: int) -> None:
        """Delete all snippets for an index.

        Args:
            index_id: The ID of the index to delete snippets for.

        """
        await self.index_repository.delete_all_snippets(index_id)

    async def get_snippets_for_index(self, index_id: int) -> list[Snippet]:
        """Get all snippets for an index.

        Args:
            index_id: The ID of the index to get snippets for.

        Returns:
            A list of Snippet entities.

        """
        return await self.index_repository.get_snippets_for_index(index_id)

    async def add_snippet(self, snippet: dict) -> None:
        """Add a snippet to the database.

        Args:
            snippet: The snippet to add.

        """
        await self.index_repository.add_snippet(snippet)

    async def update_snippet_content(self, snippet_id: int, content: str) -> None:
        """Update the content of an existing snippet.

        Args:
            snippet_id: The ID of the snippet to update.
            content: The new content for the snippet.

        """
        await self.index_repository.update_snippet_content(snippet_id, content)

    def perform_fusion(
        self, rankings: list[list[FusionRequest]], k: float = 60
    ) -> list[FusionResult]:
        """Perform fusion on search results.

        Args:
            rankings: List of rankings to fuse.
            k: Parameter for reciprocal rank fusion.

        Returns:
            Fused search results.

        """
        return self.fusion_service.reciprocal_rank_fusion(rankings, k)

    async def get_snippets_by_ids(self, ids: list[int]) -> list[SnippetWithFile]:
        """Get snippets by IDs.

        Args:
            ids: List of snippet IDs to retrieve.

        Returns:
            List of SnippetWithFile objects containing file and snippet information.

        """
        return await self.index_repository.list_snippets_by_ids(ids)
