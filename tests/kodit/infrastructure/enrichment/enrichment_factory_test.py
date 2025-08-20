"""Tests for the enrichment factory."""

from kodit.config import AppContext, Endpoint
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.infrastructure.enrichment.enrichment_factory import (
    enrichment_domain_service_factory,
)
from kodit.infrastructure.enrichment.litellm_enrichment_provider import (
    LiteLLMEnrichmentProvider,
)
from kodit.infrastructure.enrichment.local_enrichment_provider import (
    LocalEnrichmentProvider,
)


class TestEnrichmentFactory:
    """Test the enrichment factory."""

    def test_create_enrichment_domain_service_no_endpoint(self) -> None:
        """Test creating enrichment service with no endpoint configuration."""
        app_context = AppContext()
        app_context.enrichment_endpoint = None

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, LocalEnrichmentProvider)

    def test_create_enrichment_domain_service_enrichment_openai_endpoint(self) -> None:
        """Test creating enrichment service with enrichment-specific OpenAI endpoint."""
        app_context = AppContext()
        app_context.enrichment_endpoint = Endpoint(
            api_key="enrichment-key",
            base_url="https://custom.openai.com/v1",
            model="gpt-4",
        )

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, LiteLLMEnrichmentProvider)
        assert service.enrichment_provider.api_key == "enrichment-key"
        assert service.enrichment_provider.base_url == "https://custom.openai.com/v1"
        assert service.enrichment_provider.model_name == "gpt-4"
