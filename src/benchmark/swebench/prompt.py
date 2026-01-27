"""Prompt templates for SWE-bench patch generation."""

from benchmark.swebench.instance import SWEBenchInstance
from benchmark.swebench.retriever import RetrievedSnippet

# fmt: off
BASELINE_TEMPLATE = (
    "You will be provided with an issue statement explaining a problem "
    "to resolve in a code repository.\n\n"
    "<issue>\n"
    "{problem_statement}\n"
    "</issue>\n\n"
    "Based on the issue description, generate a patch in unified diff format "
    "that resolves the issue.\n"
    "The patch must use the standard git diff format with headers like:\n"
    "diff --git a/path/to/file.py b/path/to/file.py\n\n"
    "Only output the patch, no explanations."
)

KODIT_TEMPLATE = (
    "You will be provided with a partial code base and an issue statement "
    "explaining a problem to resolve.\n\n"
    "<issue>\n"
    "{problem_statement}\n"
    "</issue>\n\n"
    "<code>\n"
    "{code_context}\n"
    "</code>\n\n"
    "Based on the issue description and the provided code context, "
    "generate a patch in unified diff format that resolves the issue.\n"
    "The patch must use the standard git diff format with headers like:\n"
    "diff --git a/path/to/file.py b/path/to/file.py\n\n"
    "Only output the patch, no explanations."
)
# fmt: on


class PromptBuilder:
    """Builds prompts for SWE-bench patch generation."""

    def baseline(self, instance: SWEBenchInstance) -> str:
        """Build baseline prompt with only the problem statement."""
        return BASELINE_TEMPLATE.format(
            problem_statement=instance.problem_statement,
        )

    def with_context(
        self,
        instance: SWEBenchInstance,
        snippets: list[RetrievedSnippet],
    ) -> str:
        """Build prompt with retrieved code context."""
        code_context = self._format_snippets(snippets)
        return KODIT_TEMPLATE.format(
            problem_statement=instance.problem_statement,
            code_context=code_context,
        )

    def _format_snippets(self, snippets: list[RetrievedSnippet]) -> str:
        """Format snippets as code blocks."""
        if not snippets:
            return "(No code context available)"

        parts = []
        for snippet in snippets:
            parts.append(f"[start of {snippet.file_path}]")
            parts.append(snippet.content)
            parts.append(f"[end of {snippet.file_path}]")
            parts.append("")  # blank line between files

        return "\n".join(parts)
