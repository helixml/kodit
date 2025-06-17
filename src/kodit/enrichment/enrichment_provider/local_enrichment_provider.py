"""Local embedding service."""

import os
from collections.abc import AsyncGenerator

import structlog
import tiktoken

from kodit.embedding.embedding_provider.embedding_provider import (
    EmbeddingRequest,
)
from kodit.enrichment.enrichment_provider.enrichment_provider import (
    ENRICHMENT_SYSTEM_PROMPT,
    EnrichmentProvider,
    EnrichmentRequest,
    EnrichmentResponse,
)

DEFAULT_ENRICHMENT_MODEL = "Qwen/Qwen3-0.6B"
DEFAULT_CONTEXT_WINDOW_SIZE = 2048  # Small so it works even on low-powered devices


class LocalEnrichmentProvider(EnrichmentProvider):
    """Local embedder."""

    def __init__(
        self,
        model_name: str = DEFAULT_ENRICHMENT_MODEL,
        context_window: int = DEFAULT_CONTEXT_WINDOW_SIZE,
    ) -> None:
        """Initialize the local enrichment provider."""
        self.log = structlog.get_logger(__name__)
        self.model_name = model_name
        self.context_window = context_window
        self.model = None
        self.tokenizer = None
        self.encoding = tiktoken.encoding_for_model("text-embedding-3-small")

    async def enrich(
        self, data: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of strings."""
        # Remove empty snippets
        data = [snippet for snippet in data if snippet.text]

        if not data or len(data) == 0:
            self.log.warning("Data is empty, skipping enrichment")
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
        prompts: list[EmbeddingRequest] = [
            EmbeddingRequest(
                id=snippet.snippet_id,
                text=self.tokenizer.apply_chat_template(
                    [
                        {"role": "system", "content": ENRICHMENT_SYSTEM_PROMPT},
                        {"role": "user", "content": snippet.text},
                    ],
                    tokenize=False,
                    add_generation_prompt=True,
                    enable_thinking=False,
                ),
            )
            for snippet in data
        ]

        for prompt in prompts:
            model_inputs = self.tokenizer(
                prompt.text,
                return_tensors="pt",
                padding=True,
                truncation=True,
            ).to(self.model.device)
            generated_ids = self.model.generate(
                **model_inputs, max_new_tokens=self.context_window
            )
            input_ids = model_inputs["input_ids"][0]
            output_ids = generated_ids[0][len(input_ids) :].tolist()
            content = self.tokenizer.decode(output_ids, skip_special_tokens=True).strip(
                "\n"
            )
            yield EnrichmentResponse(
                snippet_id=prompt.id,
                text=content,
            )
