"""Tests for the LiteLLM embedding provider."""

from typing import Any
from unittest.mock import AsyncMock, Mock, patch

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
        assert provider.model_name == "text-embedding-3-small"
        assert provider.api_key is None
        assert provider.base_url is None
        assert provider.timeout == 30.0
        assert provider.extra_params == {}
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
        assert provider.model_name == "text-embedding-3-large"
        assert provider.api_key == "test-api-key"
        assert provider.base_url == "https://custom.openai.com"
        assert provider.timeout == 60.0
        assert provider.extra_params == extra_params

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
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.aembedding"
    )
    async def test_embed_single_request_success(
        self, mock_aembedding: AsyncMock
    ) -> None:
        """Test successful embedding with a single request."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock LiteLLM response
        mock_response = Mock()
        mock_response.model_dump.return_value = {
            "data": [
                {"embedding": [0.1, 0.2, 0.3, 0.4, 0.5] * 300}  # 1500 dims
            ]
        }
        mock_aembedding.return_value = mock_response

        requests = [EmbeddingRequest(snippet_id=1, text="python programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert len(results[0].embedding) == 1500
        assert all(isinstance(v, float) for v in results[0].embedding)

        # Verify LiteLLM was called correctly
        mock_aembedding.assert_called_once_with(
            model="text-embedding-3-small",
            input=["python programming"],
            timeout=30.0,
        )

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.aembedding"
    )
    async def test_embed_multiple_requests_success(
        self, mock_aembedding: AsyncMock
    ) -> None:
        """Test successful embedding with multiple requests."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock LiteLLM response
        mock_response = Mock()
        mock_response.model_dump.return_value = {
            "data": [
                {"embedding": [0.1, 0.2, 0.3] * 500},  # 1500 dims
                {"embedding": [0.4, 0.5, 0.6] * 500},  # 1500 dims
                {"embedding": [0.7, 0.8, 0.9] * 500},  # 1500 dims
            ]
        }
        mock_aembedding.return_value = mock_response

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

        # Verify LiteLLM was called correctly
        mock_aembedding.assert_called_once_with(
            model="text-embedding-3-small",
            input=[
                "python programming",
                "javascript development",
                "java enterprise",
            ],
            timeout=30.0,
        )

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.aembedding"
    )
    async def test_embed_with_base_url(self, mock_aembedding: AsyncMock) -> None:
        """Test embedding with custom base URL."""
        endpoint = Endpoint(base_url="https://custom.api.com")
        provider = LiteLLMEmbeddingProvider(endpoint)

        mock_response = Mock()
        mock_response.model_dump.return_value = {"data": [{"embedding": [0.1] * 1500}]}
        mock_aembedding.return_value = mock_response

        requests = [EmbeddingRequest(snippet_id=1, text="test")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Verify base_url was passed
        mock_aembedding.assert_called_once_with(
            model="text-embedding-3-small",
            input=["test"],
            timeout=30.0,
            api_base="https://custom.api.com",
        )

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.aembedding"
    )
    async def test_embed_with_api_key(self, mock_aembedding: AsyncMock) -> None:
        """Test embedding with API key."""
        endpoint = Endpoint(api_key="sk-test-key-123")
        provider = LiteLLMEmbeddingProvider(endpoint)

        mock_response = Mock()
        mock_response.model_dump.return_value = {"data": [{"embedding": [0.1] * 1500}]}
        mock_aembedding.return_value = mock_response

        requests = [EmbeddingRequest(snippet_id=1, text="test")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Verify api_key was passed
        mock_aembedding.assert_called_once_with(
            model="text-embedding-3-small",
            input=["test"],
            timeout=30.0,
            api_key="sk-test-key-123",
        )

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.aembedding"
    )
    async def test_embed_with_extra_params(self, mock_aembedding: AsyncMock) -> None:
        """Test embedding with extra parameters."""
        extra_params = {"temperature": 0.5, "max_tokens": 100}
        endpoint = Endpoint(extra_params=extra_params)
        provider = LiteLLMEmbeddingProvider(endpoint)

        mock_response = Mock()
        mock_response.model_dump.return_value = {"data": [{"embedding": [0.1] * 1500}]}
        mock_aembedding.return_value = mock_response

        requests = [EmbeddingRequest(snippet_id=1, text="test")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Verify extra params were passed
        mock_aembedding.assert_called_once_with(
            model="text-embedding-3-small",
            input=["test"],
            timeout=30.0,
            temperature=0.5,
            max_tokens=100,
        )

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.aembedding"
    )
    async def test_embed_batch_processing(self, mock_aembedding: AsyncMock) -> None:
        """Test that requests are processed in batches."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock responses for different batches
        async def mock_aembedding_func(**kwargs: Any) -> Mock:
            input_size = len(kwargs["input"])
            mock_response = Mock()
            mock_response.model_dump.return_value = {
                "data": [{"embedding": [0.1] * 1500} for _ in range(input_size)]
            }
            return mock_response

        mock_aembedding.side_effect = mock_aembedding_func

        # Create more than batch_size requests (batch_size = 10)
        requests = [EmbeddingRequest(snippet_id=i, text=f"text {i}") for i in range(15)]

        batch_count = 0
        total_results = []
        async for batch in provider.embed(requests):
            batch_count += 1
            total_results.extend(batch)

        assert len(total_results) == 15
        assert batch_count == 2  # Should be 2 batches: 10 + 5
        assert mock_aembedding.call_count == 2

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.aembedding"
    )
    async def test_embed_api_error_handling(self, mock_aembedding: AsyncMock) -> None:
        """Test handling of API errors."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)
        mock_aembedding.side_effect = Exception("LiteLLM API Error")

        requests = [EmbeddingRequest(snippet_id=1, text="python programming")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Should return no embeddings on error (empty batch)
        assert len(results) == 0

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.aembedding"
    )
    async def test_embed_response_without_model_dump(
        self, mock_aembedding: AsyncMock
    ) -> None:
        """Test handling response without model_dump method."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Mock response that doesn't have model_dump method (dict response)
        mock_response = {"data": [{"embedding": [0.1] * 1500}]}
        mock_aembedding.return_value = mock_response

        requests = [EmbeddingRequest(snippet_id=1, text="test")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert len(results[0].embedding) == 1500

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.aembedding"
    )
    async def test_embed_custom_model(self, mock_aembedding: AsyncMock) -> None:
        """Test embedding with a custom model."""
        endpoint = Endpoint(model="claude-3-haiku-20240307")
        provider = LiteLLMEmbeddingProvider(endpoint)

        mock_response = Mock()
        mock_response.model_dump.return_value = {"data": [{"embedding": [0.1] * 1500}]}
        mock_aembedding.return_value = mock_response

        requests = [EmbeddingRequest(snippet_id=1, text="test text")]

        results = []
        async for batch in provider.embed(requests):
            results.extend(batch)

        # Verify the custom model was used
        mock_aembedding.assert_called_once_with(
            model="claude-3-haiku-20240307",
            input=["test text"],
            timeout=30.0,
        )

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.litellm"
    )
    async def test_socket_path_setup(self, mock_litellm: Mock) -> None:
        """Test Unix socket setup."""
        endpoint = Endpoint(socket_path="/var/run/test.sock")
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Verify socket_path was stored
        assert provider.socket_path == "/var/run/test.sock"
        # Verify mock is available (to satisfy linter)
        assert mock_litellm is not None

        # Should complete without error
        await provider.close()

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.httpx"
    )
    @patch(
        "kodit.infrastructure.embedding.embedding_providers.litellm_embedding_provider.litellm"
    )
    async def test_socket_path_httpx_client_setup(
        self, mock_litellm: Mock, mock_httpx: Mock
    ) -> None:
        """Test that Unix socket creates proper HTTPX client."""
        mock_transport = Mock()
        mock_client = AsyncMock()
        mock_httpx.AsyncHTTPTransport.return_value = mock_transport
        mock_httpx.AsyncClient.return_value = mock_client

        endpoint = Endpoint(socket_path="/var/run/test.sock", timeout=60.0)
        provider = LiteLLMEmbeddingProvider(endpoint)

        # Verify HTTPX transport was created with socket
        mock_httpx.AsyncHTTPTransport.assert_called_once_with(uds="/var/run/test.sock")

        # Verify HTTPX client was created with transport
        mock_httpx.AsyncClient.assert_called_once_with(
            transport=mock_transport,
            base_url="http://localhost",
            timeout=60.0,
        )

        # Verify LiteLLM session was set
        assert mock_litellm.aclient_session == mock_client

        await provider.close()

    @pytest.mark.asyncio
    async def test_close(self) -> None:
        """Test close method (should not raise any errors)."""
        endpoint = Endpoint()
        provider = LiteLLMEmbeddingProvider(endpoint)
        # Should complete without error
        await provider.close()
