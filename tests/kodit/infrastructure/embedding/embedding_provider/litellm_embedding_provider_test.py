"""Tests for the LiteLLM embedding provider."""

from typing import Any
from unittest.mock import AsyncMock

import pytest

from kodit.config import Endpoint
from kodit.domain.value_objects import EmbeddingRequest
from kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider import (  # noqa: E501
    LiteLLMEmbeddingProvider,
)


class TestLiteLLMEmbeddingProvider:
    """Test the LiteLLM embedding provider."""

    def test_init_default_values(self) -> None:
        """Test initialization with default values."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)
        assert provider.endpoint.model is None
        assert provider.endpoint.api_key is None
        assert provider.endpoint.base_url is None
        assert provider.endpoint.timeout == 60
        assert provider.endpoint.extra_params is None
        assert provider.log is not None

    def test_init_custom_values(self) -> None:
        """Test initialization with custom values."""
        extra_params = {"temperature": 0.5}
        endpoint = Endpoint(
            model="text-embedding-3-large",
            api_key="test-api-key",
            base_url="https://custom.openai.com",
            timeout=60.0,
            extra_params=extra_params,
        )
        provider = LiteLLMEmbeddingProvider(endpoint)
        assert provider.endpoint.model == "text-embedding-3-large"
        assert provider.endpoint.api_key == "test-api-key"
        assert provider.endpoint.base_url == "https://custom.openai.com"
        assert provider.endpoint.timeout == 60.0
        assert provider.endpoint.extra_params == extra_params

    @pytest.mark.asyncio
    async def test_embed_empty_requests(self) -> None:
        """Test embedding with empty requests."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)

        results = []
        async for batch in provider.embed([]):
            results.extend(batch)

        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_embed_single_request_success(self) -> None:
        """Test successful embedding with a single request."""
        endpoint = Endpoint(model="text-embedding-3-small")
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock the provider's embedding method
        provider.provider.embedding = AsyncMock(  # type: ignore[method-assign]
            return_value={"data": [{"embedding": [0.1, 0.2, 0.3, 0.4, 0.5] * 300}]}
        )

        requests = [EmbeddingRequest(snippet_id="1", text="python programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert results[0].snippet_id == "1"
        assert len(results[0].embedding) == 1500
        assert all(isinstance(v, float) for v in results[0].embedding)

        # Verify the provider's embedding method was called correctly
        provider.provider.embedding.assert_called_once_with(["python programming"])

    @pytest.mark.asyncio
    async def test_embed_multiple_requests_success(self) -> None:
        """Test successful embedding with multiple requests."""
        endpoint = Endpoint(model="text-embedding-3-small")
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock the provider's embedding method
        provider.provider.embedding = AsyncMock(  # type: ignore[method-assign]
            return_value={
                "data": [
                    {"embedding": [0.1, 0.2, 0.3] * 500},  # 1500 dims
                    {"embedding": [0.4, 0.5, 0.6] * 500},  # 1500 dims
                    {"embedding": [0.7, 0.8, 0.9] * 500},  # 1500 dims
                ]
            }
        )

        requests = [
            EmbeddingRequest(snippet_id="1", text="python programming"),
            EmbeddingRequest(snippet_id="2", text="javascript development"),
            EmbeddingRequest(snippet_id="3", text="java enterprise"),
        ]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 3
        for i, result in enumerate(results):
            assert result.snippet_id == str(i + 1)
            assert len(result.embedding) == 1500
            assert all(isinstance(v, float) for v in result.embedding)

        # Verify the provider's embedding method was called correctly
        provider.provider.embedding.assert_called_once_with(
            [
                "python programming",
                "javascript development",
                "java enterprise",
            ]
        )

    @pytest.mark.asyncio
    async def test_embed_with_base_url(self) -> None:
        """Test embedding with custom base URL."""
        endpoint = Endpoint(
            model="text-embedding-3-small", base_url="https://custom.api.com"
        )
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock the provider's embedding method
        provider.provider.embedding = AsyncMock(  # type: ignore[method-assign]
            return_value={"data": [{"embedding": [0.1] * 1500}]}
        )

        requests = [EmbeddingRequest(snippet_id="1", text="test")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Verify the provider's embedding method was called
        provider.provider.embedding.assert_called_once_with(["test"])

    @pytest.mark.asyncio
    async def test_embed_with_api_key(self) -> None:
        """Test embedding with API key."""
        endpoint = Endpoint(model="text-embedding-3-small", api_key="sk-test-key-123")
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock the provider's embedding method
        provider.provider.embedding = AsyncMock(  # type: ignore[method-assign]
            return_value={"data": [{"embedding": [0.1] * 1500}]}
        )

        requests = [EmbeddingRequest(snippet_id="1", text="test")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Verify the provider's embedding method was called
        provider.provider.embedding.assert_called_once_with(["test"])

    @pytest.mark.asyncio
    async def test_embed_with_extra_params(self) -> None:
        """Test embedding with extra parameters."""
        extra_params = {"temperature": 0.5, "max_tokens": 100}
        endpoint = Endpoint(model="text-embedding-3-small", extra_params=extra_params)
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock the provider's embedding method
        provider.provider.embedding = AsyncMock(  # type: ignore[method-assign]
            return_value={"data": [{"embedding": [0.1] * 1500}]}
        )

        requests = [EmbeddingRequest(snippet_id="1", text="test")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Verify the provider's embedding method was called
        provider.provider.embedding.assert_called_once_with(["test"])

    @pytest.mark.asyncio
    async def test_embed_batch_processing(self) -> None:
        """Test that requests are processed in batches."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock responses for different batches
        async def mock_embedding_func(texts: list[str]) -> dict[str, Any]:
            return {
                "data": [{"embedding": [0.1] * 1500} for _ in range(len(texts))]
            }

        provider.provider.embedding = AsyncMock(side_effect=mock_embedding_func)  # type: ignore[method-assign]

        # Create more than batch_size requests (batch_size = 10)
        requests = [
            EmbeddingRequest(snippet_id=str(i), text=f"text {i}") for i in range(15)
        ]

        batch_count = 0
        total_results = []
        async for batch in provider.embed(requests):
            batch_count += 1
            total_results.extend(batch)

        assert len(total_results) == 15
        assert batch_count == 2  # Should be 2 batches: 10 + 5
        assert provider.provider.embedding.call_count == 2

    @pytest.mark.asyncio
    async def test_embed_api_error_handling(self) -> None:
        """Test handling of API errors."""
        endpoint = Endpoint(model="text-embedding-3-small")
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock the provider's embedding method to raise an error
        provider.provider.embedding = AsyncMock(  # type: ignore[method-assign]
            side_effect=Exception("LiteLLM API Error")
        )

        requests = [EmbeddingRequest(snippet_id="1", text="python programming")]

        # Should raise exception on error
        with pytest.raises(Exception, match="LiteLLM API Error"):
            async for _ in provider.embed(requests):
                pass

    @pytest.mark.asyncio
    async def test_embed_response_without_model_dump(self) -> None:
        """Test handling response without model_dump method."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock response that doesn't have model_dump method (dict response)
        provider.provider.embedding = AsyncMock(  # type: ignore[method-assign]
            return_value={"data": [{"embedding": [0.1] * 1500}]}
        )

        requests = [EmbeddingRequest(snippet_id="1", text="test")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert results[0].snippet_id == "1"
        assert len(results[0].embedding) == 1500

    @pytest.mark.asyncio
    async def test_embed_custom_model(self) -> None:
        """Test embedding with a custom model."""
        endpoint = Endpoint(model="claude-3-haiku-20240307")
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock the provider's embedding method
        provider.provider.embedding = AsyncMock(  # type: ignore[method-assign]
            return_value={"data": [{"embedding": [0.1] * 1500}]}
        )

        requests = [EmbeddingRequest(snippet_id="1", text="test text")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Verify the provider's embedding method was called
        provider.provider.embedding.assert_called_once_with(["test text"])

    @pytest.mark.asyncio
    async def test_socket_path_setup(self) -> None:
        """Test Unix socket setup."""
        endpoint = Endpoint(socket_path="/var/run/test.sock")
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Verify socket_path was stored
        assert provider.endpoint.socket_path == "/var/run/test.sock"

        # Should complete without error
        await provider.close()

    @pytest.mark.asyncio
    async def test_socket_path_httpx_client_setup(self) -> None:
        """Test that Unix socket creates proper HTTPX client."""
        endpoint = Endpoint(socket_path="/var/run/test.sock", timeout=60.0)
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Verify socket_path was passed to the provider
        assert provider.endpoint.socket_path == "/var/run/test.sock"
        assert provider.endpoint.timeout == 60.0

        await provider.close()

    @pytest.mark.asyncio
    async def test_close(self) -> None:
        """Test close method (should not raise any errors)."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)
        # Should complete without error
        await provider.close()
