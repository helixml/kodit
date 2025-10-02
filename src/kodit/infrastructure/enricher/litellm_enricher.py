"""LiteLLM enricher implementation."""

import asyncio
from collections.abc import AsyncGenerator
from typing import Any

import httpx
import litellm
import structlog
from litellm import acompletion

from kodit.config import Endpoint
from kodit.domain.protocols import Enricher
from kodit.domain.value_objects import (
    GenericEnrichmentRequest,
    GenericEnrichmentResponse,
)
from kodit.infrastructure.enrichment.utils import clean_thinking_tags

DEFAULT_NUM_PARALLEL_TASKS = 20


class LiteLLMEnricher(Enricher):
    """LiteLLM enricher that supports 100+ providers."""

    def __init__(
        self,
        endpoint: Endpoint,
    ) -> None:
        """Initialize the LiteLLM enricher.

        Args:
            endpoint: The endpoint configuration containing all settings.

        """
        self.log = structlog.get_logger(__name__)
        self.model_name = endpoint.model or "gpt-4o-mini"
        self.api_key = endpoint.api_key
        self.base_url = endpoint.base_url
        self.socket_path = endpoint.socket_path
        self.num_parallel_tasks = (
            endpoint.num_parallel_tasks or DEFAULT_NUM_PARALLEL_TASKS
        )
        self.timeout = endpoint.timeout
        self.extra_params = endpoint.extra_params or {}

        self._setup_litellm_client()

    def _setup_litellm_client(self) -> None:
        """Set up LiteLLM with custom HTTPX client for Unix socket support."""
        if self.socket_path:
            transport = httpx.AsyncHTTPTransport(uds=self.socket_path)
            unix_client = httpx.AsyncClient(
                transport=transport,
                base_url="http://localhost",
                timeout=self.timeout,
            )
            litellm.aclient_session = unix_client

    async def _call_chat_completion(self, messages: list[dict[str, str]]) -> Any:
        """Call the chat completion API using LiteLLM.

        Args:
            messages: The messages to send to the API.

        Returns:
            The API response as a dictionary.

        """
        kwargs = {
            "model": self.model_name,
            "messages": messages,
            "timeout": self.timeout,
        }

        if self.api_key:
            kwargs["api_key"] = self.api_key

        if self.base_url:
            kwargs["api_base"] = self.base_url

        kwargs.update(self.extra_params)

        try:
            response = await acompletion(**kwargs)
            self.log.debug("enrichment request", request=kwargs, response=response)
            return (
                response.model_dump() if hasattr(response, "model_dump") else response
            )
        except Exception as e:
            self.log.exception(
                "LiteLLM completion API error", error=str(e), model=self.model_name
            )
            raise

    async def enrich(
        self, requests: list[GenericEnrichmentRequest]
    ) -> AsyncGenerator[GenericEnrichmentResponse, None]:
        """Enrich a list of requests using LiteLLM.

        Args:
            requests: List of generic enrichment requests.

        Yields:
            Generic enrichment responses as they are processed.

        """
        if not requests:
            self.log.warning("No requests for enrichment")
            return

        sem = asyncio.Semaphore(self.num_parallel_tasks)

        async def process_request(
            request: GenericEnrichmentRequest,
        ) -> GenericEnrichmentResponse:
            async with sem:
                if not request.text:
                    return GenericEnrichmentResponse(
                        id=request.id,
                        text="",
                    )
                messages = [
                    {
                        "role": "system",
                        "content": request.system_prompt,
                    },
                    {"role": "user", "content": request.text},
                ]
                response = await self._call_chat_completion(messages)
                content = (
                    response.get("choices", [{}])[0]
                    .get("message", {})
                    .get("content", "")
                )
                cleaned_content = clean_thinking_tags(content or "")
                return GenericEnrichmentResponse(
                    id=request.id,
                    text=cleaned_content,
                )

        tasks = [process_request(request) for request in requests]

        for task in asyncio.as_completed(tasks):
            yield await task

    async def close(self) -> None:
        """Close the enricher and cleanup HTTPX client if using Unix sockets."""
        if (
            self.socket_path
            and hasattr(litellm, "aclient_session")
            and litellm.aclient_session
        ):
            await litellm.aclient_session.aclose()
            litellm.aclient_session = None
