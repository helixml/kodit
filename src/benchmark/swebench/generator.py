"""LLM patch generation for SWE-bench benchmarking."""

import difflib
import re
from dataclasses import dataclass

import structlog
from litellm import completion

from benchmark.swebench.instance import SWEBenchInstance
from benchmark.swebench.prediction import Prediction
from benchmark.swebench.prompt import PromptBuilder
from benchmark.swebench.retriever import KoditRetriever, RetrievedSnippet


@dataclass
class SearchReplace:
    """A single search/replace operation."""

    file_path: str
    search: str
    replace: str


class PatchGenerator:
    """Generates patches using an LLM for SWE-bench instances."""

    def __init__(
        self,
        model: str = "claude-3-5-sonnet-20241022",
        max_tokens: int = 32768,
        temperature: float = 0.0,
        api_key: str | None = None,
        timeout: int = 60,
    ) -> None:
        """Initialize generator with LLM settings."""
        self._model = model
        self._max_tokens = max_tokens
        self._temperature = temperature
        self._api_key = api_key
        self._timeout = timeout
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

    def _call_llm(self, prompt: str, retries: int = 3) -> str:
        """Call the LLM and return the response."""
        from httpx import ReadTimeout

        self._log.info(
            "Calling LLM",
            model=self._model,
            timeout=self._timeout,
            prompt_length=len(prompt),
        )

        last_error: Exception | None = None
        for attempt in range(retries):
            try:
                response = completion(
                    model=self._model,
                    messages=[{"role": "user", "content": prompt}],
                    max_tokens=self._max_tokens,
                    temperature=self._temperature,
                    api_key=self._api_key,
                    timeout=float(self._timeout),
                )
            except ReadTimeout as e:
                last_error = e
                self._log.warning(
                    "LLM request timed out, retrying",
                    attempt=attempt + 1,
                    retries=retries,
                    error=str(e),
                )
                continue

            choice = response.choices[0]
            if choice.finish_reason == "length":
                self._log.warning(
                    "Response truncated due to max_tokens limit",
                    model=self._model,
                    max_tokens=self._max_tokens,
                )

            return choice.message.content or ""

        msg = f"LLM request failed after {retries} retries"
        raise TimeoutError(msg) from last_error

    def _extract_patch(self, response: str) -> str:
        """Extract and convert patch from the LLM response."""
        # First try search/replace format
        replacements = self._parse_search_replace(response)
        if replacements:
            self._log.info(
                "Found search/replace blocks",
                count=len(replacements),
            )
            return self._convert_to_diff(replacements)

        # Fall back to diff extraction
        code_block_pattern = r"```(?:diff)?\n(diff --git.*?)```"
        match = re.search(code_block_pattern, response, re.DOTALL)
        if match:
            patch = match.group(1).strip()
            patch = self._normalize_patch(patch)
            self._validate_diff(patch)
            return patch

        diff_pattern = r"(diff --git.*)"
        match = re.search(diff_pattern, response, re.DOTALL)
        if match:
            patch = match.group(1).strip()
            patch = self._normalize_patch(patch)
            self._validate_diff(patch)
            return patch

        self._log.warning(
            "Could not extract patch from response",
            response_preview=response[:200],
        )
        return response.strip()

    def _parse_search_replace(self, response: str) -> list[SearchReplace]:
        """Parse search/replace blocks from response."""
        results = []
        current_file = None

        # Find file markers
        file_pattern = r"\[file:\s*([^\]]+)\]"

        # Find search/replace blocks
        block_pattern = (
            r"<<<<<<+\s*SEARCH\s*\n(.*?)\n======+\s*\n(.*?)\n>>>>>>+\s*REPLACE"
        )

        # Split by file markers
        parts = re.split(file_pattern, response)

        for i, part in enumerate(parts):
            # Odd indices are file paths from the split
            if i % 2 == 1:
                current_file = part.strip()
                continue

            # Even indices are content between file markers
            if current_file is None:
                # Try to find file path in the content itself
                file_match = re.search(r"^\s*([^\s]+\.py)\s*$", part, re.MULTILINE)
                if file_match:
                    current_file = file_match.group(1)

            # Find all search/replace blocks in this part
            for match in re.finditer(block_pattern, part, re.DOTALL):
                search_text = match.group(1)
                replace_text = match.group(2)

                if current_file:
                    results.append(
                        SearchReplace(
                            file_path=current_file,
                            search=search_text,
                            replace=replace_text,
                        )
                    )

        return results

    def _convert_to_diff(self, replacements: list[SearchReplace]) -> str:
        """Convert search/replace blocks to unified diff format."""
        diffs = []

        # Group by file
        by_file: dict[str, list[SearchReplace]] = {}
        for r in replacements:
            by_file.setdefault(r.file_path, []).append(r)

        for file_path, file_replacements in by_file.items():
            # Create a mock "original" content by concatenating search blocks
            # and apply replacements to generate "new" content
            # This is a simplified approach - we generate diff from search->replace

            for replacement in file_replacements:
                old_lines = replacement.search.splitlines(keepends=True)
                new_lines = replacement.replace.splitlines(keepends=True)

                # Ensure lines end with newline
                if old_lines and not old_lines[-1].endswith("\n"):
                    old_lines[-1] += "\n"
                if new_lines and not new_lines[-1].endswith("\n"):
                    new_lines[-1] += "\n"

                # Generate unified diff
                diff = list(
                    difflib.unified_diff(
                        old_lines,
                        new_lines,
                        fromfile=f"a/{file_path}",
                        tofile=f"b/{file_path}",
                    )
                )

                if diff:
                    # Replace the standard header with git-style header
                    diff_text = "".join(diff)
                    diff_text = f"diff --git a/{file_path} b/{file_path}\n{diff_text}"
                    diffs.append(diff_text)

        result = "\n".join(diffs)
        if result and not result.endswith("\n"):
            result += "\n"
        return result

    def _normalize_patch(self, patch: str) -> str:
        """Normalize patch to handle common issues."""
        # Split multiple diffs and keep only the first complete one
        parts = re.split(r"(?=diff --git)", patch)
        valid_parts = [p for p in parts if p.strip()]

        if len(valid_parts) > 1:
            self._log.warning(
                "Multiple diffs found, using first valid one",
                diff_count=len(valid_parts),
            )
            for part in valid_parts:
                if "--- " in part and "+++ " in part and "@@ " in part:
                    patch = part.strip()
                    break
            else:
                patch = valid_parts[0].strip()

        # Fix missing space prefixes on context lines within hunks
        patch = self._fix_context_lines(patch)

        # Ensure patch ends with newline
        if patch and not patch.endswith("\n"):
            patch += "\n"
        return patch

    def _fix_context_lines(self, patch: str) -> str:
        """Add missing space prefixes to context lines in hunks."""
        lines = patch.split("\n")
        result = []
        in_hunk = False

        for line in lines:
            # Detect hunk start
            if line.startswith("@@") and "@@" in line[2:]:
                in_hunk = True
                result.append(line)
                continue

            # Detect headers (not in hunk)
            if line.startswith(("diff --git", "---", "+++")):
                in_hunk = False
                result.append(line)
                continue

            # Inside a hunk - ensure proper prefix
            if in_hunk and line and not line.startswith((" ", "+", "-", "\\")):
                # Missing prefix - add space for context line
                result.append(" " + line)
            else:
                result.append(line)

        return "\n".join(result)

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
