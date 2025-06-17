"""Tests for the OpenAI enrichment provider."""

import pytest
from unittest.mock import AsyncMock

# The real OpenAI client is not required for unit tests; instead we mock it.
from openai import AsyncOpenAI

from kodit.enrichment.enrichment_provider.enrichment_provider import EnrichmentRequest
from kodit.enrichment.enrichment_provider.openai_enrichment_provider import (
    OpenAIEnrichmentProvider,
)


# ---------------------------------------------------------------------------
# Fixtures & helpers
# ---------------------------------------------------------------------------


def _default_openai_response(content: str = "# Enriched\n"):
    """Return a minimal mocked OpenAI chat completion response."""

    mock_response = AsyncMock()
    mock_choice = AsyncMock()
    mock_choice.message = AsyncMock()
    mock_choice.message.content = content
    mock_response.choices = [mock_choice]
    return mock_response


@pytest.fixture
def openai_client():
    """Return a mocked *AsyncOpenAI* instance with sensible defaults."""

    client = AsyncMock(spec=AsyncOpenAI)

    async def _create(*_args, **_kwargs):  # noqa: D401
        return _default_openai_response()

    # The provider calls: client.chat.completions.create(...)
    client.chat.completions.create = AsyncMock(side_effect=_create)  # type: ignore[attr-defined]

    return client


@pytest.fixture
def provider(openai_client):
    """Create an *OpenAIEnrichmentProvider* backed by mocked client."""
    return OpenAIEnrichmentProvider(openai_client)


@pytest.fixture
def mock_openai_client():
    """Return a fresh mocked *AsyncOpenAI* for explicit per-test customisation."""
    return AsyncMock(spec=AsyncOpenAI)


@pytest.fixture
def mock_provider(mock_openai_client):
    """OpenAIEnrichmentProvider using *mock_openai_client* passed in."""
    return OpenAIEnrichmentProvider(mock_openai_client)


@pytest.mark.asyncio
async def test_initialization(openai_client):
    """Provider initialises with correct default & custom model names."""

    # Test with default model
    provider = OpenAIEnrichmentProvider(openai_client)
    assert provider.model_name == "gpt-4o-mini"

    # Test with custom model
    custom_model = "gpt-4"
    provider = OpenAIEnrichmentProvider(openai_client, model_name=custom_model)
    assert provider.model_name == custom_model


@pytest.mark.asyncio
async def test_enrich_single_text(provider):
    """Enrich a single snippet using mocked OpenAI API."""

    text = "def hello(): print('Hello, world!')"
    enriched = [
        response
        async for response in provider.enrich(
            [EnrichmentRequest(snippet_id=0, text=text)]
        )
    ]

    assert len(enriched) == 1
    assert isinstance(enriched[0].text, str)
    assert len(enriched[0].text) > 0


@pytest.mark.asyncio
async def test_enrich_multiple_texts(provider):
    """Enrich multiple snippets via mocked OpenAI API."""

    texts = [
        "def hello(): print('Hello, world!')",
        "def add(a, b): return a + b",
        "def multiply(a, b): return a * b",
    ]
    enriched = [
        response
        async for response in provider.enrich(
            [EnrichmentRequest(snippet_id=i, text=text) for i, text in enumerate(texts)]
        )
    ]

    assert len(enriched) == 3
    assert all(isinstance(text.text, str) for text in enriched)
    assert all(len(text.text) > 0 for text in enriched)


@pytest.mark.asyncio
async def test_enrich_empty_list(provider):
    """Enriching an empty list should yield no responses."""

    enriched = [response async for response in provider.enrich([])]
    assert len(enriched) == 0


@pytest.mark.asyncio
async def test_enrich_error_handling(provider):
    """Provider returns empty text for empty snippet input."""

    # Test with empty string
    enriched = [
        response
        async for response in provider.enrich(
            [EnrichmentRequest(snippet_id=0, text="")]
        )
    ]
    assert len(enriched) == 1
    assert enriched[0].text == ""


@pytest.mark.asyncio
async def test_enrich_parallel_processing(provider):
    """Ensure provider handles many requests concurrently (mocked)."""

    # Create multiple texts to test parallel processing
    texts = [f"def test{i}(): print('Test {i}')" for i in range(20)]
    enriched = [
        response
        async for response in provider.enrich(
            [EnrichmentRequest(snippet_id=i, text=text) for i, text in enumerate(texts)]
        )
    ]

    assert len(enriched) == 20
    assert all(isinstance(text.text, str) for text in enriched)
    assert all(len(text.text) > 0 for text in enriched)


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
        test_strings.append(
            EnrichmentRequest(snippet_id=i, text=f"def test_{i}(): print('Test {i}')")
        )

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
    enriched = [response async for response in mock_provider.enrich(test_strings)]

    # Verify we got the correct number of enrichments
    assert len(enriched) == num_strings

    # Verify each enrichment is valid
    assert all(isinstance(text.text, str) for text in enriched)
    assert all(len(text.text) > 0 for text in enriched)

    # Verify that the enrichments are in the correct order
    # Each enrichment should contain its original index
    for response in enriched:
        assert f"test_{response.snippet_id}" in response.text, (
            f"Enrichment at position {response.snippet_id} does not contain expected test_{response.snippet_id}"
        )

    # Print the request order to help debug
    print("\nRequest order:")
    for i, batch in enumerate(request_order):
        print(f"Batch {i}: {batch}")
