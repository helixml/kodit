"""Tests for the OpenAI embedding provider."""

import os
import pytest
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
def provider(openai_client):
    """Create an OpenAIEmbeddingProvider instance."""
    return OpenAIEmbeddingProvider(openai_client)


@pytest.mark.asyncio
async def test_initialization(openai_client):
    """Test that the provider initializes correctly."""
    skip_if_no_api_key()

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
    skip_if_no_api_key()

    texts = ["First test sentence.", "Second test sentence.", "Third test sentence."]
    embeddings = await provider.embed(texts)

    assert len(embeddings) == 3
    assert all(isinstance(emb, list) for emb in embeddings)
    assert all(isinstance(x, float) for emb in embeddings for x in emb)


@pytest.mark.asyncio
async def test_embed_empty_list(provider):
    """Test embedding an empty list."""
    skip_if_no_api_key()

    embeddings = await provider.embed([])
    assert len(embeddings) == 0


@pytest.mark.asyncio
async def test_embed_large_text(provider):
    """Test embedding a large text that might need batching."""
    skip_if_no_api_key()

    # Create a large text that exceeds typical token limits
    large_text = "This is a test sentence. " * 1000
    embeddings = await provider.embed([large_text])

    assert len(embeddings) == 1
    assert isinstance(embeddings[0], list)
    assert all(isinstance(x, float) for x in embeddings[0])


@pytest.mark.asyncio
async def test_embed_special_characters(provider):
    """Test embedding text with special characters."""
    skip_if_no_api_key()

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
    skip_if_no_api_key()

    text = "This is a test sentence."
    embeddings1 = await provider.embed([text])
    embeddings2 = await provider.embed([text])

    assert len(embeddings1) == len(embeddings2)
    assert len(embeddings1[0]) == len(embeddings2[0])
    assert all(abs(x - y) < 1e-3 for x, y in zip(embeddings1[0], embeddings2[0]))


@pytest.mark.asyncio
async def test_embed_error_handling(provider):
    """Test error handling for invalid inputs."""
    skip_if_no_api_key()

    # Test with None
    with pytest.raises(Exception):
        await provider.embed([None])  # type: ignore

    # Test with empty string
    embeddings = await provider.embed([""])
    assert len(embeddings) == 0
