"""Simplified application service using the new Index aggregate design."""

from pathlib import Path
from typing import TYPE_CHECKING

import structlog
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.models import entities as domain_entities
from kodit.domain.models.value_objects import SourceType
from kodit.domain.services.index_service import IndexDomainService
from kodit.log import log_event

if TYPE_CHECKING:
    # Keep these for backward compatibility during transition
    from kodit.domain.services.bm25_service import BM25DomainService
    from kodit.domain.services.embedding_service import EmbeddingDomainService
    from kodit.domain.services.enrichment_service import EnrichmentDomainService


class SimplifiedIndexingApplicationService:
    """Simplified application service using the new Index aggregate design.

    This service demonstrates how the application layer becomes much simpler
    when using the Index aggregate root pattern. The complex orchestration
    logic moves into the domain service where it belongs.
    """

    def __init__(
        self,
        index_domain_service: IndexDomainService,
        session: AsyncSession,
        # Keep these during transition for search functionality
        bm25_service: "BM25DomainService | None" = None,
        code_search_service: "EmbeddingDomainService | None" = None,
        text_search_service: "EmbeddingDomainService | None" = None,
        enrichment_service: "EnrichmentDomainService | None" = None,
    ) -> None:
        """Initialize the simplified indexing service."""
        self._index_domain_service = index_domain_service
        self._session = session

        # Legacy services for gradual migration
        self._bm25_service = bm25_service
        self._code_search_service = code_search_service
        self._text_search_service = text_search_service
        self._enrichment_service = enrichment_service

        self.log = structlog.get_logger(__name__)

    async def create_index_from_uri(
        self, uri: AnyUrl, progress_callback: ProgressCallback | None = None
    ) -> domain_entities.Index:
        """Create a new index from a source URI.

        This is the new, simplified way to create indexes using the aggregate root.
        """
        log_event("kodit.index.create")

        # Use domain service to create index
        index = await self._index_domain_service.create_index(uri)

        # Commit the creation
        await self._session.commit()

        self.log.info("Index created", index_id=index.id, uri=str(uri))
        return index

    async def run_complete_indexing_workflow(
        self,
        uri: AnyUrl,
        local_path: Path,
        source_type: SourceType = SourceType.GIT,
        extraction_strategy: SnippetExtractionStrategy = SnippetExtractionStrategy.METHOD_BASED,
        progress_callback: ProgressCallback | None = None,
    ) -> domain_entities.Index:
        """Run the complete indexing workflow for a source.

        This method demonstrates the power of the aggregate root approach:
        - Single method call handles the entire workflow
        - Domain service encapsulates all business logic
        - Application service just coordinates and manages transactions
        """
        log_event("kodit.index.complete_workflow")

        self.log.info(
            "Starting complete indexing workflow",
            uri=str(uri),
            local_path=str(local_path),
            source_type=source_type.value,
        )

        # Step 1: Create or get existing index
        index = await self._index_domain_service.create_index(uri)
        await self._session.commit()

        # Step 2: Clone and populate working copy
        index = await self._index_domain_service.clone_and_populate_working_copy(
            index=index,
            local_path=local_path,
            source_type=source_type,
            progress_callback=progress_callback,
        )
        await self._session.commit()

        # Step 3: Extract snippets
        index = await self._index_domain_service.extract_snippets(
            index=index,
            strategy=extraction_strategy,
            progress_callback=progress_callback,
        )
        await self._session.commit()

        # Step 4: Enrich snippets (when implemented)
        index = await self._index_domain_service.enrich_snippets_with_summaries(
            index=index, progress_callback=progress_callback
        )
        await self._session.commit()

        self.log.info(
            "Complete indexing workflow finished",
            index_id=index.id,
            file_count=len(index.source.working_copy.files),
        )

        return index

    async def get_index_by_uri(self, uri: AnyUrl) -> domain_entities.Index | None:
        """Get an existing index by source URI."""
        return await self._index_domain_service.get_index_by_uri(uri)

    async def get_index_by_id(self, index_id: int) -> domain_entities.Index | None:
        """Get an existing index by ID."""
        return await self._index_domain_service.get_index_by_id(index_id)

    # Legacy compatibility methods for gradual migration

    async def create_index_legacy(self, source_id: int) -> dict:
        """Legacy method for backward compatibility.

        This shows how you can gradually migrate from the old API
        while using the new domain service underneath.
        """
        # For now, convert source_id to URI (would need source service lookup)
        # This is just a placeholder - real implementation would look up the source
        uri = AnyUrl(f"placeholder://source/{source_id}")

        index = await self.create_index_from_uri(uri)

        # Return old format for compatibility
        return {
            "id": index.id,
            "source_id": source_id,  # Placeholder
            "created_at": index.created_at,
            "num_snippets": 0,  # Would need to count snippets
        }

    async def run_index_legacy(
        self, index_id: int, progress_callback: ProgressCallback | None = None
    ) -> None:
        """Legacy method for backward compatibility.

        Shows how to adapt the old interface to use the new domain service.
        """
        # Get the index
        index = await self._index_domain_service.get_index_by_id(index_id)
        if not index:
            raise ValueError(f"Index not found: {index_id}")

        # Use the old workflow but with new domain service
        # This would need cloning logic to determine local_path and source_type
        local_path = Path("/tmp/kodit") / str(index_id)  # Placeholder
        source_type = SourceType.GIT  # Would need to be determined

        await self.run_complete_indexing_workflow(
            uri=index.source.working_copy.remote_uri,
            local_path=local_path,
            source_type=source_type,
            progress_callback=progress_callback,
        )


