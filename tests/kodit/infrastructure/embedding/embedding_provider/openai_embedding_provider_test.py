"""Tests for the OpenAI embedding provider."""

from typing import Any
from unittest.mock import AsyncMock, Mock

import httpx
import pytest

from kodit.domain.value_objects import EmbeddingRequest
from kodit.infrastructure.embedding.embedding_providers.openai_embedding_provider import (  # noqa: E501
    OpenAIEmbeddingProvider,
)


class TestOpenAIEmbeddingProvider:
    """Test the OpenAI embedding provider."""

    def test_init_default_values(self) -> None:
        """Test initialization with default values."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")
        assert provider.model_name == "text-embedding-3-small"
        assert provider.api_key == "test-key"
        assert provider.base_url == "https://api.openai.com"
        assert provider.socket_path is None
        assert provider.log is not None

    def test_init_custom_values(self) -> None:
        """Test initialization with custom values."""
        provider = OpenAIEmbeddingProvider(
            api_key="test-key",
            base_url="https://custom.openai.com",
            model_name="text-embedding-3-large",
            socket_path="/tmp/socket.sock",  # noqa: S108
        )
        assert provider.model_name == "text-embedding-3-large"
        assert provider.base_url == "https://custom.openai.com"
        assert provider.socket_path == "/tmp/socket.sock"  # noqa: S108

    def test_init_with_timeout(self) -> None:
        """Test initialization with custom timeout."""
        provider = OpenAIEmbeddingProvider(api_key="test-key", timeout=60.0)
        assert provider.timeout == 60.0

        # Test default timeout
        provider_default = OpenAIEmbeddingProvider(api_key="test-key")
        assert provider_default.timeout == 30.0

    @pytest.mark.asyncio
    async def test_embed_empty_requests(self) -> None:
        """Test embedding with empty requests."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")

        results = []
        async for batch in provider.embed([]):
            results.extend(batch)

        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_embed_single_request_success(self) -> None:
        """Test successful embedding with a single request."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")

        # Mock the httpx client
        mock_response = Mock()
        mock_response.json.return_value = {
            "data": [
                {"embedding": [0.1, 0.2, 0.3, 0.4, 0.5] * 300}  # 1500 dims
            ]
        }
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
        requests = [EmbeddingRequest(snippet_id=1, text="python programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert len(results[0].embedding) == 1500
        assert all(isinstance(v, float) for v in results[0].embedding)

        # Verify API was called correctly
        provider.http_client.post.assert_called_once_with(
            "/v1/embeddings",
            json={"model": "text-embedding-3-small", "input": ["python programming"]},
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer test-key",
            },
        )

    @pytest.mark.asyncio
    async def test_embed_multiple_requests_success(self) -> None:
        """Test successful embedding with multiple requests."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")

        # Mock the httpx client
        mock_response = Mock()
        mock_response.json.return_value = {
            "data": [
                {"embedding": [0.1, 0.2, 0.3] * 500},  # 1500 dims
                {"embedding": [0.4, 0.5, 0.6] * 500},  # 1500 dims
                {"embedding": [0.7, 0.8, 0.9] * 500},  # 1500 dims
            ]
        }
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
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
        provider.http_client.post.assert_called_once_with(
            "/v1/embeddings",
            json={
                "model": "text-embedding-3-small",
                "input": [
                    "python programming",
                    "javascript development",
                    "java enterprise",
                ],
            },
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer test-key",
            },
        )

    @pytest.mark.asyncio
    async def test_embed_batch_processing(self) -> None:
        """Test that requests are processed in batches."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")

        # Dynamic mock that returns embeddings matching input size
        async def mock_post(url: str, **kwargs: Any) -> httpx.Response:  # noqa: ARG001
            input_size = len(kwargs["json"]["input"])
            mock_response = Mock()
            mock_response.json.return_value = {
                "data": [{"embedding": [0.1] * 1500} for _ in range(input_size)]
            }
            mock_response.raise_for_status = Mock()
            return mock_response

        mock_client = Mock()
        mock_client.post = AsyncMock(side_effect=mock_post)
        provider.http_client = mock_client
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
        assert provider.http_client.post.call_count == 2

    @pytest.mark.asyncio
    async def test_embed_api_error_handling(self) -> None:
        """Test handling of API errors."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")
        mock_client = Mock()
        mock_client.post = AsyncMock(side_effect=Exception("API Error"))
        provider.http_client = mock_client
        requests = [EmbeddingRequest(snippet_id=1, text="python programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Should return no embeddings on error (empty batch)
        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_embed_custom_model(self) -> None:
        """Test embedding with a custom model."""
        provider = OpenAIEmbeddingProvider(
            api_key="test-key", model_name="text-embedding-3-large"
        )

        mock_response = Mock()
        mock_response.json.return_value = {
            "data": [{"embedding": [0.1, 0.2, 0.3] * 500}]
        }
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
        requests = [EmbeddingRequest(snippet_id=1, text="test text")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Verify the custom model was used
        provider.http_client.post.assert_called_once_with(
            "/v1/embeddings",
            json={"model": "text-embedding-3-large", "input": ["test text"]},
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer test-key",
            },
        )

    @pytest.mark.asyncio
    async def test_embed_empty_text(self) -> None:
        """Test embedding with empty text."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")

        mock_response = Mock()
        mock_response.json.return_value = {"data": [{"embedding": [0.1] * 1500}]}
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
        requests = [EmbeddingRequest(snippet_id=1, text="")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert len(results[0].embedding) == 1500
        provider.http_client.post.assert_called_once()

    @pytest.mark.asyncio
    async def test_embed_unicode_text(self) -> None:
        """Test embedding with unicode text."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")

        mock_response = Mock()
        mock_response.json.return_value = {"data": [{"embedding": [0.1] * 1500}]}
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
        requests = [EmbeddingRequest(snippet_id=1, text="python 🐍 programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert len(results[0].embedding) == 1500

        # Verify unicode text was sent correctly
        provider.http_client.post.assert_called_once_with(
            "/v1/embeddings",
            json={
                "model": "text-embedding-3-small",
                "input": ["python 🐍 programming"],
            },
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer test-key",
            },
        )

    @pytest.mark.asyncio
    async def test_embed_large_batch_error_handling(self) -> None:
        """Test error handling with large batches."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")
        mock_client = Mock()
        mock_client.post = AsyncMock(side_effect=Exception("Batch Error"))
        provider.http_client = mock_client
        requests = [EmbeddingRequest(snippet_id=i, text=f"text {i}") for i in range(5)]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Should return no embeddings for all requests on error
        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_embed_response_structure_validation(self) -> None:
        """Test validation of API response structure."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")

        # Mock response with missing embedding data
        mock_response = Mock()
        mock_response.json.return_value = {
            "data": [{}]  # Missing embedding field
        }
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
        requests = [EmbeddingRequest(snippet_id=1, text="test")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Should handle malformed response gracefully
        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].embedding == []  # Empty embedding

    @pytest.mark.asyncio
    async def test_non_openai_model_name(self) -> None:
        """Test embedding with a non-OpenAI model name."""
        provider = OpenAIEmbeddingProvider(
            api_key="test-key", model_name="non-openai-model"
        )

        mock_response = Mock()
        mock_response.json.return_value = {"data": [{"embedding": [0.1] * 1500}]}
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
        # This should not crash
        test_requests = [EmbeddingRequest(snippet_id=1, text="test")]
        await anext(provider.embed(test_requests))

        # Verify the custom model was used
        provider.http_client.post.assert_called_once_with(
            "/v1/embeddings",
            json={"model": "non-openai-model", "input": ["test"]},
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer test-key",
            },
        )

    @pytest.mark.asyncio
    async def test_close(self) -> None:
        """Test close method."""
        provider = OpenAIEmbeddingProvider(api_key="test-key")
        mock_client = Mock()
        mock_client.aclose = AsyncMock()
        provider.http_client = mock_client

        await provider.close()

        provider.http_client.aclose.assert_called_once()
