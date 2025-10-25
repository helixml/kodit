"""Tests for the LiteLLM provider."""

from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import litellm
import pytest

from kodit.config import Endpoint
from kodit.infrastructure.providers.litellm_provider import LiteLLMProvider


class TestLiteLLMProvider:
    """Test the LiteLLM provider."""

    def test_init_without_socket_path(self) -> None:
        """Test initialization without Unix socket path."""
        endpoint = Endpoint(model="gpt-4o-mini", api_key="test-key")
        provider = LiteLLMProvider(endpoint)

        assert provider.endpoint.model == "gpt-4o-mini"
        assert provider.endpoint.api_key == "test-key"
        assert provider.endpoint.socket_path is None

    @patch("kodit.infrastructure.providers.litellm_provider.httpx.AsyncClient")
    @patch("kodit.infrastructure.providers.litellm_provider.httpx.AsyncHTTPTransport")
    def test_init_with_socket_path(
        self, mock_transport: MagicMock, mock_client: MagicMock
    ) -> None:
        """Test initialization with Unix socket path configures httpx client."""
        endpoint = Endpoint(
            model="gpt-4o-mini",
            socket_path="/var/run/llm.sock",
            timeout=60.0,
        )

        # Create mock instances
        mock_transport_instance = MagicMock()
        mock_transport.return_value = mock_transport_instance
        mock_client_instance = MagicMock()
        mock_client.return_value = mock_client_instance

        LiteLLMProvider(endpoint)

        # Verify transport was created with Unix socket
        mock_transport.assert_called_once_with(uds="/var/run/llm.sock")

        # Verify httpx client was created with correct parameters
        mock_client.assert_called_once_with(
            transport=mock_transport_instance,
            base_url="http://localhost",
            timeout=60.0,
        )

        # Verify litellm's aclient_session was set
        assert litellm.aclient_session == mock_client_instance

    def test_socket_path_not_set_when_none(self) -> None:
        """Test that socket setup is skipped when socket_path is None."""
        endpoint = Endpoint(model="gpt-4o-mini")

        # Store original session
        original_session = getattr(litellm, "aclient_session", None)

        try:
            # Set to None to ensure clean state
            litellm.aclient_session = None

            provider = LiteLLMProvider(endpoint)

            # Verify socket_path is None
            assert provider.endpoint.socket_path is None

            # Verify litellm.aclient_session was not set (still None)
            assert litellm.aclient_session is None
        finally:
            # Restore original session
            litellm.aclient_session = original_session

    def test_populate_base_kwargs_with_all_params(self) -> None:
        """Test _populate_base_kwargs includes all endpoint parameters."""
        endpoint = Endpoint(
            model="gpt-4o-mini",
            api_key="test-key",
            base_url="https://custom.openai.com",
            timeout=120.0,
            extra_params={"temperature": 0.7, "max_tokens": 1000},
        )
        provider = LiteLLMProvider(endpoint)

        kwargs = provider._populate_base_kwargs()  # noqa: SLF001

        assert kwargs["model"] == "gpt-4o-mini"
        assert kwargs["timeout"] == 120.0
        assert kwargs["api_key"] == "test-key"
        assert kwargs["api_base"] == "https://custom.openai.com"
        assert kwargs["temperature"] == 0.7
        assert kwargs["max_tokens"] == 1000

    def test_populate_base_kwargs_minimal(self) -> None:
        """Test _populate_base_kwargs with minimal configuration."""
        endpoint = Endpoint(model="gpt-4o-mini")
        provider = LiteLLMProvider(endpoint)

        kwargs = provider._populate_base_kwargs()  # noqa: SLF001

        assert kwargs["model"] == "gpt-4o-mini"
        assert kwargs["timeout"] == 60
        assert "api_key" not in kwargs
        assert "api_base" not in kwargs

    @pytest.mark.asyncio
    async def test_chat_completion_calls_acompletion(self) -> None:
        """Test chat_completion calls litellm's acompletion with correct params."""
        endpoint = Endpoint(model="gpt-4o-mini", api_key="test-key")
        provider = LiteLLMProvider(endpoint)

        messages = [
            {"role": "system", "content": "You are a helpful assistant."},
            {"role": "user", "content": "Hello!"},
        ]

        # Mock the acompletion function
        mock_response = MagicMock()
        mock_response.model_dump.return_value = {
            "choices": [{"message": {"content": "Hi there!"}}]
        }

        with patch(
            "kodit.infrastructure.providers.litellm_provider.acompletion",
            new_callable=AsyncMock,
            return_value=mock_response,
        ) as mock_acompletion:
            result = await provider.chat_completion(messages)

            # Verify acompletion was called with correct parameters
            mock_acompletion.assert_called_once()
            call_kwargs = mock_acompletion.call_args[1]
            assert call_kwargs["model"] == "gpt-4o-mini"
            assert call_kwargs["messages"] == messages
            assert call_kwargs["api_key"] == "test-key"
            assert call_kwargs["max_retries"] == 0

            # Verify result
            assert result == {"choices": [{"message": {"content": "Hi there!"}}]}

    @pytest.mark.asyncio
    async def test_embedding_calls_aembedding(self) -> None:
        """Test embedding calls litellm's aembedding with correct params."""
        endpoint = Endpoint(model="text-embedding-3-small", api_key="test-key")
        provider = LiteLLMProvider(endpoint)

        texts = ["Hello world", "Test text"]

        # Mock the aembedding function
        mock_response = MagicMock()
        mock_response.model_dump.return_value = {
            "data": [
                {"embedding": [0.1, 0.2, 0.3]},
                {"embedding": [0.4, 0.5, 0.6]},
            ]
        }

        with patch(
            "kodit.infrastructure.providers.litellm_provider.aembedding",
            new_callable=AsyncMock,
            return_value=mock_response,
        ) as mock_aembedding:
            result = await provider.embedding(texts)

            # Verify aembedding was called with correct parameters
            mock_aembedding.assert_called_once()
            call_kwargs = mock_aembedding.call_args[1]
            assert call_kwargs["model"] == "text-embedding-3-small"
            assert call_kwargs["input"] == texts
            assert call_kwargs["api_key"] == "test-key"
            assert call_kwargs["max_retries"] == 0

            # Verify result
            assert result == {
                "data": [
                    {"embedding": [0.1, 0.2, 0.3]},
                    {"embedding": [0.4, 0.5, 0.6]},
                ]
            }

    @pytest.mark.asyncio
    async def test_close_without_socket_path(self) -> None:
        """Test close method when not using Unix sockets."""
        endpoint = Endpoint(model="gpt-4o-mini")
        provider = LiteLLMProvider(endpoint)

        # Should complete without error
        await provider.close()

    @pytest.mark.asyncio
    async def test_close_with_socket_path(self) -> None:
        """Test close method cleans up httpx client when using Unix sockets."""
        endpoint = Endpoint(
            model="gpt-4o-mini",
            socket_path="/var/run/llm.sock",
        )

        # Mock httpx client
        mock_client = AsyncMock(spec=httpx.AsyncClient)
        mock_client.aclose = AsyncMock()

        with patch(
            "kodit.infrastructure.providers.litellm_provider.httpx.AsyncClient",
            return_value=mock_client,
        ), patch(
            "kodit.infrastructure.providers.litellm_provider.httpx.AsyncHTTPTransport"
        ):
            provider = LiteLLMProvider(endpoint)

            # Verify client was set
            assert litellm.aclient_session == mock_client

            # Close the provider
            await provider.close()

            # Verify client was closed
            mock_client.aclose.assert_called_once()

            # Verify litellm.aclient_session was set to None
            assert litellm.aclient_session is None

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.providers.litellm_provider.httpx.AsyncClient")
    @patch("kodit.infrastructure.providers.litellm_provider.httpx.AsyncHTTPTransport")
    async def test_socket_path_end_to_end(
        self, mock_transport: MagicMock, mock_client_class: MagicMock
    ) -> None:
        """Test Unix socket setup and cleanup end-to-end."""
        # Create mock client instance
        mock_client = AsyncMock()
        mock_client.aclose = AsyncMock()
        mock_client_class.return_value = mock_client

        # Create mock transport
        mock_transport_instance = MagicMock()
        mock_transport.return_value = mock_transport_instance

        endpoint = Endpoint(
            model="gpt-4o-mini",
            socket_path="/tmp/test.sock",
            timeout=30.0,
        )

        # Initialize provider (should setup socket)
        provider = LiteLLMProvider(endpoint)

        # Verify transport was created
        mock_transport.assert_called_once_with(uds="/tmp/test.sock")

        # Verify client was created
        mock_client_class.assert_called_once_with(
            transport=mock_transport_instance,
            base_url="http://localhost",
            timeout=30.0,
        )

        # Verify litellm session was set
        assert litellm.aclient_session == mock_client

        # Close provider (should cleanup socket)
        await provider.close()

        # Verify client was closed
        mock_client.aclose.assert_called_once()

        # Verify session was cleared
        assert litellm.aclient_session is None
