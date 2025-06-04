"""Tests for the OpenAI embedding provider."""

import os
import pytest
from unittest.mock import AsyncMock, patch
from openai import AsyncOpenAI

from kodit.embedding.embedding_provider.openai_embedding_provider import (
    OpenAIEmbeddingProvider,
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
def mock_openai_client():
    """Create a mock OpenAI client instance for testing without API key."""
    return AsyncMock(spec=AsyncOpenAI)


@pytest.fixture
def mock_provider(mock_openai_client):
    """Create an OpenAIEmbeddingProvider instance with a mock client."""
    return OpenAIEmbeddingProvider(mock_openai_client)


@pytest.fixture
def provider(openai_client):
    """Create an OpenAIEmbeddingProvider instance."""
    return OpenAIEmbeddingProvider(openai_client)


@pytest.mark.asyncio
async def test_initialization(openai_client):
    """Test that the provider initializes correctly."""

    # Test with default model
    provider = OpenAIEmbeddingProvider(openai_client)
    assert provider.model_name == "text-embedding-3-small"

    # Test with custom model
    custom_model = "text-embedding-3-large"
    provider = OpenAIEmbeddingProvider(openai_client, model_name=custom_model)
    assert provider.model_name == custom_model


@pytest.mark.asyncio
async def test_embed_single_text(provider):
    """Test embedding a single text."""
    skip_if_no_api_key()

    text = "This is a test sentence."
    embeddings = await provider.embed([text])

    assert len(embeddings) == 1
    assert isinstance(embeddings[0], list)
    assert all(isinstance(x, float) for x in embeddings[0])


@pytest.mark.asyncio
async def test_embed_multiple_texts(provider):
    """Test embedding multiple texts."""

    texts = ["First test sentence.", "Second test sentence.", "Third test sentence."]
    embeddings = await provider.embed(texts)

    assert len(embeddings) == 3
    assert all(isinstance(emb, list) for emb in embeddings)
    assert all(isinstance(x, float) for emb in embeddings for x in emb)


@pytest.mark.asyncio
async def test_embed_empty_list(provider):
    """Test embedding an empty list."""

    embeddings = await provider.embed([])
    assert len(embeddings) == 0


@pytest.mark.asyncio
async def test_embed_large_text(provider):
    """Test embedding a large text that might need batching."""

    # Create a large text that exceeds typical token limits
    large_text = "This is a test sentence. " * 1000
    embeddings = await provider.embed([large_text])

    assert len(embeddings) == 1
    assert isinstance(embeddings[0], list)
    assert all(isinstance(x, float) for x in embeddings[0])


@pytest.mark.asyncio
async def test_embed_special_characters(provider):
    """Test embedding text with special characters."""

    texts = [
        "Hello, world!",
        "Test with numbers: 123",
        "Special chars: @#$%^&*()",
        "Unicode: 你好世界",
    ]
    embeddings = await provider.embed(texts)

    assert len(embeddings) == 4
    assert all(isinstance(emb, list) for emb in embeddings)
    assert all(isinstance(x, float) for emb in embeddings for x in emb)


@pytest.mark.asyncio
async def test_embed_consistency(provider):
    """Test that embedding the same text multiple times produces consistent results."""

    text = "This is a test sentence."
    embeddings1 = await provider.embed([text])
    embeddings2 = await provider.embed([text])

    assert len(embeddings1) == len(embeddings2)
    assert len(embeddings1[0]) == len(embeddings2[0])
    assert all(abs(x - y) < 1e-3 for x, y in zip(embeddings1[0], embeddings2[0]))


@pytest.mark.asyncio
async def test_embed_error_handling(provider):
    """Test error handling for invalid inputs."""

    # Test with None
    with pytest.raises(Exception):
        await provider.embed([None])  # type: ignore

    # Test with empty string
    embeddings = await provider.embed([""])
    assert len(embeddings) == 0


@pytest.mark.asyncio
async def test_embed_order_consistency_with_many_tasks(mock_provider):
    """Test that embeddings maintain correct order even with many parallel tasks."""
    # Create a large number of unique test strings (much more than OPENAI_NUM_PARALLEL_TASKS)
    num_strings = 50  # Significantly more than the parallel task limit

    # Create strings with very distinct patterns that will produce different embeddings
    # and make it easy to verify order. Make them long enough to force batching.
    test_strings = []
    for i in range(num_strings):
        # Create a string with a very distinct pattern that will produce a unique embedding
        # Using a pattern that will be very different for each index
        # Make it long enough to force batching (about 1000 tokens)
        test_strings.append(f"STRING_{i}_" + "A" * 1000 + "_" + "B" * 1000)

    # Track the order of requests to verify batching
    request_order = []

    # Mock the OpenAI API response with random delays
    async def mock_create(*args, **kwargs):
        import random
        import asyncio

        # Get the input batch
        input_texts = kwargs.get("input", [])

        # Record the order of this batch
        batch_indices = [int(text.split("_")[1]) for text in input_texts]
        request_order.append(batch_indices)

        # Add a random delay for this batch
        await asyncio.sleep(random.uniform(0.1, 0.5))

        mock_response = AsyncMock()
        mock_response.data = []

        # Process each text in the batch
        for text in input_texts:
            # Extract the index from the input text (e.g., "STRING_5_..." -> 5)
            index = int(text.split("_")[1])
            mock_embedding = AsyncMock()
            mock_embedding.embedding = [
                float(index)
            ] * 10  # Use the index as the embedding value
            mock_response.data.append(mock_embedding)

        return mock_response

    # Set up the mock response
    mock_provider.openai_client.embeddings.create = AsyncMock(side_effect=mock_create)

    # Get embeddings
    embeddings = await mock_provider.embed(test_strings)

    # Verify we got the correct number of embeddings
    assert len(embeddings) == num_strings

    # Verify each embedding is valid
    assert all(isinstance(emb, list) for emb in embeddings)
    assert all(isinstance(x, float) for emb in embeddings for x in emb)

    # Verify that the embeddings are in the correct order
    # Each embedding should be a list of the same number (the index)
    for i, emb in enumerate(embeddings):
        assert all(x == float(i) for x in emb), (
            f"Embedding at position {i} does not match expected value {i}"
        )

    # Print the request order to help debug
    print("\nRequest order:")
    for i, batch in enumerate(request_order):
        print(f"Batch {i}: {batch}")
