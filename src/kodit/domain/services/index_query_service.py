"""Index query service."""

from abc import ABC, abstractmethod

from kodit.domain.entities import Index, SnippetWithContext
from kodit.domain.protocols import IndexRepository
from kodit.domain.value_objects import (
    FusionRequest,
    FusionResult,
    MultiSearchRequest,
)


class FusionService(ABC):
    """Abstract fusion service interface."""

    @abstractmethod
    def reciprocal_rank_fusion(
        self, rankings: list[list[FusionRequest]], k: float = 60
    ) -> list[FusionResult]:
        """Perform reciprocal rank fusion on search results."""


class IndexQueryService:
    """Index query service."""

    def __init__(
        self,
        index_repository: IndexRepository,
        fusion_service: FusionService,
    ) -> None:
        """Initialize the index query service."""
        self.index_repository = index_repository
        self.fusion_service = fusion_service

    async def get_index_by_id(self, index_id: int) -> Index | None:
        """Get an index by its ID."""
        return await self.index_repository.get(index_id)

    async def list_indexes(self) -> list[Index]:
        """List all indexes."""
        return await self.index_repository.all()

    async def search_snippets(
        self, request: MultiSearchRequest
    ) -> list[SnippetWithContext]:
        """Search snippets with filters.

        Args:
            request: The search request containing filters

        Returns:
            List of matching snippet items with context

        """
        return list(await self.index_repository.search(request))

    async def perform_fusion(
        self, rankings: list[list[FusionRequest]], k: float = 60
    ) -> list[FusionResult]:
        """Perform reciprocal rank fusion on search results."""
        return self.fusion_service.reciprocal_rank_fusion(rankings, k)

    async def get_snippets_by_ids(self, ids: list[int]) -> list[SnippetWithContext]:
        """Get snippets by their IDs."""
        snippets = await self.index_repository.get_snippets_by_ids(ids)

        # Return snippets in the same order as the ids
        snippets.sort(key=lambda x: ids.index(x.snippet.id or 0))
        return snippets
