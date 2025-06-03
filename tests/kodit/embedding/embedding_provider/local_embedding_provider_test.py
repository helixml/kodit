"""Tests for the local embedding provider."""

import pytest
from sentence_transformers import SentenceTransformer

from kodit.embedding.embedding_provider.local_embedding_provider import (
    LocalEmbeddingProvider,
    TINY,
)


@pytest.fixture
def provider():
    """Create a LocalEmbeddingProvider instance with the tiny model."""
    return LocalEmbeddingProvider(TINY)


@pytest.mark.asyncio
async def test_model_lazy_loading(provider):
    """Test that the model is loaded lazily."""
    assert provider.embedding_model is None
    model = provider._model()
    assert isinstance(model, SentenceTransformer)
    assert provider.embedding_model is not None


@pytest.mark.asyncio
async def test_embed_single_text(provider):
    """Test embedding a single text."""
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
    assert all(abs(x - y) < 1e-6 for x, y in zip(embeddings1[0], embeddings2[0]))
