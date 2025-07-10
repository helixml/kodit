"""Integration test for the local enrichment provider with real models."""

import pytest

from kodit.domain.value_objects import EnrichmentRequest
from kodit.infrastructure.enrichment.local_enrichment_provider import (
    LocalEnrichmentProvider,
)


class TestLocalEnrichmentProviderIntegration:
    """Integration tests for local enrichment provider with real models."""

    @pytest.mark.asyncio
    @pytest.mark.slow
    async def test_real_model_enrichment(self) -> None:
        """Test enrichment with a real small model.

        Uses Qwen/Qwen2.5-0.5B-Instruct which is tiny (~0.5B parameters)
        and supports chat templates for realistic testing.
        """
        # Use the smallest Qwen model that supports chat templates
        provider = LocalEnrichmentProvider(
            context_window=50  # Very small for speed
        )

        # Test with a simple Python function
        request = EnrichmentRequest(snippet_id=42, text="def add(x, y): return x + y")

        # Get enrichment result
        results = [result async for result in provider.enrich([request])]

        # Verify basic functionality
        assert len(results) == 1
        result = results[0]

        assert result.snippet_id == 42
        assert isinstance(result.text, str)
        assert len(result.text.strip()) > 0

        # The enriched text should be different from input
        assert result.text.strip() != "def add(x, y): return x + y"

        # The output should contain some reasonable explanation
        # (not checking exact content since model outputs can vary)
        assert "function" in result.text.lower() or "add" in result.text.lower()
