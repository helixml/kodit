"""Tests for the enrichment domain service."""

from collections.abc import AsyncGenerator
from unittest.mock import MagicMock

import pytest

from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.request import EnrichmentRequest
from kodit.domain.enrichments.response import EnrichmentResponse
from kodit.domain.services.enrichment_service import EnrichmentDomainService


class MockEnricher(MagicMock):
    """Mock enricher for testing."""

    def __init__(self) -> None:
        """Initialize the mock enricher."""
        super().__init__(spec=Enricher)
        # enrich will be set per test


@pytest.fixture
def mock_enricher() -> MockEnricher:
    """Create a mock enricher."""
    return MockEnricher()


@pytest.fixture
def enrichment_domain_service(
    mock_enricher: MockEnricher,
) -> EnrichmentDomainService:
    """Create an enrichment domain service with mocked enricher."""
    return EnrichmentDomainService(mock_enricher)


@pytest.mark.asyncio
async def test_enrich_documents_success(
    enrichment_domain_service: EnrichmentDomainService,
    mock_enricher: MockEnricher,
) -> None:
    """Test successful document enrichment."""
    # Setup
    requests = [
        EnrichmentRequest(id="1", text="def hello(): pass", system_prompt="Explain"),
        EnrichmentRequest(id="2", text="def world(): pass", system_prompt="Explain"),
    ]

    # Mock enrichment responses
    async def mock_enrichment() -> AsyncGenerator[EnrichmentResponse, None]:
        yield EnrichmentResponse(id="1", text="enriched: def hello(): pass")
        yield EnrichmentResponse(id="2", text="enriched: def world(): pass")

    mock_enricher.enrich = lambda _: mock_enrichment()

    # Execute
    results = [
        response
        async for response in enrichment_domain_service.enrich_documents(requests)
    ]

    # Verify
    assert len(results) == 2
    assert results[0].id == "1"
    assert results[0].text == "enriched: def hello(): pass"
    assert results[1].id == "2"
    assert results[1].text == "enriched: def world(): pass"


@pytest.mark.asyncio
async def test_enrich_documents_empty_requests(
    enrichment_domain_service: EnrichmentDomainService,
    mock_enricher: MockEnricher,
) -> None:
    """Test enrichment with empty requests."""
    # Setup
    requests: list[EnrichmentRequest] = []

    async def mock_enrichment() -> AsyncGenerator[EnrichmentResponse, None]:
        if False:
            yield  # type: ignore[unreachable]

    mock_enricher.enrich = lambda _: mock_enrichment()

    # Execute
    results = [
        response
        async for response in enrichment_domain_service.enrich_documents(requests)
    ]

    # Verify
    assert len(results) == 0


@pytest.mark.asyncio
async def test_enrich_documents_single_request(
    enrichment_domain_service: EnrichmentDomainService,
    mock_enricher: MockEnricher,
) -> None:
    """Test enrichment with a single request."""
    # Setup
    requests = [
        EnrichmentRequest(id="1", text="def test(): pass", system_prompt="Explain")
    ]

    async def mock_enrichment() -> AsyncGenerator[EnrichmentResponse, None]:
        yield EnrichmentResponse(id="1", text="enriched: def test(): pass")

    mock_enricher.enrich = lambda _: mock_enrichment()

    # Execute
    results = [
        response
        async for response in enrichment_domain_service.enrich_documents(requests)
    ]

    # Verify
    assert len(results) == 1
    assert results[0].id == "1"
    assert results[0].text == "enriched: def test(): pass"
