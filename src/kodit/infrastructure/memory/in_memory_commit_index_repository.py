from kodit.domain.entities import CommitIndex, IndexStatus
from kodit.domain.protocols import CommitIndexRepository


class InMemoryCommitIndexRepository(CommitIndexRepository):
    """Simple in-memory implementation of CommitIndexRepository."""

    def __init__(self):
        self._indexes: dict[
            str, CommitIndex
        ] = {}  # f"{repo_uri}:{commit_sha}" -> CommitIndex

    def _make_key(self, repo_uri: str, commit_sha: str) -> str:
        return f"{repo_uri}:{commit_sha}"

    async def save(self, commit_index: CommitIndex) -> None:
        key = self._make_key(commit_index.repo_uri, commit_index.commit_sha)
        self._indexes[key] = commit_index

    async def get_by_commit(self, repo_uri: str, commit_sha: str) -> CommitIndex | None:
        key = self._make_key(repo_uri, commit_sha)
        return self._indexes.get(key)

    async def get_indexed_commits_for_repo(self, repo_uri: str) -> list[CommitIndex]:
        return [idx for idx in self._indexes.values() if idx.repo_uri == repo_uri]

    async def delete(self, repo_uri: str, commit_sha: str) -> bool:
        key = self._make_key(repo_uri, commit_sha)
        if key in self._indexes:
            del self._indexes[key]
            return True
        return False

    async def is_indexed(self, repo_uri: str, commit_sha: str) -> bool:
        index = await self.get_by_commit(repo_uri, commit_sha)
        return index is not None and index.status == IndexStatus.COMPLETED
