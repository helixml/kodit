"""Tests for the null enrichment provider."""

import pytest

from kodit.domain.value_objects import EnrichmentRequest
from kodit.infrastructure.enrichment.null_enrichment_provider import (
    NullEnrichmentProvider,
)


class TestNullEnrichmentProvider:
    """Test the null enrichment provider."""

    def test_init(self) -> None:
        """Test initialization."""
        provider = NullEnrichmentProvider()
        assert provider is not None

    @pytest.mark.asyncio
    async def test_enrich_empty_requests(self) -> None:
        """Test enrichment with empty requests."""
        provider = NullEnrichmentProvider()
        requests: list[EnrichmentRequest] = []

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_enrich_single_request(self) -> None:
        """Test enrichment with a single request."""
        provider = NullEnrichmentProvider()
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == "def test    pass"

    @pytest.mark.asyncio
    async def test_enrich_multiple_requests(self) -> None:
        """Test enrichment with multiple requests."""
        provider = NullEnrichmentProvider()
        requests = [
            EnrichmentRequest(snippet_id=1, text="def hello(): pass"),
            EnrichmentRequest(snippet_id=2, text="def world(): pass"),
            EnrichmentRequest(snippet_id=3, text=""),
        ]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 3
        assert results[0].snippet_id == 1
        assert results[0].text == "def hello    pass"
        assert results[1].snippet_id == 2
        assert results[1].text == "def world    pass"
        assert results[2].snippet_id == 3
        assert results[2].text == ""

    @pytest.mark.asyncio
    async def test_enrich_preserves_snippet_ids(self) -> None:
        """Test that snippet IDs are preserved correctly."""
        provider = NullEnrichmentProvider()
        requests = [
            EnrichmentRequest(snippet_id=42, text="def test(): pass"),
            EnrichmentRequest(snippet_id=123, text="def another(): pass"),
        ]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 2
        assert results[0].snippet_id == 42
        assert results[0].text == "def test    pass"
        assert results[1].snippet_id == 123
        assert results[1].text == "def another    pass"

    @pytest.mark.asyncio
    async def test_enrich_filters_non_alphabetic_characters(self) -> None:
        """Test that the provider filters out non-alphabetic characters."""
        provider = NullEnrichmentProvider()
        requests = [
            EnrichmentRequest(snippet_id=1, text="def test(): pass"),
            EnrichmentRequest(snippet_id=2, text=""),
            EnrichmentRequest(snippet_id=3, text="   "),
            EnrichmentRequest(snippet_id=4, text="complex code with imports and logic"),
        ]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 4
        assert results[0].text == "def test    pass"
        assert results[1].text == ""
        assert results[2].text == "   "
        assert results[3].text == "complex code with imports and logic"
