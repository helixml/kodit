"""Tests for the LiteLLM enrichment provider."""

from typing import Any
from unittest.mock import AsyncMock, Mock, patch

import pytest

from kodit.config import Endpoint
from kodit.domain.value_objects import EnrichmentRequest
from kodit.infrastructure.enrichment.litellm_enrichment_provider import (
    LiteLLMEnrichmentProvider,
)


class TestLiteLLMEnrichmentProvider:
    """Test the LiteLLM enrichment provider."""

    def test_init_default_values(self) -> None:
        """Test initialization with default values."""
        endpoint = Endpoint()
        provider = LiteLLMEnrichmentProvider(endpoint)
        assert provider.model_name == "gpt-4o-mini"
        assert provider.api_key is None
        assert provider.base_url is None
        assert provider.timeout == 30.0
        assert provider.extra_params == {}
        assert provider.log is not None

    def test_init_custom_values(self) -> None:
        """Test initialization with custom values."""
        extra_params = {"temperature": 0.7, "max_tokens": 150}
        endpoint = Endpoint(
            model="claude-3-sonnet-20240229",
            api_key="test-api-key",
            base_url="https://api.anthropic.com",
            timeout=60.0,
            extra_params=extra_params,
        )
        provider = LiteLLMEnrichmentProvider(endpoint)
        assert provider.model_name == "claude-3-sonnet-20240229"
        assert provider.api_key == "test-api-key"
        assert provider.base_url == "https://api.anthropic.com"
        assert provider.timeout == 60.0
        assert provider.extra_params == extra_params

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_single_request_success(
        self, mock_acompletion: AsyncMock
    ) -> None:
        """Test successful enrichment with a single request."""
        endpoint = Endpoint()
        provider = LiteLLMEnrichmentProvider(endpoint)

        # Mock LiteLLM response
        mock_response = Mock()
        mock_response.model_dump.return_value = {
            "choices": [
                {
                    "message": {
                        "content": "This is a Python function that calculates the sum."
                    }
                }
            ]
        }
        mock_acompletion.return_value = mock_response

        requests = [EnrichmentRequest(snippet_id=1, text="def add(a, b): return a + b")]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == "This is a Python function that calculates the sum."

        # Verify LiteLLM was called correctly
        mock_acompletion.assert_called_once()
        call_args = mock_acompletion.call_args[1]
        assert call_args["model"] == "gpt-4o-mini"
        assert len(call_args["messages"]) == 2
        assert call_args["messages"][0]["role"] == "system"
        assert call_args["messages"][1]["role"] == "user"
        assert call_args["messages"][1]["content"] == "def add(a, b): return a + b"
        assert call_args["timeout"] == 30.0

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_multiple_requests_success(
        self, mock_acompletion: AsyncMock
    ) -> None:
        """Test successful enrichment with multiple requests."""
        endpoint = Endpoint()
        provider = LiteLLMEnrichmentProvider(endpoint)

        # Mock LiteLLM responses
        responses = [
            "This is a function that adds two numbers.",
            "This is a function that multiplies two numbers.",
            "This is a function that divides two numbers.",
        ]

        async def mock_acompletion_func(**kwargs: Any) -> Mock:
            mock_response = Mock()
            # Get the request content to determine response
            content = kwargs["messages"][1]["content"]
            if "add" in content:
                response_text = responses[0]
            elif "multiply" in content:
                response_text = responses[1]
            else:
                response_text = responses[2]

            mock_response.model_dump.return_value = {
                "choices": [{"message": {"content": response_text}}]
            }
            return mock_response

        mock_acompletion.side_effect = mock_acompletion_func

        requests = [
            EnrichmentRequest(snippet_id=1, text="def add(a, b): return a + b"),
            EnrichmentRequest(snippet_id=2, text="def multiply(a, b): return a * b"),
            EnrichmentRequest(snippet_id=3, text="def divide(a, b): return a / b"),
        ]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 3

        # Sort results by snippet_id since async processing may return them out of order
        sorted_results = sorted(results, key=lambda r: r.snippet_id)

        assert sorted_results[0].snippet_id == 1
        assert "adds two numbers" in sorted_results[0].text
        assert sorted_results[1].snippet_id == 2
        assert "multiplies two numbers" in sorted_results[1].text
        assert sorted_results[2].snippet_id == 3
        assert "divides two numbers" in sorted_results[2].text

        # Verify LiteLLM was called 3 times
        assert mock_acompletion.call_count == 3

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_with_base_url(self, mock_acompletion: AsyncMock) -> None:
        """Test enrichment with custom base URL."""
        endpoint = Endpoint(base_url="https://custom.api.com")
        provider = LiteLLMEnrichmentProvider(endpoint)

        mock_response = Mock()
        mock_response.model_dump.return_value = {
            "choices": [{"message": {"content": "Custom response"}}]
        }
        mock_acompletion.return_value = mock_response

        requests = [EnrichmentRequest(snippet_id=1, text="test code")]

        [result async for result in provider.enrich(requests)]

        # Verify base_url was passed
        call_args = mock_acompletion.call_args[1]
        assert call_args["api_base"] == "https://custom.api.com"

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_with_api_key(self, mock_acompletion: AsyncMock) -> None:
        """Test enrichment with API key."""
        endpoint = Endpoint(api_key="sk-test-key-456")
        provider = LiteLLMEnrichmentProvider(endpoint)

        mock_response = Mock()
        mock_response.model_dump.return_value = {
            "choices": [{"message": {"content": "Response with API key"}}]
        }
        mock_acompletion.return_value = mock_response

        requests = [EnrichmentRequest(snippet_id=1, text="test code")]

        [result async for result in provider.enrich(requests)]

        # Verify api_key was passed
        call_args = mock_acompletion.call_args[1]
        assert call_args["api_key"] == "sk-test-key-456"

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_with_extra_params(self, mock_acompletion: AsyncMock) -> None:
        """Test enrichment with extra parameters."""
        extra_params = {"temperature": 0.8, "max_tokens": 200}
        endpoint = Endpoint(extra_params=extra_params)
        provider = LiteLLMEnrichmentProvider(endpoint)

        mock_response = Mock()
        mock_response.model_dump.return_value = {
            "choices": [{"message": {"content": "Response with extra params"}}]
        }
        mock_acompletion.return_value = mock_response

        requests = [EnrichmentRequest(snippet_id=1, text="test")]

        [result async for result in provider.enrich(requests)]

        # Verify extra params were passed
        call_args = mock_acompletion.call_args[1]
        assert call_args["temperature"] == 0.8
        assert call_args["max_tokens"] == 200

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_empty_text_request(self, mock_acompletion: AsyncMock) -> None:
        """Test enrichment with empty text."""
        endpoint = Endpoint()
        provider = LiteLLMEnrichmentProvider(endpoint)

        requests = [EnrichmentRequest(snippet_id=1, text="")]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == ""

        # Should not call LiteLLM for empty text
        mock_acompletion.assert_not_called()

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_api_error_handling(self, mock_acompletion: AsyncMock) -> None:
        """Test handling of API errors."""
        endpoint = Endpoint()
        provider = LiteLLMEnrichmentProvider(endpoint)
        mock_acompletion.side_effect = Exception("LiteLLM API Error")

        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        results = [result async for result in provider.enrich(requests)]

        # Should return empty text on error
        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == ""

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_response_without_model_dump(
        self, mock_acompletion: AsyncMock
    ) -> None:
        """Test handling response without model_dump method."""
        endpoint = Endpoint()
        provider = LiteLLMEnrichmentProvider(endpoint)

        # Mock response that doesn't have model_dump method (dict response)
        mock_response = {
            "choices": [{"message": {"content": "Response without model_dump"}}]
        }
        mock_acompletion.return_value = mock_response

        requests = [EnrichmentRequest(snippet_id=1, text="test")]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == "Response without model_dump"

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_malformed_response(self, mock_acompletion: AsyncMock) -> None:
        """Test handling of malformed API response."""
        endpoint = Endpoint()
        provider = LiteLLMEnrichmentProvider(endpoint)

        # Mock malformed response
        mock_response = Mock()
        mock_response.model_dump.return_value = {
            "choices": [{}]  # Missing message field
        }
        mock_acompletion.return_value = mock_response

        requests = [EnrichmentRequest(snippet_id=1, text="test")]

        results = [result async for result in provider.enrich(requests)]

        # Should handle gracefully and return empty text
        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == ""

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_custom_model(self, mock_acompletion: AsyncMock) -> None:
        """Test enrichment with a custom model."""
        endpoint = Endpoint(model="claude-3-opus-20240229")
        provider = LiteLLMEnrichmentProvider(endpoint)

        mock_response = Mock()
        mock_response.model_dump.return_value = {
            "choices": [{"message": {"content": "Custom model response"}}]
        }
        mock_acompletion.return_value = mock_response

        requests = [EnrichmentRequest(snippet_id=1, text="test code")]

        [result async for result in provider.enrich(requests)]

        # Verify the custom model was used
        call_args = mock_acompletion.call_args[1]
        assert call_args["model"] == "claude-3-opus-20240229"

    @pytest.mark.asyncio
    @patch(
        "kodit.infrastructure.enrichment.litellm_enrichment_provider.clean_thinking_tags"
    )
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.acompletion")
    async def test_enrich_cleans_thinking_tags(
        self, mock_acompletion: AsyncMock, mock_clean_thinking: Mock
    ) -> None:
        """Test that thinking tags are cleaned from responses."""
        endpoint = Endpoint()
        provider = LiteLLMEnrichmentProvider(endpoint)

        mock_response = Mock()
        mock_response.model_dump.return_value = {
            "choices": [
                {"message": {"content": "Response with <thinking>tags</thinking>"}}
            ]
        }
        mock_acompletion.return_value = mock_response
        mock_clean_thinking.return_value = "Response with tags"

        requests = [EnrichmentRequest(snippet_id=1, text="test")]

        results = [result async for result in provider.enrich(requests)]

        # Verify thinking tag cleaning was called
        mock_clean_thinking.assert_called_once_with(
            "Response with <thinking>tags</thinking>"
        )
        assert results[0].text == "Response with tags"

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.litellm")
    async def test_socket_path_setup(self, mock_litellm: Mock) -> None:
        """Test Unix socket setup."""
        endpoint = Endpoint(socket_path="/var/run/test.sock")
        provider = LiteLLMEnrichmentProvider(endpoint)

        # Verify socket_path was stored
        assert provider.socket_path == "/var/run/test.sock"
        # Verify mock is available (to satisfy linter)
        assert mock_litellm is not None

        # Should complete without error
        await provider.close()

    @pytest.mark.asyncio
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.httpx")
    @patch("kodit.infrastructure.enrichment.litellm_enrichment_provider.litellm")
    async def test_socket_path_httpx_client_setup(
        self, mock_litellm: Mock, mock_httpx: Mock
    ) -> None:
        """Test that Unix socket creates proper HTTPX client."""
        mock_transport = Mock()
        mock_client = AsyncMock()
        mock_httpx.AsyncHTTPTransport.return_value = mock_transport
        mock_httpx.AsyncClient.return_value = mock_client

        endpoint = Endpoint(socket_path="/var/run/test.sock", timeout=60.0)
        provider = LiteLLMEnrichmentProvider(endpoint)

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
        provider = LiteLLMEnrichmentProvider(endpoint)
        # Should complete without error
        await provider.close()
