"""Bundle of services for commit processing operations."""

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from kodit.application.services.commit_scanning_service import (
        CommitScanningService,
    )
    from kodit.application.services.enrichment_generation_service import (
        EnrichmentGenerationService,
    )
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )
    from kodit.application.services.search_indexing_service import (
        SearchIndexingService,
    )
    from kodit.application.services.snippet_extraction_service import (
        SnippetExtractionService,
    )


class CommitProcessingServices:
    """Bundles all services needed for commit processing.

    This is a Parameter Object pattern to reduce constructor complexity.
    """

    def __init__(
        self,
        commit_scanning_service: "CommitScanningService",
        snippet_extraction_service: "SnippetExtractionService",
        search_indexing_service: "SearchIndexingService",
        enrichment_generation_service: "EnrichmentGenerationService",
        enrichment_query_service: "EnrichmentQueryService",
    ) -> None:
        """Initialize commit processing services bundle."""
        self.commit_scanning = commit_scanning_service
        self.snippet_extraction = snippet_extraction_service
        self.search_indexing = search_indexing_service
        self.enrichment_generation = enrichment_generation_service
        self.enrichment_query = enrichment_query_service
