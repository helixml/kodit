"""Tests for the OpenAI enrichment provider."""

import os
import pytest
from unittest.mock import AsyncMock
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


@pytest.fixture
def mock_openai_client():
    """Create a mock OpenAI client instance for testing without API key."""
    return AsyncMock(spec=AsyncOpenAI)


@pytest.fixture
def mock_provider(mock_openai_client):
    """Create an OpenAIEnrichmentProvider instance with a mock client."""
    return OpenAIEnrichmentProvider(mock_openai_client)


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


@pytest.mark.asyncio
async def test_enrich_order_consistency_with_many_tasks(mock_provider):
    """Test that enrichments maintain correct order even with many parallel tasks."""
    # Create a large number of unique test strings
    num_strings = 50  # Significantly more than the parallel task limit

    # Create strings with very distinct patterns that will produce different enrichments
    # and make it easy to verify order
    test_strings = []
    for i in range(num_strings):
        # Create a string with a very distinct pattern that will produce a unique enrichment
        test_strings.append(f"def test_{i}(): print('Test {i}')")

    # Track the order of requests to verify batching
    request_order = []

    # Mock the OpenAI API response with random delays
    async def mock_create(*args, **kwargs):
        import random
        import asyncio

        # Get the user message content which contains our test data
        messages = kwargs.get("messages", [])
        user_message = next((msg for msg in messages if msg["role"] == "user"), None)
        if not user_message:
            raise ValueError("No user message found in request")

        # Extract the test number from the user message
        test_text = user_message["content"]
        test_num = int(test_text.split("_")[1].split("(")[0])

        # Record the order of this request
        request_order.append([test_num])

        # Add a random delay for this request
        await asyncio.sleep(random.uniform(0.1, 0.5))

        mock_response = AsyncMock()
        mock_response.choices = []

        # Create a mock choice with the enriched content
        mock_choice = AsyncMock()
        mock_choice.message = AsyncMock()
        mock_choice.message.content = (
            f"# Enriched version of test_{test_num}\n{test_text}"
        )
        mock_response.choices.append(mock_choice)

        return mock_response

    # Set up the mock response
    mock_provider.openai_client.chat.completions.create = AsyncMock(
        side_effect=mock_create
    )

    # Get enrichments
    enriched = await mock_provider.enrich(test_strings)

    # Verify we got the correct number of enrichments
    assert len(enriched) == num_strings

    # Verify each enrichment is valid
    assert all(isinstance(text, str) for text in enriched)
    assert all(len(text) > 0 for text in enriched)

    # Verify that the enrichments are in the correct order
    # Each enrichment should contain its original index
    for i, text in enumerate(enriched):
        assert f"test_{i}" in text, (
            f"Enrichment at position {i} does not contain expected test_{i}"
        )

    # Print the request order to help debug
    print("\nRequest order:")
    for i, batch in enumerate(request_order):
        print(f"Batch {i}: {batch}")
