import hashlib
import shutil
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path

import structlog
from pydantic import AnyUrl

from kodit.domain.entities import GitBranch, GitCommit, GitFile, GitRepo, WorkingCopy
from kodit.domain.protocols import GitAdapter


@dataclass(frozen=True)
class RepositoryScanResult:
    """Immutable scan result containing all repository metadata."""

    branches: list[GitBranch]
    all_commits: list[GitCommit]
    scan_timestamp: datetime
    total_unique_commits: int
    total_files_across_commits: int


@dataclass(frozen=True)
class RepositoryInfo:
    """Immutable repository information needed for GitRepo construction."""

    remote_uri: AnyUrl
    sanitized_remote_uri: AnyUrl
    cloned_path: Path


class GitRepositoryScanner:
    """Pure scanner that extracts data without mutation."""

    def __init__(self, git_adapter: GitAdapter) -> None:
        self._log = structlog.getLogger(__name__)
        self.git_adapter = git_adapter

    async def scan_repository(self, cloned_path: Path) -> RepositoryScanResult:
        """Scan repository and return immutable result data."""
        self._log.info(f"Starting repository scan at: {cloned_path}")

        # Get all branches
        branch_data = await self.git_adapter.get_all_branches(cloned_path)
        self._log.info(f"Found {len(branch_data)} branches")

        branches = []
        commit_cache = {}  # Cache commits to avoid duplicates

        for branch_info in branch_data:
            self._log.info(f"Processing branch: {branch_info['name']}")

            # Get commit history for this branch
            commits_data = await self.git_adapter.get_branch_commits(
                cloned_path, branch_info["name"]
            )

            if not commits_data:
                self._log.warning(f"No commits found for branch {branch_info['name']}")
                continue

            # Process commits for this branch
            head_commit = None

            for commit_data in commits_data:
                commit_sha = commit_data["sha"]

                # Use cached commit if already processed
                if commit_sha in commit_cache:
                    if head_commit is None:  # First commit is head
                        head_commit = commit_cache[commit_sha]
                    continue

                # Get file information for this commit
                try:
                    files_data = await self.git_adapter.get_commit_files(
                        cloned_path, commit_sha
                    )

                    # Create GitFile entities from the git data
                    files = [
                        GitFile(
                            blob_sha=f["blob_sha"],
                            path=str(cloned_path / f["path"]),
                            mime_type=f.get("mime_type", "application/octet-stream"),
                            size=f["size"],
                        )
                        for f in files_data
                    ]

                    # Format author string from name and email
                    author_name = commit_data.get("author_name", "")
                    author_email = commit_data.get("author_email", "")
                    if author_name and author_email:
                        author = f"{author_name} <{author_email}>"
                    else:
                        author = author_name or "Unknown"

                    git_commit = GitCommit(
                        commit_sha=commit_sha,
                        date=commit_data["date"],
                        message=commit_data["message"],
                        parent_commit_sha=commit_data["parent_sha"],
                        files=files,
                        author=author,
                    )

                    commit_cache[commit_sha] = git_commit

                    if head_commit is None:  # First commit is head
                        head_commit = git_commit

                except Exception as e:
                    self._log.error(f"Failed to process commit {commit_sha}: {e}")
                    continue

            if head_commit:
                branch = GitBranch(name=branch_info["name"], head_commit=head_commit)
                branches.append(branch)

        total_files = sum(len(commit.files) for commit in commit_cache.values())

        scan_result = RepositoryScanResult(
            branches=branches,
            all_commits=list(commit_cache.values()),
            scan_timestamp=datetime.now(UTC),
            total_unique_commits=len(commit_cache),
            total_files_across_commits=total_files,
        )

        self._log.info(
            f"Scan completed. Found {len(branches)} branches with "
            f"{len(commit_cache)} unique commits"
        )
        return scan_result


class GitRepoFactory:
    """Factory for creating GitRepo instances from scan results."""

    @staticmethod
    def create_from_scan(
        repo_info: RepositoryInfo, scan_result: RepositoryScanResult
    ) -> GitRepo:
        """Create GitRepo from repository info and scan results."""
        # Determine tracking branch (prefer main, then master, then first available)
        tracking_branch = None
        for preferred_name in ["main", "master"]:
            tracking_branch = next(
                (b for b in scan_result.branches if b.name == preferred_name), None
            )
            if tracking_branch:
                break

        if not tracking_branch and scan_result.branches:
            tracking_branch = scan_result.branches[0]

        if not tracking_branch:
            raise ValueError("No tracking branch found")

        return GitRepo(
            id=GitRepo.create_id(repo_info.sanitized_remote_uri),
            sanitized_remote_uri=repo_info.sanitized_remote_uri,
            branches=scan_result.branches,
            tracking_branch=tracking_branch,
            cloned_path=repo_info.cloned_path,
            remote_uri=repo_info.remote_uri,
            last_scanned_at=scan_result.scan_timestamp,
            total_unique_commits=scan_result.total_unique_commits,
            commits=scan_result.all_commits,
        )


class RepositoryCloner:
    """Pure service for cloning repositories."""

    def __init__(self, git_adapter: GitAdapter, clone_dir: Path) -> None:
        self.git_adapter = git_adapter
        self.clone_dir = clone_dir

    def _get_clone_path(self, sanitized_uri: AnyUrl) -> Path:
        """Get the clone path for a Git working copy."""
        dir_hash = hashlib.sha256(str(sanitized_uri).encode("utf-8")).hexdigest()[:16]
        dir_name = f"repo-{dir_hash}"
        return self.clone_dir / dir_name

    async def clone_repository(self, remote_uri: AnyUrl) -> RepositoryInfo:
        """Clone repository and return repository info."""
        sanitized_uri = WorkingCopy.sanitize_git_url(str(remote_uri))
        clone_path = self._get_clone_path(sanitized_uri)

        try:
            await self.git_adapter.clone_repository(str(remote_uri), clone_path)
        except Exception:
            shutil.rmtree(clone_path)
            raise

        return RepositoryInfo(
            remote_uri=remote_uri,
            sanitized_remote_uri=sanitized_uri,
            cloned_path=clone_path,
        )
