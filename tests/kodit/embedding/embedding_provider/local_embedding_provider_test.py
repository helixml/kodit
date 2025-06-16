"""Tests for the local embedding provider."""

import pytest
from sentence_transformers import SentenceTransformer
import tiktoken

# Helper imports
from kodit.embedding.embedding_provider.embedding_provider import (
    EmbeddingRequest,
    split_sub_batches,
)
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
    embeddings = await collect_embeddings(provider, [EmbeddingRequest(id=0, text=text)])

    assert len(embeddings) == 1
    assert isinstance(embeddings[0], list)
    assert all(isinstance(x, float) for x in embeddings[0])


@pytest.mark.asyncio
async def test_embed_multiple_texts(provider):
    """Test embedding multiple texts."""
    texts = ["First test sentence.", "Second test sentence.", "Third test sentence."]
    embeddings = await collect_embeddings(
        provider, [EmbeddingRequest(id=i, text=text) for i, text in enumerate(texts)]
    )

    assert len(embeddings) == 3
    assert all(isinstance(emb, list) for emb in embeddings)
    assert all(isinstance(x, float) for emb in embeddings for x in emb)


@pytest.mark.asyncio
async def test_embed_empty_list(provider):
    """Test embedding an empty list."""
    embeddings = await collect_embeddings(provider, [EmbeddingRequest(id=0, text="")])
    assert len(embeddings) == 0


@pytest.mark.asyncio
async def test_embed_large_text(provider):
    """Test embedding a large text that might need batching."""
    # Create a large text that exceeds typical token limits
    large_text = "This is a test sentence. " * 1000
    embeddings = await collect_embeddings(
        provider, [EmbeddingRequest(id=0, text=large_text)]
    )

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
    embeddings = await collect_embeddings(
        provider, [EmbeddingRequest(id=i, text=text) for i, text in enumerate(texts)]
    )

    assert len(embeddings) == 4
    assert all(isinstance(emb, list) for emb in embeddings)
    assert all(isinstance(x, float) for emb in embeddings for x in emb)


@pytest.mark.asyncio
async def test_embed_consistency(provider):
    """Test that embedding the same text multiple times produces consistent results."""
    text = "This is a test sentence."
    response1 = await anext(provider.embed([EmbeddingRequest(id=0, text=text)]))
    response2 = await anext(provider.embed([EmbeddingRequest(id=0, text=text)]))

    assert len(response1) == len(response2)
    # Extract embeddings and compare
    embeddings1 = response1[0].embedding
    embeddings2 = response2[0].embedding
    assert all(abs(x - y) < 1e-6 for x, y in zip(embeddings1, embeddings2))


@pytest.mark.asyncio
async def test_split_sub_batches(provider):
    """Test that the embedding provider batches the text correctly."""
    encoding = tiktoken.encoding_for_model("text-embedding-3-small")
    # Should not crash
    split_sub_batches(
        encoding,
        [EmbeddingRequest(id=0, text="This is a test sentence. <|endoftext|>")],
    )


@pytest.mark.asyncio
async def test_split_sub_batches_performance_test(provider):
    """Test that the embedding provider batches the text correctly."""
    encoding = tiktoken.encoding_for_model("text-embedding-3-small")
    requests = [
        EmbeddingRequest(id=i, text="This is a test sentence. <|endoftext|>" * 1000)
        for i in range(1)
    ]
    split_sub_batches(encoding, requests)


# Utility to gather embeddings from the async generator returned by the provider.
async def collect_embeddings(provider, requests: list[EmbeddingRequest]):
    """Collect embeddings from the provider while preserving request order."""
    # Map id -> embedding
    embeddings_map: dict[int, list[float]] = {}
    async for batch in provider.embed(requests):
        for resp in batch:
            embeddings_map[resp.id] = resp.embedding

    # Return in order of request ids
    return [embeddings_map[idx] for idx in sorted(embeddings_map.keys())]
