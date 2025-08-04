"""Tests for the enrichment factory."""

from kodit.config import AppContext, Endpoint
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.infrastructure.enrichment.enrichment_factory import (
    enrichment_domain_service_factory,
)
from kodit.infrastructure.enrichment.local_enrichment_provider import (
    LocalEnrichmentProvider,
)
from kodit.infrastructure.enrichment.openai_enrichment_provider import (
    OpenAIEnrichmentProvider,
)


class TestEnrichmentFactory:
    """Test the enrichment factory."""

    def test_create_enrichment_domain_service_no_endpoint(self) -> None:
        """Test creating enrichment service with no endpoint configuration."""
        app_context = AppContext()
        app_context.default_endpoint = None
        app_context.enrichment_endpoint = None

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, LocalEnrichmentProvider)

    def test_create_enrichment_domain_service_default_openai_endpoint(self) -> None:
        """Test creating enrichment service with default OpenAI endpoint."""
        app_context = AppContext()
        app_context.default_endpoint = Endpoint(
            type="openai",
            api_key="test-key",
            base_url="https://api.openai.com/v1",
            model="gpt-4o-mini",
        )
        app_context.enrichment_endpoint = None

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)
        assert service.enrichment_provider.api_key == "test-key"
        assert service.enrichment_provider.base_url == "https://api.openai.com/v1"
        assert service.enrichment_provider.model_name == "gpt-4o-mini"

    def test_create_enrichment_domain_service_enrichment_openai_endpoint(self) -> None:
        """Test creating enrichment service with enrichment-specific OpenAI endpoint."""
        app_context = AppContext()
        app_context.default_endpoint = Endpoint(
            type="openai",
            api_key="default-key",
            base_url="https://api.openai.com/v1",
            model="gpt-4o-mini",
        )
        app_context.enrichment_endpoint = Endpoint(
            type="openai",
            api_key="enrichment-key",
            base_url="https://custom.openai.com/v1",
            model="gpt-4",
        )

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)
        assert service.enrichment_provider.api_key == "enrichment-key"
        assert service.enrichment_provider.base_url == "https://custom.openai.com/v1"
        assert service.enrichment_provider.model_name == "gpt-4"

    def test_create_enrichment_domain_service_default_openai_endpoint_no_model(
        self,
    ) -> None:
        """Test creating enrichment service with OpenAI endpoint but no model."""
        app_context = AppContext()
        app_context.default_endpoint = Endpoint(
            type="openai",
            api_key="test-key",
            base_url="https://api.openai.com/v1",
            model=None,
        )
        app_context.enrichment_endpoint = None

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)
        assert service.enrichment_provider.model_name == "gpt-4o-mini"  # Default model

    def test_create_enrichment_domain_service_default_openai_endpoint_no_base_url(
        self,
    ) -> None:
        """Test creating enrichment service with OpenAI endpoint but no base URL."""
        app_context = AppContext()
        app_context.default_endpoint = Endpoint(
            type="openai",
            api_key="test-key",
            base_url=None,
            model="gpt-4o-mini",
        )
        app_context.enrichment_endpoint = None

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)
        assert service.enrichment_provider.api_key == "test-key"
        assert service.enrichment_provider.base_url == "https://api.openai.com/v1"
        assert service.enrichment_provider.model_name == "gpt-4o-mini"

    def test_create_enrichment_domain_service_default_openai_endpoint_no_api_key(
        self,
    ) -> None:
        """Test creating enrichment service with OpenAI endpoint but no API key."""
        app_context = AppContext()
        app_context.default_endpoint = Endpoint(
            type="openai",
            api_key=None,
            base_url="https://api.openai.com/v1",
            model="gpt-4o-mini",
        )
        app_context.enrichment_endpoint = None

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)
        assert service.enrichment_provider.api_key is None
        assert service.enrichment_provider.base_url == "https://api.openai.com/v1"
        assert service.enrichment_provider.model_name == "gpt-4o-mini"

    def test_create_enrichment_domain_service_non_openai_endpoint(self) -> None:
        """Test creating enrichment service with non-OpenAI endpoint."""
        app_context = AppContext()
        app_context.default_endpoint = Endpoint(
            type=None,
            api_key="test-key",
            base_url="https://other.com/v1",
            model="other-model",
        )
        app_context.enrichment_endpoint = None

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, LocalEnrichmentProvider)

    def test_create_enrichment_domain_service_enrichment_non_openai_endpoint(
        self,
    ) -> None:
        """Test creating enrichment service with non-OpenAI endpoint."""
        app_context = AppContext()
        app_context.default_endpoint = Endpoint(
            type="openai",
            api_key="default-key",
            base_url="https://api.openai.com/v1",
            model="gpt-4o-mini",
        )
        app_context.enrichment_endpoint = Endpoint(
            type=None,
            api_key="enrichment-key",
            base_url="https://other.com/v1",
            model="other-model",
        )

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, LocalEnrichmentProvider)

    def test_create_enrichment_domain_service_with_socket_path(self) -> None:
        """Test creating enrichment service with socket path."""
        app_context = AppContext()
        app_context.default_endpoint = Endpoint(
            type="openai",
            api_key="test-key",
            socket_path="/tmp/openai.sock",  # noqa: S108
            model="gpt-4o-mini",
        )
        app_context.enrichment_endpoint = None

        service = enrichment_domain_service_factory(app_context)

        assert isinstance(service, EnrichmentDomainService)
        assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)
        assert service.enrichment_provider.socket_path == "/tmp/openai.sock"  # noqa: S108
        assert service.enrichment_provider.api_key == "test-key"
        assert service.enrichment_provider.model_name == "gpt-4o-mini"
