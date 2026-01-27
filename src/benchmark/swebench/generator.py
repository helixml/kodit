"""LLM patch generation for SWE-bench benchmarking."""

import re

import structlog
from litellm import completion

from benchmark.swebench.instance import SWEBenchInstance
from benchmark.swebench.prediction import Prediction
from benchmark.swebench.prompt import PromptBuilder
from benchmark.swebench.retriever import KoditRetriever, RetrievedSnippet


class PatchGenerator:
    """Generates patches using an LLM for SWE-bench instances."""

    def __init__(
        self,
        model: str = "claude-3-5-sonnet-20241022",
        max_tokens: int = 32768,
        temperature: float = 0.0,
        api_key: str | None = None,
    ) -> None:
        """Initialize generator with LLM settings."""
        self._model = model
        self._max_tokens = max_tokens
        self._temperature = temperature
        self._api_key = api_key
        self._prompt_builder = PromptBuilder()
        self._log = structlog.get_logger(__name__)

    def generate_baseline(self, instance: SWEBenchInstance) -> Prediction:
        """Generate patch with only the problem statement (no retrieval)."""
        self._log.info(
            "Generating baseline prediction",
            instance_id=instance.instance_id,
        )

        prompt = self._prompt_builder.baseline(instance)
        patch = self._call_llm(prompt)
        patch = self._extract_patch(patch)

        return Prediction(
            instance_id=instance.instance_id,
            model_name_or_path=f"baseline-{self._model}",
            model_patch=patch,
        )

    def generate_with_context(
        self,
        instance: SWEBenchInstance,
        snippets: list[RetrievedSnippet],
    ) -> Prediction:
        """Generate patch with retrieved code context."""
        self._log.info(
            "Generating prediction with context",
            instance_id=instance.instance_id,
            snippet_count=len(snippets),
        )

        prompt = self._prompt_builder.with_context(instance, snippets)
        patch = self._call_llm(prompt)
        patch = self._extract_patch(patch)

        return Prediction(
            instance_id=instance.instance_id,
            model_name_or_path=f"kodit-{self._model}",
            model_patch=patch,
        )

    def _call_llm(self, prompt: str) -> str:
        """Call the LLM and return the response."""
        response = completion(
            model=self._model,
            messages=[{"role": "user", "content": prompt}],
            max_tokens=self._max_tokens,
            temperature=self._temperature,
            api_key=self._api_key,
        )

        choice = response.choices[0]
        if choice.finish_reason == "length":
            self._log.warning(
                "Response truncated due to max_tokens limit",
                model=self._model,
                max_tokens=self._max_tokens,
            )

        return choice.message.content or ""

    def _extract_patch(self, response: str) -> str:
        """Extract the patch from the LLM response."""
        # First try to find a diff block in code fences
        code_block_pattern = r"```(?:diff)?\n(diff --git.*?)```"
        match = re.search(code_block_pattern, response, re.DOTALL)
        if match:
            patch = match.group(1).strip()
            self._validate_diff(patch)
            return patch

        # Otherwise, find content starting with 'diff --git'
        diff_pattern = r"(diff --git.*)"
        match = re.search(diff_pattern, response, re.DOTALL)
        if match:
            patch = match.group(1).strip()
            self._validate_diff(patch)
            return patch

        # If no diff found, return the whole response (may fail evaluation)
        self._log.warning(
            "Could not extract patch from response",
            response_preview=response[:200],
        )
        return response.strip()

    def _validate_diff(self, patch: str) -> None:
        """Log warnings if the diff appears malformed."""
        has_file_headers = "--- " in patch and "+++ " in patch
        has_hunk_header = bool(re.search(r"@@ .+ @@", patch))

        if not has_file_headers:
            self._log.warning(
                "Patch missing file headers (--- / +++)",
                patch_preview=patch[:200],
            )

        if not has_hunk_header:
            self._log.warning(
                "Patch missing hunk header (@@ ... @@)",
                patch_preview=patch[:200],
            )


class BenchmarkRunner:
    """Runs benchmark predictions for SWE-bench instances."""

    def __init__(
        self,
        generator: PatchGenerator,
        retriever: KoditRetriever | None = None,
    ) -> None:
        """Initialize runner with generator and optional retriever."""
        self._generator = generator
        self._retriever = retriever
        self._log = structlog.get_logger(__name__)

    def run_baseline(self, instance: SWEBenchInstance) -> Prediction:
        """Run baseline prediction (no retrieval)."""
        return self._generator.generate_baseline(instance)

    def run_kodit(
        self,
        instance: SWEBenchInstance,
        top_k: int = 10,
    ) -> Prediction:
        """Run Kodit-augmented prediction."""
        if self._retriever is None:
            msg = "Retriever required for Kodit condition"
            raise ValueError(msg)

        snippets = self._retriever.retrieve(instance, top_k=top_k)
        return self._generator.generate_with_context(instance, snippets)
