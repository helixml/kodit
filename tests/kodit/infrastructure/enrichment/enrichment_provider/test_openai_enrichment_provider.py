"""Tests for the OpenAI enrichment provider."""

import asyncio
from unittest.mock import AsyncMock, MagicMock

import pytest

from kodit.domain.value_objects import EnrichmentRequest
from kodit.infrastructure.enrichment.openai_enrichment_provider import (
    OpenAIEnrichmentProvider,
)


class TestOpenAIEnrichmentProvider:
    """Test the OpenAI enrichment provider."""

    def test_init_default_values(self) -> None:
        """Test initialization with default values."""
        mock_client = MagicMock()
        provider = OpenAIEnrichmentProvider(openai_client=mock_client)
        assert provider.openai_client == mock_client
        assert provider.model_name == "gpt-4o-mini"

    def test_init_custom_values(self) -> None:
        """Test initialization with custom values."""
        mock_client = MagicMock()
        provider = OpenAIEnrichmentProvider(
            openai_client=mock_client, model_name="gpt-4"
        )
        assert provider.openai_client == mock_client
        assert provider.model_name == "gpt-4"

    @pytest.mark.asyncio
    async def test_enrich_empty_requests(self) -> None:
        """Test enrichment with empty requests."""
        mock_client = MagicMock()
        provider = OpenAIEnrichmentProvider(openai_client=mock_client)
        requests = []

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 0
        mock_client.chat.completions.create.assert_not_called()

    @pytest.mark.asyncio
    async def test_enrich_empty_text_requests(self) -> None:
        """Test enrichment with requests containing empty text."""
        mock_client = MagicMock()
        # Mock the chat.completions.create method as AsyncMock with a proper response
        mock_response = MagicMock()
        mock_response.choices = [MagicMock()]
        mock_response.choices[0].message.content = "Whitespace response"
        mock_client.chat.completions.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEnrichmentProvider(openai_client=mock_client)
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
        mock_client.chat.completions.create.assert_called_once()

    @pytest.mark.asyncio
    async def test_enrich_single_request_success(self) -> None:
        """Test successful enrichment with a single request."""
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.choices = [MagicMock()]
        mock_response.choices[0].message.content = "This is a test function"

        # Use AsyncMock for the async method
        mock_client.chat.completions.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEnrichmentProvider(openai_client=mock_client)
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == "This is a test function"

        # Verify the API was called correctly - note the extra newlines in the prompt
        mock_client.chat.completions.create.assert_called_once_with(
            model="gpt-4o-mini",
            messages=[
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
        )

    @pytest.mark.asyncio
    async def test_enrich_multiple_requests_success(self) -> None:
        """Test successful enrichment with multiple requests."""
        mock_client = MagicMock()

        # Mock responses for multiple calls
        mock_response1 = MagicMock()
        mock_response1.choices = [MagicMock()]
        mock_response1.choices[0].message.content = "First function"

        mock_response2 = MagicMock()
        mock_response2.choices = [MagicMock()]
        mock_response2.choices[0].message.content = "Second function"

        # Use AsyncMock with side_effect for multiple calls
        mock_client.chat.completions.create = AsyncMock(
            side_effect=[mock_response1, mock_response2]
        )

        provider = OpenAIEnrichmentProvider(openai_client=mock_client)
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

        # The content should match one of the responses
        assert result1.text in ["First function", "Second function"]
        assert result2.text in ["First function", "Second function"]
        assert result1.text != result2.text  # They should be different

        # Verify the API was called twice
        assert mock_client.chat.completions.create.call_count == 2

    @pytest.mark.asyncio
    async def test_enrich_mixed_requests(self) -> None:
        """Test enrichment with mixed valid and empty requests."""
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.choices = [MagicMock()]
        mock_response.choices[0].message.content = "Valid function"
        mock_client.chat.completions.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEnrichmentProvider(openai_client=mock_client)
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

        # Whitespace-only text will be processed by the API, but since we mocked it
        # to return the same response, it will get "Valid function" too
        whitespace_result = next(r for r in results if r.snippet_id == 3)
        assert whitespace_result.text == "Valid function"

        # Should call API for both valid and whitespace requests (2 calls)
        assert mock_client.chat.completions.create.call_count == 2

    @pytest.mark.asyncio
    async def test_enrich_api_error_handling(self) -> None:
        """Test handling of API errors."""
        mock_client = MagicMock()
        mock_client.chat.completions.create = AsyncMock(
            side_effect=Exception("API Error")
        )

        provider = OpenAIEnrichmentProvider(openai_client=mock_client)
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        results = [result async for result in provider.enrich(requests)]

        # Should return empty response on error
        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == ""

    @pytest.mark.asyncio
    async def test_enrich_null_content_handling(self) -> None:
        """Test handling of null content in API response."""
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.choices = [MagicMock()]
        mock_response.choices[0].message.content = None
        mock_client.chat.completions.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEnrichmentProvider(openai_client=mock_client)
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        results = [result async for result in provider.enrich(requests)]

        # Should return empty string for null content
        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == ""

    @pytest.mark.asyncio
    async def test_enrich_concurrent_requests(self) -> None:
        """Test that requests are processed concurrently."""
        mock_client = MagicMock()

        # Track call order to verify concurrency
        call_order = []

        async def mock_create(*args, **kwargs) -> MagicMock:  # noqa: ANN002, ANN003, ARG001
            # Simulate some processing time
            await asyncio.sleep(0.1)
            call_order.append(kwargs.get("messages", [{}])[1].get("content", ""))

            mock_response = MagicMock()
            mock_response.choices = [MagicMock()]
            mock_response.choices[
                0
            ].message.content = (
                f"Response for {kwargs.get('messages', [{}])[1].get('content', '')}"
            )
            return mock_response

        mock_client.chat.completions.create = mock_create

        provider = OpenAIEnrichmentProvider(openai_client=mock_client)
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
        mock_client = MagicMock()

        active_requests = 0
        max_concurrent = 0

        async def mock_create(*args, **kwargs) -> MagicMock:  # noqa: ANN002, ANN003, ARG001
            nonlocal active_requests, max_concurrent
            active_requests += 1
            max_concurrent = max(max_concurrent, active_requests)

            # Simulate processing time
            await asyncio.sleep(0.1)

            active_requests -= 1

            mock_response = MagicMock()
            mock_response.choices = [MagicMock()]
            mock_response.choices[0].message.content = "Response"
            return mock_response

        mock_client.chat.completions.create = mock_create

        provider = OpenAIEnrichmentProvider(openai_client=mock_client)
        requests = [
            EnrichmentRequest(snippet_id=i, text=f"def func{i}(): pass")
            for i in range(10)  # More than the semaphore limit
        ]

        results = [result async for result in provider.enrich(requests)]

        # Should process all requests
        assert len(results) == 10

        # Should not exceed semaphore limit (default is 10, not 5)
        assert max_concurrent <= 10

    @pytest.mark.asyncio
    async def test_enrich_custom_model(self) -> None:
        """Test enrichment with a custom model."""
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.choices = [MagicMock()]
        mock_response.choices[0].message.content = "Custom model response"
        mock_client.chat.completions.create = AsyncMock(return_value=mock_response)

        provider = OpenAIEnrichmentProvider(
            openai_client=mock_client, model_name="gpt-4"
        )
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        [result async for result in provider.enrich(requests)]

        # Verify the custom model was used
        mock_client.chat.completions.create.assert_called_once_with(
            model="gpt-4",
            messages=[
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
        )
