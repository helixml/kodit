"""GitPython adapter for Git operations."""

import asyncio
import mimetypes
import shutil
from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import structlog

from git import InvalidGitRepositoryError, Repo
from kodit.domain.protocols import GitAdapter


class GitPythonAdapter(GitAdapter):
    """GitPython implementation of Git operations."""

    def __init__(self, max_workers: int = 4) -> None:
        """Initialize GitPython adapter.

        Args:
            max_workers: Maximum number of worker threads.

        """
        self._log = structlog.getLogger(__name__)
        self.executor = ThreadPoolExecutor(max_workers=max_workers)

    def _raise_branch_not_found_error(self, branch_name: str) -> None:
        """Raise branch not found error."""
        raise ValueError(f"Branch {branch_name} not found")

    async def clone_repository(self, remote_uri: str, local_path: Path) -> None:
        """Clone a repository to local path."""

        def _clone() -> None:
            try:
                if local_path.exists():
                    self._log.warning(
                        f"Local path {local_path} already exists, removing and "
                        f"re-cloning..."
                    )
                    shutil.rmtree(local_path)
                local_path.mkdir(parents=True, exist_ok=True)
                self._log.debug(f"Cloning {remote_uri} to {local_path}")

                repo = Repo.clone_from(remote_uri, local_path)

                self._log.debug(
                    f"Successfully cloned {remote_uri} with {len(repo.tags)} tags"
                )
            except Exception as e:
                self._log.error(f"Failed to clone {remote_uri}: {e}")
                raise

        await asyncio.get_event_loop().run_in_executor(self.executor, _clone)

    async def pull_repository(self, local_path: Path) -> None:
        """Pull latest changes for existing repository."""

        def _pull() -> None:
            try:
                repo = Repo(local_path)
                origin = repo.remotes.origin
                origin.pull()
                self._log.info(f"Successfully pulled latest changes for {local_path}")
            except Exception as e:
                self._log.error(f"Failed to pull {local_path}: {e}")
                raise

        await asyncio.get_event_loop().run_in_executor(self.executor, _pull)

    async def get_all_branches(self, local_path: Path) -> list[dict[str, Any]]:
        """Get all branches in repository."""

        def _get_branches() -> list[dict[str, Any]]:
            try:
                repo = Repo(local_path)

                # Get local branches
                branches = [
                    {
                        "name": branch.name,
                        "type": "local",
                        "head_commit_sha": branch.commit.hexsha,
                        "is_active": branch == repo.active_branch,
                    }
                    for branch in repo.branches
                ]

                # Get remote branches
                for remote in repo.remotes:
                    for ref in remote.refs:
                        if ref.name != f"{remote.name}/HEAD":
                            branch_name = ref.name.replace(f"{remote.name}/", "")
                            # Skip if we already have this as a local branch
                            if not any(b["name"] == branch_name for b in branches):
                                branches.append(
                                    {
                                        "name": branch_name,
                                        "type": "remote",
                                        "head_commit_sha": ref.commit.hexsha,
                                        "is_active": False,
                                        "remote": remote.name,
                                    }
                                )

            except Exception as e:
                self._log.error(f"Failed to get branches for {local_path}: {e}")
                raise
            else:
                return branches

        return await asyncio.get_event_loop().run_in_executor(
            self.executor, _get_branches
        )

    async def get_branch_commits(
        self, local_path: Path, branch_name: str
    ) -> list[dict[str, Any]]:
        """Get commit history for a specific branch."""

        def _get_commits() -> list[dict[str, Any]]:
            try:
                repo = Repo(local_path)

                # Get the branch reference
                branch_ref = None
                try:
                    branch_ref = repo.branches[branch_name]
                except IndexError:
                    # Try remote branches
                    for remote in repo.remotes:
                        try:
                            branch_ref = remote.refs[branch_name]
                            break
                        except IndexError:
                            continue

                if not branch_ref:
                    self._raise_branch_not_found_error(branch_name)

                commits = []
                for commit in repo.iter_commits(branch_ref):
                    parent_sha = ""
                    if commit.parents:
                        parent_sha = commit.parents[0].hexsha

                    commits.append(
                        {
                            "sha": commit.hexsha,
                            "date": datetime.fromtimestamp(commit.committed_date, UTC),
                            "message": commit.message.strip(),
                            "parent_sha": parent_sha,
                            "author_name": commit.author.name,
                            "author_email": commit.author.email,
                            "committer_name": commit.committer.name,
                            "committer_email": commit.committer.email,
                            "tree_sha": commit.tree.hexsha,
                        }
                    )

            except Exception as e:
                self._log.error(
                    f"Failed to get commits for branch {branch_name} in "
                    f"{local_path}: {e}"
                )
                raise
            else:
                return commits

        return await asyncio.get_event_loop().run_in_executor(
            self.executor, _get_commits
        )

    async def get_commit_files(
        self, local_path: Path, commit_sha: str
    ) -> list[dict[str, Any]]:
        """Get all files in a specific commit."""

        def _get_files() -> list[dict[str, Any]]:
            try:
                repo = Repo(local_path)
                commit = repo.commit(commit_sha)

                files = []

                def process_tree(tree: Any, _: str = "") -> None:
                    for item in tree.traverse():
                        if item.type == "blob":  # It's a file
                            # Guess mime type from file path
                            mime_type = mimetypes.guess_type(item.path)[0]
                            if not mime_type:
                                mime_type = "application/octet-stream"

                            files.append(
                                {
                                    "path": item.path,
                                    "blob_sha": item.hexsha,
                                    "size": item.size,
                                    "mode": oct(item.mode),
                                    "mime_type": mime_type,
                                }
                            )

                process_tree(commit.tree)
            except Exception as e:
                self._log.error(
                    f"Failed to get files for commit {commit_sha} in {local_path}: {e}"
                )
                raise
            else:
                return files

        return await asyncio.get_event_loop().run_in_executor(self.executor, _get_files)

    async def repository_exists(self, local_path: Path) -> bool:
        """Check if repository exists at local path."""

        def _check_exists() -> bool:
            try:
                Repo(local_path)
            except (InvalidGitRepositoryError, Exception):
                return False
            else:
                return True

        return await asyncio.get_event_loop().run_in_executor(
            self.executor, _check_exists
        )

    async def get_commit_details(
        self, local_path: Path, commit_sha: str
    ) -> dict[str, Any]:
        """Get detailed information about a specific commit."""

        def _get_commit_details() -> dict[str, Any]:
            try:
                repo = Repo(local_path)
                commit = repo.commit(commit_sha)

                parent_sha = ""
                if commit.parents:
                    parent_sha = commit.parents[0].hexsha

                return {
                    "sha": commit.hexsha,
                    "date": datetime.fromtimestamp(commit.committed_date, UTC),
                    "message": commit.message.strip(),
                    "parent_sha": parent_sha,
                    "author_name": commit.author.name,
                    "author_email": commit.author.email,
                    "committer_name": commit.committer.name,
                    "committer_email": commit.committer.email,
                    "tree_sha": commit.tree.hexsha,
                    "stats": commit.stats.total,
                }
            except Exception as e:
                self._log.error(
                    f"Failed to get commit details for {commit_sha} in "
                    f"{local_path}: {e}"
                )
                raise

        return await asyncio.get_event_loop().run_in_executor(
            self.executor, _get_commit_details
        )

    async def ensure_repository(self, remote_uri: str, local_path: Path) -> None:
        """Clone repository if it doesn't exist, otherwise pull latest changes."""
        if await self.repository_exists(local_path):
            await self.pull_repository(local_path)
        else:
            await self.clone_repository(remote_uri, local_path)

    async def get_file_content(
        self, local_path: Path, commit_sha: str, file_path: str
    ) -> bytes:
        """Get file content at specific commit."""

        def _get_file_content() -> bytes:
            try:
                repo = Repo(local_path)
                commit = repo.commit(commit_sha)

                # Navigate to the file in the tree
                blob = commit.tree[file_path]
                return blob.data_stream.read()
            except Exception as e:
                self._log.error(
                    f"Failed to get file content for {file_path} at {commit_sha}: {e}"
                )
                raise

        return await asyncio.get_event_loop().run_in_executor(
            self.executor, _get_file_content
        )

    async def get_latest_commit_sha(
        self, local_path: Path, branch_name: str = "HEAD"
    ) -> str:
        """Get the latest commit SHA for a branch."""

        def _get_latest_commit() -> str:
            try:
                repo = Repo(local_path)
                if branch_name == "HEAD":
                    commit_sha = repo.head.commit.hexsha
                else:
                    branch = repo.branches[branch_name]
                    commit_sha = branch.commit.hexsha
            except Exception as e:
                self._log.error(
                    f"Failed to get latest commit for {branch_name} in "
                    f"{local_path}: {e}"
                )
                raise
            else:
                return commit_sha

        return await asyncio.get_event_loop().run_in_executor(
            self.executor, _get_latest_commit
        )

    def __del__(self) -> None:
        """Cleanup executor on deletion."""
        if hasattr(self, "executor"):
            self.executor.shutdown(wait=True)

    async def get_all_tags(self, local_path: Path) -> list[dict[str, Any]]:
        """Get all tags in repository."""

        def _get_tags() -> list[dict[str, Any]]:
            try:
                repo = Repo(local_path)
                self._log.info(f"Getting all tags for {local_path}: {len(repo.tags)}")
                return [
                    {
                        "name": tag.name,
                        "target_commit_sha": tag.commit.hexsha,
                    }
                    for tag in repo.tags
                ]

            except Exception as e:
                self._log.error(f"Failed to get tags for {local_path}: {e}")
                raise

        return await asyncio.get_event_loop().run_in_executor(self.executor, _get_tags)
