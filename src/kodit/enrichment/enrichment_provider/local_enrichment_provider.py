"""Local embedding service."""

import os

import structlog
from transformers.models.auto.modeling_auto import AutoModelForCausalLM
from transformers.models.auto.tokenization_auto import AutoTokenizer

from kodit.enrichment.enrichment_provider.enrichment_provider import (
    ENRICHMENT_SYSTEM_PROMPT,
    EnrichmentProvider,
)


class LocalEnrichmentProvider(EnrichmentProvider):
    """Local embedder."""

    def __init__(self, model_name: str = "Qwen/Qwen3-0.6B") -> None:
        """Initialize the local enrichment provider."""
        self.log = structlog.get_logger(__name__)
        self.model_name = model_name
        self.model = None
        self.tokenizer = None

    async def enrich(self, data: list[str]) -> list[str]:
        """Enrich a list of strings."""
        if self.tokenizer is None:
            self.tokenizer = AutoTokenizer.from_pretrained(self.model_name)
        if self.model is None:
            os.environ["TOKENIZERS_PARALLELISM"] = "false"  # Avoid warnings
            self.model = AutoModelForCausalLM.from_pretrained(
                self.model_name,
                torch_dtype="auto",
                trust_remote_code=True,
            )

        results = []
        for snippet in data:
            # prepare the model input
            messages = [
                {"role": "system", "content": ENRICHMENT_SYSTEM_PROMPT},
                {"role": "user", "content": snippet},
            ]
            text = self.tokenizer.apply_chat_template(
                messages,
                tokenize=False,
                add_generation_prompt=True,
                enable_thinking=False,
            )
            model_inputs = self.tokenizer([text], return_tensors="pt").to(
                self.model.device
            )

            # conduct text completion
            generated_ids = self.model.generate(**model_inputs, max_new_tokens=32768)
            output_ids = generated_ids[0][len(model_inputs.input_ids[0]) :].tolist()
            content = self.tokenizer.decode(output_ids, skip_special_tokens=True).strip(
                "\n"
            )

            results.append(content)

        return results
