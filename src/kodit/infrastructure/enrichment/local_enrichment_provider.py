"""Local enrichment provider implementation."""

import os
from collections.abc import AsyncGenerator

import structlog
import tiktoken

from kodit.domain.services.enrichment_service import EnrichmentProvider
from kodit.domain.value_objects import EnrichmentRequest, EnrichmentResponse
from kodit.infrastructure.enrichment.utils import clean_thinking_tags

ENRICHMENT_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""

DEFAULT_ENRICHMENT_MODEL = "Qwen/Qwen3-0.6B"
DEFAULT_CONTEXT_WINDOW_SIZE = 2048  # Small so it works even on low-powered devices


class LocalEnrichmentProvider(EnrichmentProvider):
    """Local enrichment provider implementation."""

    def __init__(
        self,
        model_name: str = DEFAULT_ENRICHMENT_MODEL,
        context_window: int = DEFAULT_CONTEXT_WINDOW_SIZE,
    ) -> None:
        """Initialize the local enrichment provider.

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
            requests: List of enrichment requests.

        Yields:
            Enrichment responses as they are processed.

        """
        # Remove empty snippets
        requests = [req for req in requests if req.text]

        if not requests:
            self.log.warning("No valid requests for enrichment")
            return

        from transformers.models.auto.modeling_auto import (
            AutoModelForCausalLM,
        )
        from transformers.models.auto.tokenization_auto import AutoTokenizer

        if self.tokenizer is None:
            self.tokenizer = AutoTokenizer.from_pretrained(
                self.model_name, padding_side="left"
            )
        if self.model is None:
            os.environ["TOKENIZERS_PARALLELISM"] = "false"  # Avoid warnings
            self.model = AutoModelForCausalLM.from_pretrained(
                self.model_name,
                torch_dtype="auto",
                trust_remote_code=True,
                device_map="auto",
            )

        # Prepare prompts
        prompts = [
            {
                "id": req.snippet_id,
                "text": self.tokenizer.apply_chat_template(  # type: ignore[attr-defined]
                    [
                        {"role": "system", "content": ENRICHMENT_SYSTEM_PROMPT},
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
            content = self.tokenizer.decode(output_ids, skip_special_tokens=True).strip(  # type: ignore[attr-defined]
                "\n"
            )
            # Remove thinking tags from the response
            cleaned_content = clean_thinking_tags(content)
            yield EnrichmentResponse(
                snippet_id=prompt["id"],
                text=cleaned_content,
            )
