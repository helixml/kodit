"""Service for git operations."""

import asyncio
import hashlib
from datetime import UTC, datetime
from pathlib import Path
from typing import TYPE_CHECKING

import git
import structlog
from git import InvalidGitRepositoryError, Repo
from pydantic import AnyUrl

from kodit.application.factories.reporting_factory import create_noop_operation
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities import WorkingCopy
from kodit.domain.entities.git import (
    GitBranch,
    GitCommit,
    GitFile,
    GitRepo,
    GitTag,
)
from kodit.domain.factories.git_repo_factory import GitRepoFactory

if TYPE_CHECKING:
    from git.objects import Commit


class GitService:
    """Service for git operations."""

    def __init__(self, clone_dir: Path) -> None:
        """Initialize the git service."""
        self.clone_dir = clone_dir
        self.log = structlog.get_logger(__name__)

    def get_clone_path(self, uri: str) -> Path:
        """Get the clone path for a Git working copy."""
        sanitized_uri = WorkingCopy.sanitize_git_url(uri)
        dir_hash = hashlib.sha256(str(sanitized_uri).encode("utf-8")).hexdigest()[:16]
        dir_name = f"repo-{dir_hash}"
        return self.clone_dir / dir_name

    async def clone_and_extract_repo_info(
        self, uri: str, step: ProgressTracker | None = None
    ) -> GitRepo:
        """Clone repository and extract complete git repository information."""
        step = step or create_noop_operation()
        # Verify the clone path doesn't already exist
        clone_path = self.get_clone_path(uri)
        if clone_path.exists():
            raise ValueError(f"Clone path already exists: {clone_path}")
        sanitized_uri = WorkingCopy.sanitize_git_url(uri)
        clone_path.mkdir(parents=True, exist_ok=True)

        step_record = []
        await step.set_total(12)

        def _clone_progress_callback(
            a: int, _: str | float | None, __: str | float | None, _d: str
        ) -> None:
            if a not in step_record:
                step_record.append(a)

            # Git reports a really weird format. This is a quick hack to get some
            # progress.
            # Normally this would fail because the loop is already running,
            # but in this case, this callback is called by some git sub-thread.
            asyncio.run(
                step.set_current(
                    len(step_record), f"Cloning repository ({step_record[-1]})"
                )
            )

        try:
            self.log.info(
                "Cloning repository", uri=sanitized_uri, clone_path=str(clone_path)
            )
            # Use the original URI for cloning (with credentials if present)
            options = ["--depth=1", "--single-branch"]
            git.Repo.clone_from(
                uri,
                clone_path,
                progress=_clone_progress_callback,
                multi_options=options,
            )
        except git.GitCommandError as e:
            if "already exists and is not an empty directory" not in str(e):
                msg = f"Failed to clone repository: {e}"
                raise ValueError(msg) from e
            self.log.info("Repository already exists, reusing...", uri=sanitized_uri)

        # Extract git repository information from cloned path
        # Convert original URI to AnyUrl for GitRepo
        from pydantic import AnyUrl

        original_uri = AnyUrl(uri)
        return self.get_repo_info_from_path(clone_path, original_uri, sanitized_uri)

    def get_repo_info_from_path(
        self, repo_path: Path, remote_uri: AnyUrl, sanitized_remote_uri: AnyUrl
    ) -> GitRepo:
        """Extract complete git repository information from a local path."""
        try:
            repo = Repo(repo_path)
        except InvalidGitRepositoryError as e:
            raise ValueError(f"Path is not a git repository: {repo_path}") from e

        # Get all branches with their commit histories
        branches = self._get_all_branches(repo)

        # Count commits for num_commits field (managed by GitCommitRepository)
        all_commits = self._get_all_commits(repo)
        num_commits = len(all_commits)

        # Get all tags
        all_tags = self._get_all_tags(repo)

        # Get current branch as tracking branch
        try:
            current_branch = repo.active_branch
            tracking_branch = next(
                (b for b in branches if b.name == current_branch.name),
                branches[0] if branches else None,
            )
        except (AttributeError, TypeError):
            # Handle detached HEAD state or other branch access issues
            tracking_branch = branches[0] if branches else None

        if tracking_branch is None:
            raise ValueError("No branches found in repository")

        return GitRepoFactory.create_from_path_scan(
            remote_uri=remote_uri,
            sanitized_remote_uri=sanitized_remote_uri,
            repo_path=repo_path,
            tracking_branch=tracking_branch,
            last_scanned_at=datetime.now(UTC),
            num_commits=num_commits,
            num_branches=len(branches),
            num_tags=len(all_tags),
        )

    def get_commit_history(
        self, repo_path: Path, branch_name: str, limit: int = 100
    ) -> list[GitCommit]:
        """Get commit history for a specific branch."""
        try:
            repo = Repo(repo_path)

            # Get the branch reference
            branch_ref = None
            for branch in repo.branches:
                if branch.name == branch_name:
                    branch_ref = branch
                    break

            if branch_ref is None:
                return []

            # Get commit history for the branch
            commits = []
            for commit in repo.iter_commits(branch_ref, max_count=limit):
                try:
                    git_commit = self._convert_commit(repo, commit)
                    commits.append(git_commit)
                except Exception:  # noqa: BLE001, S112
                    # Skip commits we can't process
                    continue

        except (InvalidGitRepositoryError, Exception):
            return []
        else:
            return commits

    def _get_all_branches(self, repo: Repo) -> list[GitBranch]:
        """Get all branches with their commit histories."""
        branches = []

        for branch in repo.branches:
            try:
                # Get head commit for this branch
                head_commit = self._convert_commit(repo, branch.commit)
                branches.append(GitBranch(name=branch.name, head_commit=head_commit))
            except Exception:  # noqa: BLE001, S112
                # Skip branches that can't be accessed
                continue

        return branches

    def _get_all_commits(self, repo: Repo) -> list[GitCommit]:
        """Get all unique commits across all branches."""
        commit_cache = {}  # Use SHA as key to avoid duplicates

        # Get all commits from all branches
        for branch in repo.branches:
            try:
                # Traverse the entire commit history for this branch
                for commit in repo.iter_commits(branch):
                    if commit.hexsha not in commit_cache:
                        domain_commit = self._convert_commit(repo, commit)
                        commit_cache[commit.hexsha] = domain_commit
            except Exception:  # noqa: BLE001, S112
                # Skip branches that can't be accessed
                continue

        return list(commit_cache.values())

    def _get_all_tags(self, repo: Repo) -> list[GitTag]:
        """Get all tags in the repository."""
        all_commits = self._get_all_commits(repo)
        all_commits_map = {commit.commit_sha: commit for commit in all_commits}
        tags = []
        try:
            for tag_ref in repo.tags:
                try:
                    # Get the commit that the tag points to
                    target_commit = tag_ref.commit

                    tag = GitTag(
                        created_at=datetime.now(UTC),
                        name=tag_ref.name,
                        target_commit=all_commits_map[target_commit.hexsha],
                    )
                    tags.append(tag)
                except Exception:  # noqa: BLE001, S112
                    # Skip tags that can't be processed
                    continue
        except Exception:  # noqa: BLE001
            # If we can't get tags, return empty list
            return []

        return tags

    def _convert_commit(self, repo: Repo, commit: "Commit") -> GitCommit:
        """Convert a GitPython commit object to domain GitCommit."""
        # Convert timestamp to datetime
        commit_date = datetime.fromtimestamp(commit.committed_date, tz=UTC)

        # Get parent commit SHA (first parent if merge commit)
        parent_sha = commit.parents[0].hexsha if commit.parents else ""

        # Get files changed in this commit
        files = self._get_commit_files(repo, commit)

        # Format author string from name and email
        author_name = str(commit.author.name) if commit.author.name else ""
        author_email = str(commit.author.email) if commit.author.email else ""
        if author_name and author_email:
            author = f"{author_name} <{author_email}>"
        else:
            author = author_name or "Unknown"

        return GitCommit(
            commit_sha=commit.hexsha,
            date=commit_date,
            message=str(commit.message).strip(),
            parent_commit_sha=parent_sha,
            files=files,
            author=author,
        )

    def _get_commit_files(self, repo: Repo, commit: "Commit") -> list[GitFile]:
        """Get files changed in a specific commit."""
        try:
            files = []

            # Get files changed in this commit
            if commit.parents:
                # Compare with first parent to get changed files
                changed_files = commit.parents[0].diff(commit)
            else:
                # Initial commit - get all files
                changed_files = commit.diff(None)

            for diff_item in changed_files:
                # Handle both a_path and b_path (for renames/moves)
                file_path = diff_item.b_path or diff_item.a_path
                if file_path and diff_item.b_blob:
                    try:
                        blob = diff_item.b_blob
                        file_entity = GitFile(
                            created_at=datetime.now(UTC),
                            blob_sha=blob.hexsha,
                            path=str(Path(repo.working_dir) / file_path),
                            mime_type="application/octet-stream",  # Default
                            size=blob.size,
                            extension=GitFile.extension_from_path(file_path),
                        )
                        files.append(file_entity)
                    except Exception:  # noqa: BLE001, S112
                        # Skip files we can't process
                        continue

        except Exception:  # noqa: BLE001
            # If we can't get files for this commit, return empty list
            return []
        else:
            return files
