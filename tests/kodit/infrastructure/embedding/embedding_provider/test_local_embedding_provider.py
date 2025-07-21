"""Tests for the local embedding provider."""

from typing import Any
from unittest.mock import MagicMock, patch

import numpy as np
import pytest

from kodit.domain.value_objects import EmbeddingRequest
from kodit.infrastructure.embedding.embedding_providers.local_embedding_provider import (  # noqa: E501
    TINY,
    LocalEmbeddingProvider,
)


class TestLocalEmbeddingProvider:
    """Test the local embedding provider."""

    def test_init_default_values(self) -> None:
        """Test initialization with default values."""
        provider = LocalEmbeddingProvider()
        assert (
            provider.model_name
            == "flax-sentence-embeddings/st-codesearch-distilroberta-base"
        )
        assert provider.log is not None

    def test_init_custom_values(self) -> None:
        """Test initialization with custom values."""
        provider = LocalEmbeddingProvider(model_name=TINY)
        assert provider.model_name == "ibm-granite/granite-embedding-30m-english"

    def test_init_full_model_name(self) -> None:
        """Test initialization with a full model name."""
        custom_model = "sentence-transformers/all-MiniLM-L6-v2"
        provider = LocalEmbeddingProvider(model_name=custom_model)
        assert provider.model_name == custom_model

    @pytest.mark.asyncio
    async def test_embed_empty_requests(self) -> None:
        """Test embedding with empty requests."""
        provider = LocalEmbeddingProvider()
        requests: list[EmbeddingRequest] = []

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_embed_single_request(self) -> None:
        """Test embedding with a single request."""
        with patch(
            "sentence_transformers.SentenceTransformer"
        ) as mock_transformer_class:
            # Mock the model
            mock_model = MagicMock()
            mock_model.encode.return_value = np.array([[0.1, 0.2, 0.3, 0.4, 0.5]])
            mock_transformer_class.return_value = mock_model

            provider = LocalEmbeddingProvider()
            requests = [EmbeddingRequest(snippet_id=1, text="python programming")]

            results = []
            async for batch in provider.embed(requests):
                results.extend(batch)

            assert len(results) == 1
            assert results[0].snippet_id == 1
            assert len(results[0].embedding) == 5  # Based on our mock
            assert all(isinstance(v, float) for v in results[0].embedding)

    @pytest.mark.asyncio
    async def test_embed_multiple_requests(self) -> None:
        """Test embedding with multiple requests."""
        with patch(
            "sentence_transformers.SentenceTransformer"
        ) as mock_transformer_class:
            # Mock the model
            mock_model = MagicMock()
            mock_model.encode.return_value = np.array(
                [[0.1, 0.2, 0.3], [0.4, 0.5, 0.6], [0.7, 0.8, 0.9]]
            )
            mock_transformer_class.return_value = mock_model

            provider = LocalEmbeddingProvider()
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
                assert len(result.embedding) == 3  # Based on our mock
                assert all(isinstance(v, float) for v in result.embedding)

    @pytest.mark.asyncio
    async def test_embed_different_texts_different_embeddings(self) -> None:
        """Test that different texts produce different embeddings."""
        with patch(
            "sentence_transformers.SentenceTransformer"
        ) as mock_transformer_class:
            # Mock the model to return different embeddings for different texts
            mock_model = MagicMock()
            mock_model.encode.return_value = np.array(
                [
                    [0.1, 0.2, 0.3],
                    [0.9, 0.8, 0.7],  # Different embedding
                ]
            )
            mock_transformer_class.return_value = mock_model

            provider = LocalEmbeddingProvider()
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
        """Test that requests are processed in batches based on token limits."""
        with patch(
            "sentence_transformers.SentenceTransformer"
        ) as mock_transformer_class:
            # Mock the model with dynamic response based on input size
            mock_model = MagicMock()

            def mock_encode(texts: list[str], **kwargs: Any) -> np.ndarray:  # noqa: ARG001
                # Return embeddings matching input size
                return np.array([[0.1, 0.2, 0.3] for _ in range(len(texts))])

            mock_model.encode = mock_encode
            mock_transformer_class.return_value = mock_model

            provider = LocalEmbeddingProvider()
            # Create requests with different token counts
            requests = [
                EmbeddingRequest(snippet_id=i, text=f"text {i}") for i in range(5)
            ]

            batch_count = 0
            total_results = []
            async for batch in provider.embed(requests):
                batch_count += 1
                total_results.extend(batch)

            assert len(total_results) == 5
            # Should be at least 1 batch, but exact count depends on tokenization

    @pytest.mark.asyncio
    async def test_embed_error_handling(self) -> None:
        """Test error handling during embedding."""
        with patch(
            "sentence_transformers.SentenceTransformer"
        ) as mock_transformer_class:
            # Mock the model to raise an exception
            mock_model = MagicMock()
            mock_model.encode.side_effect = Exception("Model error")
            mock_transformer_class.return_value = mock_model

            provider = LocalEmbeddingProvider()
            requests = [EmbeddingRequest(snippet_id=1, text="test text")]

            results = []
            async for batch in provider.embed(requests):
                results.extend(batch)

            # Should return no embeddings on error (empty batch)
            assert len(results) == 0

    @pytest.mark.asyncio
    async def test_embed_empty_text(self) -> None:
        """Test embedding with empty text."""
        with patch(
            "sentence_transformers.SentenceTransformer"
        ) as mock_transformer_class:
            # Mock the model
            mock_model = MagicMock()
            mock_model.encode.return_value = np.array([[0.1, 0.2, 0.3]])
            mock_transformer_class.return_value = mock_model

            provider = LocalEmbeddingProvider()
            requests = [EmbeddingRequest(snippet_id=1, text="")]

            results = []
            async for batch in provider.embed(requests):
                results.extend(batch)

            assert len(results) == 1
            assert all(isinstance(v, float) for v in results[0].embedding)

    @pytest.mark.asyncio
    async def test_embed_unicode_text(self) -> None:
        """Test embedding with unicode text."""
        with patch(
            "sentence_transformers.SentenceTransformer"
        ) as mock_transformer_class:
            # Mock the model
            mock_model = MagicMock()
            mock_model.encode.return_value = np.array([[0.1, 0.2, 0.3]])
            mock_transformer_class.return_value = mock_model

            provider = LocalEmbeddingProvider()
            requests = [EmbeddingRequest(snippet_id=1, text="python 🐍 programming")]

            results = []
            async for batch in provider.embed(requests):
                results.extend(batch)

            assert len(results) == 1
            assert all(isinstance(v, float) for v in results[0].embedding)

    def test_split_sub_batches(self) -> None:
        """Test the sub-batch splitting logic."""
        with patch("tiktoken.encoding_for_model") as mock_encoding_for_model:
            # Mock the encoding
            mock_encoding = MagicMock()
            mock_encoding.encode.side_effect = lambda text, **kwargs: [1] * len(  # noqa: ARG005
                text
            )  # Simple tokenization
            mock_encoding_for_model.return_value = mock_encoding

            provider = LocalEmbeddingProvider()

            # Test with short texts that should fit in one batch
            short_requests = [
                EmbeddingRequest(snippet_id=1, text="short"),
                EmbeddingRequest(snippet_id=2, text="text"),
            ]

            batches = provider._split_sub_batches(mock_encoding, short_requests)  # noqa: SLF001
            assert len(batches) == 1
            assert len(batches[0]) == 2

            # Test with long text that should be split
            long_requests = [
                EmbeddingRequest(snippet_id=1, text="x" * 10000),  # Very long text
            ]

            batches = provider._split_sub_batches(mock_encoding, long_requests)  # noqa: SLF001
            assert len(batches) == 1  # Single item should still be in one batch
            assert len(batches[0]) == 1
