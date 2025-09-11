from kodit.domain.entities import SnippetV2
from kodit.domain.protocols import SnippetRepositoryV2


class InMemorySnippetRepository(SnippetRepositoryV2):
    """Simple in-memory implementation of SnippetRepository."""

    def __init__(self):
        self._snippets: dict[
            str, list[SnippetV2]
        ] = {}  # f"{repo_uri}:{commit_sha}" -> List[Snippet]

    def _make_key(self, repo_uri: str, commit_sha: str) -> str:
        return f"{repo_uri}:{commit_sha}"

    async def save_snippets(
        self, repo_uri: str, commit_sha: str, snippets: list[SnippetV2]
    ) -> None:
        key = self._make_key(repo_uri, commit_sha)
        self._snippets[key] = snippets.copy()

    async def get_snippets_for_commit(
        self, repo_uri: str, commit_sha: str
    ) -> list[SnippetV2]:
        key = self._make_key(repo_uri, commit_sha)
        return self._snippets.get(key, [])
