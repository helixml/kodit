"""Tests for the null enrichment provider."""

import pytest

from kodit.domain.value_objects import EnrichmentRequest, EnrichmentResponse
from kodit.infrastructure.enrichment.null_enrichment_provider import (
    NullEnrichmentProvider,
)


class TestNullEnrichmentProvider:
    """Test the null enrichment provider."""

    def test_init(self):
        """Test initialization."""
        provider = NullEnrichmentProvider()
        assert provider is not None

    @pytest.mark.asyncio
    async def test_enrich_empty_requests(self):
        """Test enrichment with empty requests."""
        provider = NullEnrichmentProvider()
        requests = []

        results = []
        async for result in provider.enrich(requests):
            results.append(result)

        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_enrich_single_request(self):
        """Test enrichment with a single request."""
        provider = NullEnrichmentProvider()
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        results = []
        async for result in provider.enrich(requests):
            results.append(result)

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == ""

    @pytest.mark.asyncio
    async def test_enrich_multiple_requests(self):
        """Test enrichment with multiple requests."""
        provider = NullEnrichmentProvider()
        requests = [
            EnrichmentRequest(snippet_id=1, text="def hello(): pass"),
            EnrichmentRequest(snippet_id=2, text="def world(): pass"),
            EnrichmentRequest(snippet_id=3, text=""),
        ]

        results = []
        async for result in provider.enrich(requests):
            results.append(result)

        assert len(results) == 3
        assert results[0].snippet_id == 1
        assert results[0].text == ""
        assert results[1].snippet_id == 2
        assert results[1].text == ""
        assert results[2].snippet_id == 3
        assert results[2].text == ""

    @pytest.mark.asyncio
    async def test_enrich_preserves_snippet_ids(self):
        """Test that snippet IDs are preserved correctly."""
        provider = NullEnrichmentProvider()
        requests = [
            EnrichmentRequest(snippet_id=42, text="def test(): pass"),
            EnrichmentRequest(snippet_id=123, text="def another(): pass"),
        ]

        results = []
        async for result in provider.enrich(requests):
            results.append(result)

        assert len(results) == 2
        assert results[0].snippet_id == 42
        assert results[1].snippet_id == 123

    @pytest.mark.asyncio
    async def test_enrich_always_returns_empty_text(self):
        """Test that the provider always returns empty text regardless of input."""
        provider = NullEnrichmentProvider()
        requests = [
            EnrichmentRequest(snippet_id=1, text="def test(): pass"),
            EnrichmentRequest(snippet_id=2, text=""),
            EnrichmentRequest(snippet_id=3, text="   "),
            EnrichmentRequest(snippet_id=4, text="complex code with imports and logic"),
        ]

        results = []
        async for result in provider.enrich(requests):
            results.append(result)

        assert len(results) == 4
        for result in results:
            assert result.text == ""
