"""Tests for the OpenAI enrichment provider."""

import asyncio
from typing import Any
from unittest.mock import AsyncMock, Mock

import httpx
import pytest

from kodit.domain.value_objects import EnrichmentRequest
from kodit.infrastructure.enrichment.openai_enrichment_provider import (
    OpenAIEnrichmentProvider,
)


class TestOpenAIEnrichmentProvider:
    """Test the OpenAI enrichment provider."""

    def test_init_default_values(self) -> None:
        """Test initialization with default values."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")
        assert provider.model_name == "gpt-4o-mini"
        assert provider.api_key == "test-key"
        assert provider.base_url == "https://api.openai.com"
        assert provider.socket_path is None
        assert provider.log is not None

    def test_init_custom_values(self) -> None:
        """Test initialization with custom values."""
        provider = OpenAIEnrichmentProvider(
            api_key="test-key",
            base_url="https://custom.openai.com",
            model_name="gpt-4",
            socket_path="/tmp/socket.sock",  # noqa: S108
        )
        assert provider.model_name == "gpt-4"
        assert provider.base_url == "https://custom.openai.com"
        assert provider.socket_path == "/tmp/socket.sock"  # noqa: S108

    def test_init_with_timeout(self) -> None:
        """Test initialization with custom timeout."""
        provider = OpenAIEnrichmentProvider(api_key="test-key", timeout=45.0)
        assert provider.timeout == 45.0

        # Test default timeout
        provider_default = OpenAIEnrichmentProvider(api_key="test-key")
        assert provider_default.timeout == 30.0

    @pytest.mark.asyncio
    async def test_enrich_empty_requests(self) -> None:
        """Test enrichment with empty requests."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")
        requests: list[EnrichmentRequest] = []

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_enrich_empty_text_requests(self) -> None:
        """Test enrichment with requests containing empty text."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")

        # Mock the httpx client
        mock_response = Mock()
        mock_response.json.return_value = {
            "choices": [{"message": {"content": "Whitespace response"}}]
        }
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client

        requests = [
            EnrichmentRequest(snippet_id=1, text=""),
            EnrichmentRequest(snippet_id=2, text="   "),
        ]

        results = [result async for result in provider.enrich(requests)]

        # Should return responses for all requests
        assert len(results) == 2
        # Results come back in completion order, not request order
        snippet_ids = [result.snippet_id for result in results]
        assert 1 in snippet_ids
        assert 2 in snippet_ids
        # Empty text should return empty response
        empty_result = next(r for r in results if r.snippet_id == 1)
        assert empty_result.text == ""
        # The whitespace-only text will be processed by the API
        whitespace_result = next(r for r in results if r.snippet_id == 2)
        assert whitespace_result.text == "Whitespace response"
        # Should only call API for the whitespace request (empty text is skipped)
        provider.http_client.post.assert_called_once()

    @pytest.mark.asyncio
    async def test_enrich_single_request_success(self) -> None:
        """Test successful enrichment with a single request."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")

        # Mock the httpx client
        mock_response = Mock()
        mock_response.json.return_value = {
            "choices": [{"message": {"content": "This is a test function"}}]
        }
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == "This is a test function"

        # Verify the API was called correctly
        provider.http_client.post.assert_called_once_with(
            "/v1/chat/completions",
            json={
                "model": "gpt-4o-mini",
                "messages": [
                    {
                        "role": "system",
                        "content": (
                            "\nYou are a professional software developer. "
                            "You will be given a snippet of code.\nPlease provide "
                            "a concise explanation of the code.\n"
                        ),
                    },
                    {"role": "user", "content": "def test(): pass"},
                ],
            },
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer test-key",
            },
        )

    @pytest.mark.asyncio
    async def test_enrich_multiple_requests_success(self) -> None:
        """Test successful enrichment with multiple requests."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")

        # Mock responses for multiple calls
        async def mock_post(url: str, **kwargs: Any) -> httpx.Response:  # noqa: ARG001
            content = kwargs["json"]["messages"][1]["content"]
            if "hello" in content:
                response_text = "First function"
            else:
                response_text = "Second function"

            mock_response = Mock()
            mock_response.json.return_value = {
                "choices": [{"message": {"content": response_text}}]
            }
            mock_response.raise_for_status = Mock()
            return mock_response

        mock_client = Mock()
        mock_client.post = AsyncMock(side_effect=mock_post)
        provider.http_client = mock_client
        requests = [
            EnrichmentRequest(snippet_id=1, text="def hello(): pass"),
            EnrichmentRequest(snippet_id=2, text="def world(): pass"),
        ]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 2
        # Results come back in completion order, so we need to check by snippet_id
        snippet_ids = [result.snippet_id for result in results]
        assert 1 in snippet_ids
        assert 2 in snippet_ids

        # Find each result by snippet_id
        result1 = next(r for r in results if r.snippet_id == 1)
        result2 = next(r for r in results if r.snippet_id == 2)

        # The content should match expected responses
        assert result1.text == "First function"
        assert result2.text == "Second function"

        # Verify the API was called twice
        assert provider.http_client.post.call_count == 2

    @pytest.mark.asyncio
    async def test_enrich_mixed_requests(self) -> None:
        """Test enrichment with mixed valid and empty requests."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")

        mock_response = Mock()
        mock_response.json.return_value = {
            "choices": [{"message": {"content": "Valid function"}}]
        }
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
        requests = [
            EnrichmentRequest(snippet_id=1, text=""),  # Empty
            EnrichmentRequest(snippet_id=2, text="def valid(): pass"),  # Valid
            EnrichmentRequest(snippet_id=3, text="   "),  # Whitespace only
        ]

        results = [result async for result in provider.enrich(requests)]

        # Should return responses for all requests
        assert len(results) == 3
        # Results come back in completion order, so we need to check by snippet_id
        snippet_ids = [result.snippet_id for result in results]
        assert 1 in snippet_ids
        assert 2 in snippet_ids
        assert 3 in snippet_ids

        # Find the valid response
        valid_result = next(r for r in results if r.snippet_id == 2)
        assert valid_result.text == "Valid function"

        # Empty response should have empty text
        empty_result = next(r for r in results if r.snippet_id == 1)
        assert empty_result.text == ""

        # Whitespace-only text will be processed by the API
        whitespace_result = next(r for r in results if r.snippet_id == 3)
        assert whitespace_result.text == "Valid function"

        # Should call API for both valid and whitespace requests (2 calls)
        assert provider.http_client.post.call_count == 2

    @pytest.mark.asyncio
    async def test_enrich_api_error_handling(self) -> None:
        """Test handling of API errors."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")
        mock_client = Mock()
        mock_client.post = AsyncMock(side_effect=Exception("API Error"))
        provider.http_client = mock_client
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        results = [result async for result in provider.enrich(requests)]

        # Should return empty response on error
        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == ""

    @pytest.mark.asyncio
    async def test_enrich_null_content_handling(self) -> None:
        """Test handling of null content in API response."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")

        mock_response = Mock()
        mock_response.json.return_value = {"choices": [{"message": {"content": None}}]}
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        results = [result async for result in provider.enrich(requests)]

        # Should return empty string for null content
        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == ""

    @pytest.mark.asyncio
    async def test_enrich_concurrent_requests(self) -> None:
        """Test that requests are processed concurrently."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")

        # Track call order to verify concurrency
        call_order = []

        async def mock_post(url: str, **kwargs: Any) -> httpx.Response:  # noqa: ARG001
            # Simulate some processing time
            await asyncio.sleep(0.1)
            content = kwargs.get("json", {}).get("messages", [{}])[1].get("content", "")
            call_order.append(content)

            mock_response = Mock()
            mock_response.json.return_value = {
                "choices": [{"message": {"content": f"Response for {content}"}}]
            }
            mock_response.raise_for_status = Mock()
            return mock_response

        mock_client = Mock()
        mock_client.post = AsyncMock(side_effect=mock_post)
        provider.http_client = mock_client
        requests = [
            EnrichmentRequest(snippet_id=1, text="def first(): pass"),
            EnrichmentRequest(snippet_id=2, text="def second(): pass"),
            EnrichmentRequest(snippet_id=3, text="def third(): pass"),
        ]

        start_time = asyncio.get_event_loop().time()
        results = [result async for result in provider.enrich(requests)]
        end_time = asyncio.get_event_loop().time()

        # Should process all requests
        assert len(results) == 3

        # Should complete faster than sequential processing (3 * 0.1 = 0.3 seconds)
        # Allow some overhead for async processing
        assert end_time - start_time < 0.4

    @pytest.mark.asyncio
    async def test_enrich_semaphore_limit(self) -> None:
        """Test that the semaphore limits concurrent requests."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")

        active_requests = 0
        max_concurrent = 0

        async def mock_post(url: str, **kwargs: Any) -> httpx.Response:  # noqa: ARG001
            nonlocal active_requests, max_concurrent
            active_requests += 1
            max_concurrent = max(max_concurrent, active_requests)

            # Simulate processing time
            await asyncio.sleep(0.1)

            active_requests -= 1

            mock_response = Mock()
            mock_response.json.return_value = {
                "choices": [{"message": {"content": "Response"}}]
            }
            mock_response.raise_for_status = Mock()
            return mock_response

        mock_client = Mock()
        mock_client.post = AsyncMock(side_effect=mock_post)
        provider.http_client = mock_client
        requests = [
            EnrichmentRequest(snippet_id=i, text=f"def func{i}(): pass")
            for i in range(50)  # More than the semaphore limit
        ]

        results = [result async for result in provider.enrich(requests)]

        # Should process all requests
        assert len(results) == 50

        # Should not exceed semaphore limit (default is 40)
        assert max_concurrent <= 40

    @pytest.mark.asyncio
    async def test_enrich_custom_model(self) -> None:
        """Test enrichment with a custom model."""
        provider = OpenAIEnrichmentProvider(api_key="test-key", model_name="gpt-4")

        mock_response = Mock()
        mock_response.json.return_value = {
            "choices": [{"message": {"content": "Custom model response"}}]
        }
        mock_response.raise_for_status = Mock()

        mock_client = Mock()
        mock_client.post = AsyncMock(return_value=mock_response)
        provider.http_client = mock_client
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        [result async for result in provider.enrich(requests)]

        # Verify the custom model was used
        provider.http_client.post.assert_called_once_with(
            "/v1/chat/completions",
            json={
                "model": "gpt-4",
                "messages": [
                    {
                        "role": "system",
                        "content": (
                            "\nYou are a professional software developer. "
                            "You will be given a snippet of code.\nPlease provide "
                            "a concise explanation of the code.\n"
                        ),
                    },
                    {"role": "user", "content": "def test(): pass"},
                ],
            },
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer test-key",
            },
        )

    @pytest.mark.asyncio
    async def test_close(self) -> None:
        """Test close method."""
        provider = OpenAIEnrichmentProvider(api_key="test-key")
        mock_client = Mock()
        mock_client.aclose = AsyncMock()
        provider.http_client = mock_client

        await provider.close()

        provider.http_client.aclose.assert_called_once()
