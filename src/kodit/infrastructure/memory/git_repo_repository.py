"""In-memory implementation of GitRepoRepository."""

from pydantic import AnyUrl

from kodit.domain.entities import GitRepo
from kodit.domain.protocols import GitRepoRepository


class InMemoryGitRepoRepository(GitRepoRepository):
    """In-memory repository for GitRepo entities."""

    def __init__(self) -> None:
        """Initialize the in-memory repository."""
        self._repos: dict[str, GitRepo] = {}

    async def save(self, repo: GitRepo) -> GitRepo:
        """Save a GitRepo aggregate (insert or update)."""
        key = str(repo.sanitized_remote_uri)
        self._repos[key] = repo
        return repo

    async def get_by_uri(self, sanitized_remote_uri: AnyUrl) -> GitRepo | None:
        """Get a GitRepo by sanitized remote URI."""
        return self._repos.get(str(sanitized_remote_uri))

    async def delete(self, repo: GitRepo) -> None:
        """Delete a GitRepo aggregate."""
        key = str(repo.sanitized_remote_uri)
        if key in self._repos:
            del self._repos[key]

    async def list(self) -> list[GitRepo]:
        """List all GitRepo entities."""
        return list(self._repos.values())
