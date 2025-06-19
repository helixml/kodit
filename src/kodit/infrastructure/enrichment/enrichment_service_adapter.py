"""Adapter for backward compatibility with old enrichment service interface."""

from collections.abc import AsyncGenerator

from kodit.domain.models import (
    EnrichmentIndexRequest,
    EnrichmentRequest,
)
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.infrastructure.enrichment.legacy_enrichment_models import (
    EnrichmentRequest as OldEnrichmentRequest,
)
from kodit.infrastructure.enrichment.legacy_enrichment_models import (
    EnrichmentResponse as OldEnrichmentResponse,
)
from kodit.infrastructure.enrichment.legacy_enrichment_models import (
    EnrichmentService,
)


class EnrichmentServiceAdapter(EnrichmentService):
    """Adapter to maintain backward compatibility with old enrichment service interface."""

    def __init__(self, enrichment_domain_service: EnrichmentDomainService) -> None:
        """Initialize the adapter.

        Args:
            enrichment_domain_service: The enrichment domain service to adapt.

        """
        self.enrichment_domain_service = enrichment_domain_service

    async def enrich(
        self, data: list[OldEnrichmentRequest]
    ) -> AsyncGenerator[OldEnrichmentResponse, None]:
        """Enrich a list of requests using the domain service.

        Args:
            data: List of old enrichment requests.

        Yields:
            Old enrichment responses as they are processed.

        """
        # Convert old requests to new domain models
        requests = [
            EnrichmentRequest(snippet_id=req.snippet_id, text=req.text) for req in data
        ]

        # Create domain request
        domain_request = EnrichmentIndexRequest(requests=requests)

        # Use domain service
        async for response in self.enrichment_domain_service.enrich_documents(
            domain_request
        ):
            # Convert back to old response format
            yield OldEnrichmentResponse(
                snippet_id=response.snippet_id,
                text=response.text,
            )