class IndexWorkflowBuilder:
    """Builder pattern for complex indexing workflows.

    This demonstrates how the application layer can provide
    convenience builders while keeping the domain clean.
    """

    def __init__(self, index_service: SimplifiedIndexingApplicationService):
        self._index_service = index_service
        self._uri: AnyUrl | None = None
        self._local_path: Path | None = None
        self._source_type = SourceType.GIT
        self._extraction_strategy = SnippetExtractionStrategy.METHOD_BASED
        self._progress_callback: ProgressCallback | None = None

    def from_uri(self, uri: AnyUrl) -> "IndexWorkflowBuilder":
        """Set the source URI."""
        self._uri = uri
        return self

    def to_local_path(self, path: Path) -> "IndexWorkflowBuilder":
        """Set the local clone path."""
        self._local_path = path
        return self

    def with_source_type(self, source_type: SourceType) -> "IndexWorkflowBuilder":
        """Set the source type."""
        self._source_type = source_type
        return self

    def with_extraction_strategy(
        self, strategy: SnippetExtractionStrategy
    ) -> "IndexWorkflowBuilder":
        """Set the snippet extraction strategy."""
        self._extraction_strategy = strategy
        return self

    def with_progress_callback(
        self, callback: ProgressCallback
    ) -> "IndexWorkflowBuilder":
        """Set a progress callback."""
        self._progress_callback = callback
        return self

    async def execute(self) -> domain_entities.Index:
        """Execute the configured workflow."""
        if not self._uri:
            raise ValueError("URI must be set")
        if not self._local_path:
            raise ValueError("Local path must be set")

        return await self._index_service.run_complete_indexing_workflow(
            uri=self._uri,
            local_path=self._local_path,
            source_type=self._source_type,
            extraction_strategy=self._extraction_strategy,
            progress_callback=self._progress_callback,
        )


# Example usage:
#
# # Simple case
# index = await service.create_index_from_uri(AnyUrl("https://github.com/user/repo.git"))
#
# # Complete workflow
# index = await service.run_complete_indexing_workflow(
#     uri=AnyUrl("https://github.com/user/repo.git"),
#     local_path=Path("/tmp/my-repo"),
#     source_type=SourceType.GIT
# )
#
# # Builder pattern
# index = await (IndexWorkflowBuilder(service)
#     .from_uri(AnyUrl("https://github.com/user/repo.git"))
#     .to_local_path(Path("/tmp/my-repo"))
#     .with_source_type(SourceType.GIT)
#     .with_extraction_strategy(SnippetExtractionStrategy.METHOD_BASED)
#     .execute())
