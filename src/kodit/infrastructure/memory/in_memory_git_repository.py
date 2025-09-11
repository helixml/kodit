from pydantic import AnyUrl

from kodit.domain.entities import GitBranch, GitCommit, GitRepo
from kodit.domain.protocols import (
    GitBranchRepository,
    GitCommitRepository,
    GitRepoRepository,
)


class InMemoryGitRepoRepository(GitRepoRepository):
    """Simple in-memory implementation of GitRepoRepository."""

    def __init__(self) -> None:
        self._repos: dict[str, GitRepo] = {}

    async def save(self, repo: GitRepo) -> None:
        """Save or update a repository."""
        self._repos[str(repo.sanitized_remote_uri)] = repo

    async def get_by_uri(self, sanitized_uri: AnyUrl) -> GitRepo | None:
        """Get repository by sanitized URI."""
        return self._repos.get(str(sanitized_uri))

    async def get_all(self) -> list[GitRepo]:
        """Get all repositories."""
        return list(self._repos.values())

    async def delete(self, sanitized_uri: AnyUrl) -> bool:
        """Delete a repository."""
        key = str(sanitized_uri)
        if key in self._repos:
            del self._repos[key]
            return True
        return False


class InMemoryGitCommitRepository(GitCommitRepository):
    """In-memory implementation with proper branch commit traversal."""

    def __init__(self, branch_repository: GitBranchRepository) -> None:
        self._commits: dict[str, list[GitCommit]] = {}
        self._commit_lookup: dict[str, dict[str, GitCommit]] = {}
        self._branch_repository = branch_repository

    async def save_commits(self, repo_uri: AnyUrl, commits: list[GitCommit]) -> None:
        """Batch save commits for a repository."""
        repo_key = str(repo_uri)

        if repo_key not in self._commits:
            self._commits[repo_key] = []
            self._commit_lookup[repo_key] = {}

        for commit in commits:
            if commit.commit_sha not in self._commit_lookup[repo_key]:
                self._commits[repo_key].append(commit)
                self._commit_lookup[repo_key][commit.commit_sha] = commit

    async def get_commits_for_branch(
        self, repo_uri: AnyUrl, branch_name: str
    ) -> list[GitCommit]:
        """Get commits for a specific branch by traversing from head commit."""
        repo_key = str(repo_uri)

        # Get the branch to find head commit
        branches = await self._branch_repository.get_branches_for_repo(repo_uri)
        target_branch = next((b for b in branches if b.name == branch_name), None)

        if not target_branch or repo_key not in self._commit_lookup:
            return []

        # Traverse commit history from head
        result = []
        current_sha = target_branch.head_commit.commit_sha
        commit_lookup = self._commit_lookup[repo_key]

        while current_sha and current_sha in commit_lookup:
            commit = commit_lookup[current_sha]
            result.append(commit)
            current_sha = commit.parent_commit_sha

        return result


class InMemoryGitBranchRepository(GitBranchRepository):
    """Simple in-memory implementation of GitBranchRepository."""

    def __init__(self) -> None:
        self._branches: dict[str, list[GitBranch]] = {}  # repo_uri -> list of branches

    async def save_branches(self, repo_uri: AnyUrl, branches: list[GitBranch]) -> None:
        """Save branches for a repository."""
        self._branches[str(repo_uri)] = branches.copy()

    async def get_branches_for_repo(self, repo_uri: AnyUrl) -> list[GitBranch]:
        """Get all branches for a repository."""
        return self._branches.get(str(repo_uri), [])
