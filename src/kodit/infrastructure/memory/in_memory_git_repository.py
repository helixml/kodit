"""In-memory implementation of Git repository with aggregate root pattern.

This module provides an in-memory implementation of GitRepoRepository that treats
GitRepo as the aggregate root owning branches, commits, and tags.
"""

from pydantic import AnyUrl

from kodit.domain.entities import GitBranch, GitCommit, GitRepo, GitTag
from kodit.domain.protocols import GitRepoRepository


class InMemoryGitRepoRepository(GitRepoRepository):
    """In-memory implementation of GitRepoRepository.

    This repository treats GitRepo as the aggregate root and handles
    persistence of the entire aggregate including branches, commits, and tags.
    """

    def __init__(self) -> None:
        """Initialize the in-memory Git repository."""
        self._repos: dict[int, GitRepo] = {}
        self._repos_by_uri: dict[str, int] = {}  # URI -> repo_id mapping
        self._next_id = 1
        # Internal storage for aggregate components
        self._commits: dict[str, list[GitCommit]] = {}
        self._branches: dict[str, list[GitBranch]] = {}
        self._tags: dict[str, list[GitTag]] = {}

    async def save(self, repo: GitRepo) -> None:
        """Save or update a repository with all its branches, commits, and tags."""
        # Assign ID if new repo
        if repo.id is None:
            repo.id = self._next_id
            self._next_id += 1

        self._repos[repo.id] = repo
        self._repos_by_uri[str(repo.sanitized_remote_uri)] = repo.id

        # Store commits
        repo_key = str(repo.sanitized_remote_uri)
        self._commits[repo_key] = repo.commits

        # Store branches
        self._branches[repo_key] = repo.branches

        # Store tags
        self._tags[repo_key] = repo.tags

    async def get_by_id(self, repo_id: int) -> GitRepo | None:
        """Get repository by ID."""
        return self._repos.get(repo_id)

    async def get_by_uri(self, sanitized_uri: AnyUrl) -> GitRepo | None:
        """Get repository by sanitized URI with all associated data."""
        uri_str = str(sanitized_uri)
        repo_id = self._repos_by_uri.get(uri_str)
        if not repo_id:
            return None

        repo = self._repos.get(repo_id)
        if repo:
            # Ensure repo has all its aggregate data
            repo_key = uri_str
            if repo_key in self._commits:
                repo.commits = self._commits[repo_key]
            if repo_key in self._branches:
                repo.branches = self._branches[repo_key]
            if repo_key in self._tags:
                repo.tags = self._tags[repo_key]

        return repo

    async def get_by_commit(self, commit_sha: str) -> GitRepo | None:
        """Get repository by commit SHA."""
        for repo in self._repos.values():
            for commit in repo.commits:
                if commit.commit_sha == commit_sha:
                    return repo
        return None

    async def get_all(self) -> list[GitRepo]:
        """Get all repositories."""
        return list(self._repos.values())

    async def delete(self, sanitized_uri: AnyUrl) -> bool:
        """Delete a repository and all its associated data."""
        uri_str = str(sanitized_uri)
        repo_id = self._repos_by_uri.get(uri_str)
        if not repo_id:
            return False

        # Delete the repo
        del self._repos[repo_id]
        del self._repos_by_uri[uri_str]

        # Delete associated aggregate data
        if uri_str in self._commits:
            del self._commits[uri_str]
        if uri_str in self._branches:
            del self._branches[uri_str]
        if uri_str in self._tags:
            del self._tags[uri_str]

        return True

    async def get_commit_by_sha(self, commit_sha: str) -> GitCommit | None:
        """Get a specific commit by its SHA across all repositories."""
        # Search through all repositories' commits
        for repo in self._repos.values():
            for commit in repo.commits:
                if commit.commit_sha == commit_sha:
                    return commit

        # Also search in the internal commit storage
        for commits_list in self._commits.values():
            for commit in commits_list:
                if commit.commit_sha == commit_sha:
                    return commit

        return None
