from kodit.domain.entities import SnippetV2
from kodit.domain.protocols import SnippetRepositoryV2


class InMemorySnippetRepository(SnippetRepositoryV2):
    """Simple in-memory implementation of SnippetRepository."""

    def __init__(self) -> None:
        self._snippets: dict[
            str, list[SnippetV2]
        ] = {}  # f"{repo_uri}:{commit_sha}" -> List[Snippet]

    async def save_snippets(self, commit_sha: str, snippets: list[SnippetV2]) -> None:
        self._snippets[commit_sha] = snippets.copy()

    async def get_snippets_for_commit(self, commit_sha: str) -> list[SnippetV2]:
        return self._snippets.get(commit_sha, [])
