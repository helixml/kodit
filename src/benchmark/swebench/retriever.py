"""Kodit retrieval wrapper for SWE-bench benchmarking."""

from dataclasses import dataclass

import httpx
import structlog

from benchmark.swebench.instance import SWEBenchInstance


@dataclass(frozen=True)
class RetrievedSnippet:
    """A code snippet retrieved from Kodit."""

    file_path: str
    content: str
    scores: list[float]


class KoditRetriever:
    """Retrieves relevant code snippets from Kodit for SWE-bench instances."""

    def __init__(
        self,
        kodit_base_url: str,
        timeout: float = 30.0,
    ) -> None:
        """Initialize with Kodit server URL."""
        self._base_url = kodit_base_url.rstrip("/")
        self._timeout = timeout
        self._log = structlog.get_logger(__name__)

    def retrieve(
        self,
        instance: SWEBenchInstance,
        top_k: int = 10,
    ) -> list[RetrievedSnippet]:
        """Retrieve relevant snippets for a SWE-bench instance."""
        repo_url = f"github.com/{instance.repo}"

        self._log.info(
            "Retrieving snippets",
            instance_id=instance.instance_id,
            repo_url=repo_url,
            top_k=top_k,
        )

        snippets = self._search(
            query=instance.problem_statement,
            repo_url=repo_url,
            limit=top_k,
        )

        self._log.info(
            "Retrieved snippets",
            instance_id=instance.instance_id,
            count=len(snippets),
        )

        return snippets

    def _search(
        self,
        query: str,
        repo_url: str,
        limit: int,
    ) -> list[RetrievedSnippet]:
        """Execute a search query."""
        payload = {
            "data": {
                "type": "search",
                "attributes": {
                    "text": query,
                    "limit": limit,
                    "repo_url": repo_url,
                },
            }
        }

        url = f"{self._base_url}/api/v1/search"
        response = httpx.post(url, json=payload, timeout=self._timeout)

        if response.status_code != 200:
            self._log.error(
                "Search request failed",
                status_code=response.status_code,
                response=response.text[:500],
            )
            return []

        data = response.json()
        return self._parse_results(data)

    def _parse_results(self, data: dict) -> list[RetrievedSnippet]:
        """Parse search results into RetrievedSnippet objects."""
        snippets = []

        for item in data.get("data", []):
            attributes = item.get("attributes", {})

            # Get file path from derives_from
            derives_from = attributes.get("derives_from", [])
            file_path = derives_from[0]["path"] if derives_from else "unknown"

            # Get content
            content_obj = attributes.get("content", {})
            content = content_obj.get("value", "")

            # Get scores
            scores = attributes.get("original_scores", [])

            snippet = RetrievedSnippet(
                file_path=file_path,
                content=content,
                scores=scores,
            )
            snippets.append(snippet)

        return snippets


class RetrievalError(Exception):
    """Raised when retrieval fails."""
