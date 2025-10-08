"""Local enricher implementation."""

import asyncio
import os
from collections.abc import AsyncGenerator
from typing import Any

import structlog
import tiktoken

from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.request import EnrichmentRequest
from kodit.domain.enrichments.response import EnrichmentResponse
from kodit.infrastructure.enricher.utils import clean_thinking_tags

DEFAULT_ENRICHER_MODEL = "Qwen/Qwen3-0.6B"
DEFAULT_CONTEXT_WINDOW_SIZE = 2048


class LocalEnricher(Enricher):
    """Local enricher implementation using local models."""

    def __init__(
        self,
        model_name: str = DEFAULT_ENRICHER_MODEL,
        context_window: int = DEFAULT_CONTEXT_WINDOW_SIZE,
    ) -> None:
        """Initialize the local enricher.

        Args:
            model_name: The model name to use for enrichment.
            context_window: The context window size for the model.

        """
        self.log = structlog.get_logger(__name__)
        self.model_name = model_name
        self.context_window = context_window
        self.model = None
        self.tokenizer = None
        self.encoding = tiktoken.encoding_for_model("text-embedding-3-small")

    async def enrich(
        self, requests: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of requests using local model.

        Args:
            requests: List of generic enrichment requests.

        Yields:
            Generic enrichment responses as they are processed.

        """
        # Remove empty requests
        requests = [req for req in requests if req.text]

        if not requests:
            self.log.warning("No valid requests for enrichment")
            return

        def _init_model() -> None:
            from transformers.models.auto.modeling_auto import (
                AutoModelForCausalLM,
            )
            from transformers.models.auto.tokenization_auto import AutoTokenizer

            if self.tokenizer is None:
                self.tokenizer = AutoTokenizer.from_pretrained(
                    self.model_name, padding_side="left"
                )
            if self.model is None:
                os.environ["TOKENIZERS_PARALLELISM"] = "false"
                self.model = AutoModelForCausalLM.from_pretrained(
                    self.model_name,
                    torch_dtype="auto",
                    trust_remote_code=True,
                    device_map="auto",
                )

        await asyncio.to_thread(_init_model)

        # Prepare prompts with custom system prompts
        prompts = [
            {
                "id": req.id,
                "text": self.tokenizer.apply_chat_template(  # type: ignore[attr-defined]
                    [
                        {"role": "system", "content": req.system_prompt},
                        {"role": "user", "content": req.text},
                    ],
                    tokenize=False,
                    add_generation_prompt=True,
                    enable_thinking=False,
                ),
            }
            for req in requests
        ]

        for prompt in prompts:

            def process_prompt(prompt: dict[str, Any]) -> str:
                model_inputs = self.tokenizer(  # type: ignore[misc]
                    prompt["text"],
                    return_tensors="pt",
                    padding=True,
                    truncation=True,
                ).to(self.model.device)  # type: ignore[attr-defined]
                generated_ids = self.model.generate(  # type: ignore[attr-defined]
                    **model_inputs, max_new_tokens=self.context_window
                )
                input_ids = model_inputs["input_ids"][0]
                output_ids = generated_ids[0][len(input_ids) :].tolist()
                return self.tokenizer.decode(  # type: ignore[attr-defined]
                    output_ids, skip_special_tokens=True
                ).strip(  # type: ignore[attr-defined]
                    "\n"
                )

            content = await asyncio.to_thread(process_prompt, prompt)
            cleaned_content = clean_thinking_tags(content)
            yield EnrichmentResponse(
                id=prompt["id"],
                text=cleaned_content,
            )
