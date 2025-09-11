from kodit.domain.entities import CommitIndex
from kodit.domain.protocols import CommitIndexRepository


class InMemoryCommitIndexRepository(CommitIndexRepository):
    """Simple in-memory implementation of CommitIndexRepository."""

    def __init__(self) -> None:
        self._indexes: dict[
            str, CommitIndex
        ] = {}  # f"{repo_uri}:{commit_sha}" -> CommitIndex

    async def save(self, commit_index: CommitIndex) -> None:
        self._indexes[commit_index.commit_sha] = commit_index

    async def get_by_commit(self, commit_sha: str) -> CommitIndex | None:
        return self._indexes.get(commit_sha)

    async def get_indexed_commits_for_repo(self, repo_uri: str) -> list[CommitIndex]:
        return list(self._indexes.values())

    async def delete(self, commit_sha: str) -> bool:
        if commit_sha in self._indexes:
            del self._indexes[commit_sha]
            return True
        return False
