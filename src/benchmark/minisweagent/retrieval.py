"""Kodit retrieval integration for mini-swe-agent benchmarks."""

import structlog

from benchmark.swebench.instance import SWEBenchInstance
from benchmark.swebench.retriever import KoditRetriever, RetrievedSnippet


class NoSnippetsRetrievedError(Exception):
    """Raised when no snippets are retrieved from Kodit."""


class KoditContextProvider:
    """Pre-fetches Kodit snippets and formats them for mini-swe-agent."""

    def __init__(
        self,
        kodit_base_url: str,
        top_k: int = 10,
        timeout: float = 30.0,
    ) -> None:
        """Initialize with Kodit server URL."""
        self._retriever = KoditRetriever(
            kodit_base_url=kodit_base_url,
            timeout=timeout,
        )
        self._top_k = top_k
        self._log = structlog.get_logger(__name__)

    def get_context(self, instance: SWEBenchInstance) -> str:
        """Retrieve and format Kodit snippets for an instance."""
        self._log.info(
            "Fetching Kodit context",
            instance_id=instance.instance_id,
            top_k=self._top_k,
        )

        snippets = self._retriever.retrieve(instance, top_k=self._top_k)

        if not snippets:
            self._log.error(
                "No snippets retrieved - indexing may not be complete",
                instance_id=instance.instance_id,
            )
            raise NoSnippetsRetrievedError(
                f"No snippets retrieved for instance {instance.instance_id}. "
                "This likely indicates that indexing is not complete or failed."
            )

        self._log.info(
            "Retrieved snippets",
            instance_id=instance.instance_id,
            count=len(snippets),
        )

        return self._format_snippets(snippets)

    def _format_snippets(self, snippets: list[RetrievedSnippet]) -> str:
        """Format snippets as markdown code blocks."""
        parts = []
        for snippet in snippets:
            parts.append(f"### {snippet.file_path}")
            parts.append("```")
            parts.append(snippet.content)
            parts.append("```")
            parts.append("")

        return "\n".join(parts)

    def augment_problem_statement(
        self,
        instance: SWEBenchInstance,
    ) -> str:
        """Augment the problem statement with Kodit-retrieved context."""
        context = self.get_context(instance)

        if not context:
            return instance.problem_statement

        return f"""## Relevant Code Context
The following code snippets from the repository may be relevant to this issue:

{context}

## Issue Description
{instance.problem_statement}"""
