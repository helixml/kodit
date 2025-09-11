"""In-memory implementation of CommitIndexRepository."""

from datetime import UTC, datetime

from kodit.domain.entities import CommitIndex


class InMemoryCommitIndexRepository:
    """In-memory implementation of CommitIndexRepository."""

    def __init__(self) -> None:
        """Initialize the repository."""
        self._storage: dict[str, CommitIndex] = {}

    async def save(self, commit_index: CommitIndex) -> CommitIndex:
        """Save a CommitIndex aggregate (insert or update)."""
        now = datetime.now(UTC)

        if commit_index.created_at is None:
            # Insert new commit index
            commit_index.created_at = now
            commit_index.updated_at = now
        else:
            # Update existing commit index
            commit_index.updated_at = now

        self._storage[commit_index.commit_sha] = commit_index
        return commit_index

    async def get_by_commit_sha(self, commit_sha: str) -> CommitIndex | None:
        """Get a CommitIndex by commit SHA."""
        return self._storage.get(commit_sha)

    async def delete(self, commit_index: CommitIndex) -> None:
        """Delete a CommitIndex aggregate."""
        if commit_index.commit_sha in self._storage:
            del self._storage[commit_index.commit_sha]
