"""In-memory implementation of SnippetRepositoryV2."""

from kodit.domain.entities import SnippetV2
from kodit.domain.protocols import SnippetRepositoryV2


class InMemorySnippetRepository(SnippetRepositoryV2):
    """Simple in-memory implementation of SnippetRepository."""

    def __init__(self) -> None:
        """Initialize repository."""
        self._snippets: dict[
            str, list[SnippetV2]
        ] = {}  # f"{repo_uri}:{commit_sha}" -> List[Snippet]

    async def save_snippets(self, commit_sha: str, snippets: list[SnippetV2]) -> None:
        """Save snippets for a commit."""
        self._snippets[commit_sha] = snippets.copy()

    async def get_snippets_for_commit(self, commit_sha: str) -> list[SnippetV2]:
        """Get snippets for a commit."""
        return self._snippets.get(commit_sha, [])
