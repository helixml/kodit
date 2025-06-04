"""Tests for the OpenAI enrichment provider."""

import os
import pytest
from openai import AsyncOpenAI

from kodit.enrichment.enrichment_provider.openai_enrichment_provider import (
    OpenAIEnrichmentProvider,
)


def skip_if_no_api_key():
    """Skip test if OPENAI_API_KEY is not set."""
    if not os.getenv("OPENAI_API_KEY"):
        pytest.skip("OPENAI_API_KEY environment variable is not set, skipping test")


@pytest.fixture
def openai_client():
    """Create an OpenAI client instance."""
    skip_if_no_api_key()
    return AsyncOpenAI()


@pytest.fixture
def provider(openai_client):
    """Create an OpenAIEnrichmentProvider instance."""
    return OpenAIEnrichmentProvider(openai_client)


@pytest.mark.asyncio
async def test_initialization(openai_client):
    """Test that the provider initializes correctly."""
    skip_if_no_api_key()

    # Test with default model
    provider = OpenAIEnrichmentProvider(openai_client)
    assert provider.model_name == "gpt-4o-mini"

    # Test with custom model
    custom_model = "gpt-4"
    provider = OpenAIEnrichmentProvider(openai_client, model_name=custom_model)
    assert provider.model_name == custom_model


@pytest.mark.asyncio
async def test_enrich_single_text(provider):
    """Test enriching a single text."""
    skip_if_no_api_key()

    text = "def hello(): print('Hello, world!')"
    enriched = await provider.enrich([text])

    assert len(enriched) == 1
    assert isinstance(enriched[0], str)
    assert len(enriched[0]) > 0


@pytest.mark.asyncio
async def test_enrich_multiple_texts(provider):
    """Test enriching multiple texts."""
    skip_if_no_api_key()

    texts = [
        "def hello(): print('Hello, world!')",
        "def add(a, b): return a + b",
        "def multiply(a, b): return a * b",
    ]
    enriched = await provider.enrich(texts)

    assert len(enriched) == 3
    assert all(isinstance(text, str) for text in enriched)
    assert all(len(text) > 0 for text in enriched)


@pytest.mark.asyncio
async def test_enrich_empty_list(provider):
    """Test enriching an empty list."""
    skip_if_no_api_key()

    enriched = await provider.enrich([])
    assert len(enriched) == 0


@pytest.mark.asyncio
async def test_enrich_error_handling(provider):
    """Test error handling for invalid inputs."""
    skip_if_no_api_key()

    # Test with None
    enriched = await provider.enrich([None])
    assert len(enriched) == 1
    assert enriched[0] == ""

    # Test with empty string
    enriched = await provider.enrich([""])
    assert len(enriched) == 1
    assert enriched[0] == ""


@pytest.mark.asyncio
async def test_enrich_parallel_processing(provider):
    """Test that multiple enrichments can be processed in parallel."""
    skip_if_no_api_key()

    # Create multiple texts to test parallel processing
    texts = [f"def test{i}(): print('Test {i}')" for i in range(20)]
    enriched = await provider.enrich(texts)

    assert len(enriched) == 20
    assert all(isinstance(text, str) for text in enriched)
    assert all(len(text) > 0 for text in enriched)
