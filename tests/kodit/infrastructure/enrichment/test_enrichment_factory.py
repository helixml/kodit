"""Tests for the enrichment factory."""

from unittest.mock import MagicMock, patch

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

        with patch("openai.AsyncOpenAI") as mock_openai:
            mock_client = MagicMock()
            mock_openai.return_value = mock_client

            service = enrichment_domain_service_factory(app_context)

            assert isinstance(service, EnrichmentDomainService)
            assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)
            assert service.enrichment_provider.openai_client == mock_client
            assert service.enrichment_provider.model_name == "gpt-4o-mini"

            # Verify OpenAI client was created with correct parameters
        mock_openai.assert_called_once_with(
            api_key="test-key",
            base_url="https://api.openai.com/v1",
            timeout=60,
            max_retries=2,
        )

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

        with patch("openai.AsyncOpenAI") as mock_openai:
            mock_client = MagicMock()
            mock_openai.return_value = mock_client

            service = enrichment_domain_service_factory(app_context)

            assert isinstance(service, EnrichmentDomainService)
            assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)
            assert service.enrichment_provider.openai_client == mock_client
            assert service.enrichment_provider.model_name == "gpt-4"

            # Verify OpenAI client was created with enrichment endpoint parameters
        mock_openai.assert_called_once_with(
            api_key="enrichment-key",
            base_url="https://custom.openai.com/v1",
            timeout=60,
            max_retries=2,
        )

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

        with patch("openai.AsyncOpenAI") as mock_openai:
            mock_client = MagicMock()
            mock_openai.return_value = mock_client

            service = enrichment_domain_service_factory(app_context)

            assert isinstance(service, EnrichmentDomainService)
            assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)
            assert (
                service.enrichment_provider.model_name == "gpt-4o-mini"
            )  # Default model

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

        with patch("openai.AsyncOpenAI") as mock_openai:
            mock_client = MagicMock()
            mock_openai.return_value = mock_client

            service = enrichment_domain_service_factory(app_context)

            assert isinstance(service, EnrichmentDomainService)
            assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)

            # Verify OpenAI client was created with default base URL
        mock_openai.assert_called_once_with(
            api_key="test-key",
            base_url="https://api.openai.com/v1",
            timeout=60,
            max_retries=2,
        )

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

        with patch("openai.AsyncOpenAI") as mock_openai:
            mock_client = MagicMock()
            mock_openai.return_value = mock_client

            service = enrichment_domain_service_factory(app_context)

            assert isinstance(service, EnrichmentDomainService)
            assert isinstance(service.enrichment_provider, OpenAIEnrichmentProvider)

            # Verify OpenAI client was created with default API key
        mock_openai.assert_called_once_with(
            api_key="default",
            base_url="https://api.openai.com/v1",
            timeout=60,
            max_retries=2,
        )

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
