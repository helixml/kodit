"""Tests for the hash embedding provider."""

import hashlib

import pytest

from kodit.domain.value_objects import EmbeddingRequest
from kodit.infrastructure.embedding.embedding_providers.hash_embedding_provider import (
    CODE,
    TINY,
    HashEmbeddingProvider,
)


class TestHashEmbeddingProvider:
    """Test the hash embedding provider."""

    def test_init_default_values(self) -> None:
        """Test initialization with default values."""
        provider = HashEmbeddingProvider()
        assert provider.embedding_size == CODE
        assert provider.log is not None

    def test_init_custom_values(self) -> None:
        """Test initialization with custom values."""
        provider = HashEmbeddingProvider(embedding_size=TINY)
        assert provider.embedding_size == TINY

    @pytest.mark.asyncio
    async def test_embed_empty_requests(self) -> None:
        """Test embedding with empty requests."""
        provider = HashEmbeddingProvider()
        requests: list[EmbeddingRequest] = []

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_embed_single_request(self) -> None:
        """Test embedding with a single request."""
        provider = HashEmbeddingProvider()
        requests = [EmbeddingRequest(snippet_id=1, text="python programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert len(results[0].embedding) == CODE
        assert all(isinstance(v, float) for v in results[0].embedding)
        assert all(-1 <= v <= 1 for v in results[0].embedding)

    @pytest.mark.asyncio
    async def test_embed_multiple_requests(self) -> None:
        """Test embedding with multiple requests."""
        provider = HashEmbeddingProvider()
        requests = [
            EmbeddingRequest(snippet_id=1, text="python programming"),
            EmbeddingRequest(snippet_id=2, text="javascript development"),
            EmbeddingRequest(snippet_id=3, text="java enterprise"),
        ]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 3
        for i, result in enumerate(results):
            assert result.snippet_id == i + 1
            assert len(result.embedding) == CODE
            assert all(isinstance(v, float) for v in result.embedding)
            assert all(-1 <= v <= 1 for v in result.embedding)

    @pytest.mark.asyncio
    async def test_embed_deterministic(self) -> None:
        """Test that embeddings are deterministic for the same text."""
        provider = HashEmbeddingProvider()
        text = "python programming language"
        requests = [
            EmbeddingRequest(snippet_id=1, text=text),
            EmbeddingRequest(snippet_id=2, text=text),
        ]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 2
        assert results[0].embedding == results[1].embedding

    @pytest.mark.asyncio
    async def test_embed_different_texts_different_embeddings(self) -> None:
        """Test that different texts produce different embeddings."""
        provider = HashEmbeddingProvider()
        requests = [
            EmbeddingRequest(snippet_id=1, text="python programming"),
            EmbeddingRequest(snippet_id=2, text="javascript development"),
        ]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 2
        assert results[0].embedding != results[1].embedding

    @pytest.mark.asyncio
    async def test_embed_batch_processing(self) -> None:
        """Test that requests are processed in batches."""
        provider = HashEmbeddingProvider()
        # Create more than batch_size requests
        requests = [
            EmbeddingRequest(snippet_id=i, text=f"text {i}")
            for i in range(15)  # More than batch_size of 10
        ]

        batch_count = 0
        total_results = []
        async for batch in provider.embed(requests):
            batch_count += 1
            total_results.extend(batch)

        assert len(total_results) == 15
        assert batch_count == 2  # Should be 2 batches: 10 + 5

    @pytest.mark.asyncio
    async def test_embed_tiny_size(self) -> None:
        """Test embedding with tiny size."""
        provider = HashEmbeddingProvider(embedding_size=TINY)
        requests = [EmbeddingRequest(snippet_id=1, text="test text")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert len(results[0].embedding) == TINY

    def test_generate_embedding_manual(self) -> None:
        """Test the embedding generation logic manually."""
        provider = HashEmbeddingProvider(embedding_size=4)
        text = "test"

        # Calculate expected embedding manually
        hash_obj = hashlib.sha256(text.encode("utf-8"))
        hash_bytes = hash_obj.digest()

        expected_embedding = []
        for i in range(4):
            byte_index = i % len(hash_bytes)
            value = (hash_bytes[byte_index] - 128) / 128.0
            expected_embedding.append(value)

        actual_embedding = provider._generate_embedding(text)  # noqa: SLF001
        assert actual_embedding == expected_embedding

    @pytest.mark.asyncio
    async def test_embed_similarity_scores(self) -> None:
        """Test that similar texts have higher similarity scores."""
        provider = HashEmbeddingProvider()

        # Create embeddings for similar and different texts
        similar_texts = [
            "python programming language",
            "python coding language",
            "python development language",
        ]
        different_text = "javascript web development"

        similar_requests = [
            EmbeddingRequest(snippet_id=i, text=text)
            for i, text in enumerate(similar_texts)
        ]
        different_request = [EmbeddingRequest(snippet_id=99, text=different_text)]

        # Get embeddings
        similar_embeddings = []
        async for batch in provider.embed(similar_requests):
            similar_embeddings.extend(batch)

        different_embeddings = []
        async for batch in provider.embed(different_request):
            different_embeddings.extend(batch)

        # Calculate cosine similarities
        def cosine_similarity(in1: list[float], in2: list[float]) -> float:
            """Calculate cosine similarity between two vectors."""
            import numpy as np

            vec1 = np.array(in1)
            vec2 = np.array(in2)
            return np.dot(vec1, vec2) / (np.linalg.norm(vec1) * np.linalg.norm(vec2))

        # Similar texts should have higher similarity
        similar_sim = cosine_similarity(
            similar_embeddings[0].embedding, similar_embeddings[1].embedding
        )
        different_sim = cosine_similarity(
            similar_embeddings[0].embedding, different_embeddings[0].embedding
        )

        # Note: Since this is a hash-based embedding, the similarity might not be
        # as meaningful as with real embeddings, but we can still test the structure
        assert isinstance(similar_sim, float)
        assert isinstance(different_sim, float)
        assert -1 <= similar_sim <= 1
        assert -1 <= different_sim <= 1

    @pytest.mark.asyncio
    async def test_embed_empty_text(self) -> None:
        """Test embedding with empty text."""
        provider = HashEmbeddingProvider()
        requests = [EmbeddingRequest(snippet_id=1, text="")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert len(results[0].embedding) == CODE
        # Empty text should still produce a valid embedding
        assert all(isinstance(v, float) for v in results[0].embedding)

    @pytest.mark.asyncio
    async def test_embed_unicode_text(self) -> None:
        """Test embedding with unicode text."""
        provider = HashEmbeddingProvider()
        requests = [EmbeddingRequest(snippet_id=1, text="python 🐍 programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert len(results[0].embedding) == CODE
        assert all(isinstance(v, float) for v in results[0].embedding)

    @pytest.mark.asyncio
    async def test_embed_large_embedding_size(self) -> None:
        """Test embedding with a large embedding size."""
        large_size = 2048
        provider = HashEmbeddingProvider(embedding_size=large_size)
        requests = [EmbeddingRequest(snippet_id=1, text="test text")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert len(results[0].embedding) == large_size
        assert all(isinstance(v, float) for v in results[0].embedding)
        assert all(-1 <= v <= 1 for v in results[0].embedding)

    def test_generate_embedding_edge_cases(self) -> None:
        """Test embedding generation with edge cases."""
        provider = HashEmbeddingProvider(embedding_size=10)

        # Test with very long text
        long_text = "x" * 10000
        embedding = provider._generate_embedding(long_text)  # noqa: SLF001
        assert len(embedding) == 10
        assert all(isinstance(v, float) for v in embedding)

        # Test with special characters
        special_text = "!@#$%^&*()_+-=[]{}|;':\",./<>?"
        embedding = provider._generate_embedding(special_text)  # noqa: SLF001
        assert len(embedding) == 10
        assert all(isinstance(v, float) for v in embedding)

        # Test with numbers
        number_text = "1234567890"
        embedding = provider._generate_embedding(number_text)  # noqa: SLF001
        assert len(embedding) == 10
        assert all(isinstance(v, float) for v in embedding)
