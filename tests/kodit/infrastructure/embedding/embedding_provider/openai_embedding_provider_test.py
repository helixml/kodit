"""Tests for the OpenAI embedding provider."""

from typing import Any
from unittest.mock import AsyncMock, MagicMock

import pytest

from kodit.domain.value_objects import EmbeddingRequest
from kodit.infrastructure.embedding.embedding_providers.openai_embedding_provider import (  # noqa: E501
    OpenAIEmbeddingProvider,
)


class TestOpenAIEmbeddingProvider:
    """Test the OpenAI embedding provider."""

    def test_init_default_values(self) -> None:
        """Test initialization with default values."""
        mock_client = MagicMock()
        provider = OpenAIEmbeddingProvider(openai_client=mock_client)
        assert provider.openai_client == mock_client
        assert provider.model_name == "text-embedding-3-small"
        assert provider.log is not None

    def test_init_custom_values(self) -> None:
        """Test initialization with custom values."""
        mock_client = MagicMock()
        provider = OpenAIEmbeddingProvider(
            openai_client=mock_client, model_name="text-embedding-3-large"
        )
        assert provider.openai_client == mock_client
        assert provider.model_name == "text-embedding-3-large"

    @pytest.mark.asyncio
    async def test_embed_empty_requests(self) -> None:
        """Test embedding with empty requests."""
        mock_client = MagicMock()
        provider = OpenAIEmbeddingProvider(openai_client=mock_client)

        results = []
        async for batch in provider.embed([]):
            results.extend(batch)

        assert len(results) == 0
        mock_client.embeddings.create.assert_not_called()

    @pytest.mark.asyncio
    async def test_embed_single_request_success(self) -> None:
        """Test successful embedding with a single request."""
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.data = [MagicMock()]
        mock_response.data[0].embedding = [0.1, 0.2, 0.3, 0.4, 0.5] * 300  # 1500 dims
        mock_client.embeddings.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEmbeddingProvider(openai_client=mock_client)
        requests = [EmbeddingRequest(snippet_id=1, text="python programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert len(results[0].embedding) == 1500
        assert all(isinstance(v, float) for v in results[0].embedding)

        # Verify API was called correctly
        mock_client.embeddings.create.assert_called_once_with(
            input=["python programming"], model="text-embedding-3-small"
        )

    @pytest.mark.asyncio
    async def test_embed_multiple_requests_success(self) -> None:
        """Test successful embedding with multiple requests."""
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.data = [
            MagicMock(embedding=[0.1, 0.2, 0.3] * 500),  # 1500 dims
            MagicMock(embedding=[0.4, 0.5, 0.6] * 500),  # 1500 dims
            MagicMock(embedding=[0.7, 0.8, 0.9] * 500),  # 1500 dims
        ]
        mock_client.embeddings.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEmbeddingProvider(openai_client=mock_client)
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
            assert len(result.embedding) == 1500
            assert all(isinstance(v, float) for v in result.embedding)

        # Verify API was called correctly
        mock_client.embeddings.create.assert_called_once_with(
            input=["python programming", "javascript development", "java enterprise"],
            model="text-embedding-3-small",
        )

    @pytest.mark.asyncio
    async def test_embed_batch_processing(self) -> None:
        """Test that requests are processed in batches."""
        mock_client = MagicMock()

        # Dynamic mock that returns embeddings matching input size
        async def mock_create(**kwargs: Any) -> MagicMock:
            input_size = len(kwargs["input"])
            mock_response = MagicMock()
            mock_response.data = [
                MagicMock(embedding=[0.1] * 1500) for _ in range(input_size)
            ]
            return mock_response

        mock_client.embeddings.create = AsyncMock(side_effect=mock_create)

        provider = OpenAIEmbeddingProvider(openai_client=mock_client)
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
        assert mock_client.embeddings.create.call_count == 2

    @pytest.mark.asyncio
    async def test_embed_api_error_handling(self) -> None:
        """Test handling of API errors."""
        mock_client = MagicMock()
        mock_client.embeddings.create = AsyncMock(side_effect=Exception("API Error"))

        provider = OpenAIEmbeddingProvider(openai_client=mock_client)
        requests = [EmbeddingRequest(snippet_id=1, text="python programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Should return no embeddings on error (empty batch)
        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_embed_custom_model(self) -> None:
        """Test embedding with a custom model."""
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=[0.1, 0.2, 0.3] * 500)]
        mock_client.embeddings.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEmbeddingProvider(
            openai_client=mock_client, model_name="text-embedding-3-large"
        )
        requests = [EmbeddingRequest(snippet_id=1, text="test text")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Verify the custom model was used
        mock_client.embeddings.create.assert_called_once_with(
            input=["test text"], model="text-embedding-3-large"
        )

    @pytest.mark.asyncio
    async def test_embed_empty_text(self) -> None:
        """Test embedding with empty text."""
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=[0.1] * 1500)]
        mock_client.embeddings.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEmbeddingProvider(openai_client=mock_client)
        requests = [EmbeddingRequest(snippet_id=1, text="")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert len(results[0].embedding) == 1500
        mock_client.embeddings.create.assert_called_once_with(
            input=[""], model="text-embedding-3-small"
        )

    @pytest.mark.asyncio
    async def test_embed_unicode_text(self) -> None:
        """Test embedding with unicode text."""
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=[0.1] * 1500)]
        mock_client.embeddings.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEmbeddingProvider(openai_client=mock_client)
        requests = [EmbeddingRequest(snippet_id=1, text="python 🐍 programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert len(results[0].embedding) == 1500
        mock_client.embeddings.create.assert_called_once_with(
            input=["python 🐍 programming"], model="text-embedding-3-small"
        )

    @pytest.mark.asyncio
    async def test_embed_large_batch_error_handling(self) -> None:
        """Test error handling with large batches."""
        mock_client = MagicMock()
        mock_client.embeddings.create = AsyncMock(side_effect=Exception("Batch Error"))

        provider = OpenAIEmbeddingProvider(openai_client=mock_client)
        requests = [EmbeddingRequest(snippet_id=i, text=f"text {i}") for i in range(5)]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Should return no embeddings for all requests on error
        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_embed_response_structure_validation(self) -> None:
        """Test validation of API response structure."""
        mock_client = MagicMock()

        # Dynamic mock that returns malformed response matching input size
        async def mock_create(**kwargs: Any) -> MagicMock:
            input_size = len(kwargs["input"])
            mock_response = MagicMock()
            mock_response.data = [MagicMock() for _ in range(input_size)]
            # Missing embedding attribute on all items
            for item in mock_response.data:
                del item.embedding
            return mock_response

        mock_client.embeddings.create = AsyncMock(side_effect=mock_create)

        provider = OpenAIEmbeddingProvider(openai_client=mock_client)
        requests = [EmbeddingRequest(snippet_id=1, text="test")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Should handle malformed response gracefully by returning empty results
        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_non_openai_model_name(self) -> None:
        """Test embedding with a non-OpenAI model name."""
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=[0.1] * 1500)]
        mock_client.embeddings.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEmbeddingProvider(
            openai_client=mock_client, model_name="non-openai-model"
        )

        # This should not crash
        test_requests = [EmbeddingRequest(snippet_id=1, text="test")]
        await anext(provider.embed(test_requests))

        # Verify the custom model was used
        mock_client.embeddings.create.assert_called_once_with(
            input=["test"], model="non-openai-model"
        )
