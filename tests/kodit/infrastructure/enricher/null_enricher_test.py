"""Tests for null enricher."""

import pytest

from kodit.domain.enrichments.request import EnrichmentRequest
from kodit.infrastructure.enricher.null_enricher import NullEnricher


@pytest.mark.asyncio
async def test_null_enricher_returns_empty_content() -> None:
    """Test that null enricher returns empty content for all requests."""
    enricher = NullEnricher()

    requests = [
        EnrichmentRequest(
            id="req1",
            text="Analyze this code",
            system_prompt="You are a code analyzer",
        ),
        EnrichmentRequest(
            id="req2",
            text="Another request",
            system_prompt="You are helpful",
        ),
    ]

    results = [response async for response in enricher.enrich(requests)]

    assert len(results) == 2
    assert all(r.text == "" for r in results)
    assert results[0].id == "req1"
    assert results[1].id == "req2"
