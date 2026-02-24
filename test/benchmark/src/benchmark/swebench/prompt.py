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
    "Generate changes using SEARCH/REPLACE blocks. For each file you need to "
    "modify, first specify the file path, then provide SEARCH/REPLACE blocks.\n\n"
    "Format:\n"
    "[file: path/to/file.py]\n"
    "<<<<<<< SEARCH\n"
    "exact code to find (include enough context to be unique)\n"
    "=======\n"
    "replacement code\n"
    ">>>>>>> REPLACE\n\n"
    "Important:\n"
    "- SEARCH must match the existing code EXACTLY (including whitespace)\n"
    "- Include 2-3 lines of context before/after to ensure unique match\n"
    "- You can have multiple SEARCH/REPLACE blocks per file\n\n"
    "Only output the changes, no explanations."
)

KODIT_TEMPLATE = (
    "You will be provided with a partial code base and an issue statement "
    "explaining a problem to resolve.\n\n"
    "<issue>\n"
    "{problem_statement}\n"
    "</issue>\n\n"
    "<available_files>\n"
    "{file_paths}\n"
    "</available_files>\n\n"
    "<code>\n"
    "{code_context}\n"
    "</code>\n\n"
    "Generate changes using SEARCH/REPLACE blocks. Use ONLY file paths from "
    "<available_files>.\n\n"
    "Format:\n"
    "[file: {first_file_path}]\n"
    "<<<<<<< SEARCH\n"
    "exact code to find (include enough context to be unique)\n"
    "=======\n"
    "replacement code\n"
    ">>>>>>> REPLACE\n\n"
    "Important:\n"
    "- SEARCH must match existing code EXACTLY (including whitespace)\n"
    "- Include 2-3 lines of context before/after to ensure unique match\n"
    "- Use exact file paths from <available_files>\n\n"
    "Only output the changes, no explanations."
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
        file_paths_list = self._get_file_paths_list(snippets, instance)
        file_paths = "\n".join(file_paths_list) if file_paths_list else "(No files)"
        first_file_path = file_paths_list[0] if file_paths_list else "path/to/file.py"
        code_context = self._format_snippets(snippets)
        return KODIT_TEMPLATE.format(
            problem_statement=instance.problem_statement,
            file_paths=file_paths,
            first_file_path=first_file_path,
            code_context=code_context,
        )

    def _get_file_paths_list(
        self,
        snippets: list[RetrievedSnippet],
        instance: SWEBenchInstance,
    ) -> list[str]:
        """Extract unique file paths from snippets, with gold patch fallback."""
        paths = []
        seen: set[str] = set()
        for snippet in snippets:
            # Skip "unknown" paths from snippets without derives_from
            path = snippet.file_path
            if path and path != "unknown" and path not in seen:
                paths.append(path)
                seen.add(path)

        # Fallback: extract paths from gold patch if no valid paths from snippets
        if not paths:
            paths = self._extract_paths_from_patch(instance.patch)

        return paths

    def _extract_paths_from_patch(self, patch: str) -> list[str]:
        """Extract file paths from a unified diff patch."""
        import re

        paths = []
        seen: set[str] = set()
        for match in re.finditer(r"diff --git a/(.+?) b/", patch):
            path = match.group(1)
            if path not in seen:
                paths.append(path)
                seen.add(path)
        return paths

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
