"""Tests for the enrichment domain service."""

from collections.abc import AsyncGenerator
from unittest.mock import MagicMock

import pytest

from kodit.domain.services.enrichment_service import (
    EnrichmentDomainService,
    EnrichmentProvider,
)
from kodit.domain.value_objects import (
    EnrichmentIndexRequest,
    EnrichmentRequest,
    EnrichmentResponse,
)


class MockEnrichmentProvider(MagicMock):
    """Mock enrichment provider for testing."""

    def __init__(self) -> None:
        """Initialize the mock enrichment provider."""
        super().__init__(spec=EnrichmentProvider)
        # enrich will be set per test


@pytest.fixture
def mock_enrichment_provider() -> MockEnrichmentProvider:
    """Create a mock enrichment provider."""
    return MockEnrichmentProvider()


@pytest.fixture
def enrichment_domain_service(
    mock_enrichment_provider: MockEnrichmentProvider,
) -> EnrichmentDomainService:
    """Create an enrichment domain service with mocked provider."""
    return EnrichmentDomainService(mock_enrichment_provider)


@pytest.mark.asyncio
async def test_enrich_documents_success(
    enrichment_domain_service: EnrichmentDomainService,
    mock_enrichment_provider: MockEnrichmentProvider,
) -> None:
    """Test successful document enrichment."""
    # Setup
    requests = [
        EnrichmentRequest(snippet_id=1, text="def hello(): pass"),
        EnrichmentRequest(snippet_id=2, text="def world(): pass"),
    ]
    enrichment_request = EnrichmentIndexRequest(requests=requests)

    # Mock enrichment responses
    async def mock_enrichment() -> AsyncGenerator[EnrichmentResponse, None]:
        yield EnrichmentResponse(snippet_id=1, text="enriched: def hello(): pass")
        yield EnrichmentResponse(snippet_id=2, text="enriched: def world(): pass")

    mock_enrichment_provider.enrich = lambda _: mock_enrichment()

    # Execute
    results = [
        response
        async for response in enrichment_domain_service.enrich_documents(
            enrichment_request
        )
    ]

    # Verify
    assert len(results) == 2
    assert results[0].snippet_id == 1
    assert results[0].text == "enriched: def hello(): pass"
    assert results[1].snippet_id == 2
    assert results[1].text == "enriched: def world(): pass"


@pytest.mark.asyncio
async def test_enrich_documents_empty_requests(
    enrichment_domain_service: EnrichmentDomainService,
    mock_enrichment_provider: MockEnrichmentProvider,
) -> None:
    """Test enrichment with empty requests."""
    # Setup
    enrichment_request = EnrichmentIndexRequest(requests=[])

    async def mock_enrichment() -> AsyncGenerator[EnrichmentResponse, None]:
        if False:
            yield  # type: ignore[unreachable]

    mock_enrichment_provider.enrich = lambda _: mock_enrichment()

    # Execute
    results = [
        response
        async for response in enrichment_domain_service.enrich_documents(
            enrichment_request
        )
    ]

    # Verify
    assert len(results) == 0


@pytest.mark.asyncio
async def test_enrich_documents_single_request(
    enrichment_domain_service: EnrichmentDomainService,
    mock_enrichment_provider: MockEnrichmentProvider,
) -> None:
    """Test enrichment with a single request."""
    # Setup
    requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]
    enrichment_request = EnrichmentIndexRequest(requests=requests)

    async def mock_enrichment() -> AsyncGenerator[EnrichmentResponse, None]:
        yield EnrichmentResponse(snippet_id=1, text="enriched: def test(): pass")

    mock_enrichment_provider.enrich = lambda _: mock_enrichment()

    # Execute
    results = [
        response
        async for response in enrichment_domain_service.enrich_documents(
            enrichment_request
        )
    ]

    # Verify
    assert len(results) == 1
    assert results[0].snippet_id == 1
    assert results[0].text == "enriched: def test(): pass"
