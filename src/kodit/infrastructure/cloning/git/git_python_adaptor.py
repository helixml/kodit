"""GitPython adapter for Git operations."""

import asyncio
import mimetypes
import shutil
from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import structlog

from git import Blob, GitCommandError, InvalidGitRepositoryError, Repo, Tree


def _collect_unique_commits(repo: Repo, log: Any) -> set:
    """Collect all unique commits from all branches."""
    all_commits = set()

    # Collect from local branches
    for branch in repo.branches:
        for commit in repo.iter_commits(branch):
            all_commits.add(commit)

    # Collect from remote branches
    for remote in repo.remotes:
        for ref in remote.refs:
            if ref.name != f"{remote.name}/HEAD":
                try:
                    for commit in repo.iter_commits(ref):
                        all_commits.add(commit)
                except Exception as e:  # noqa: BLE001
                    log.debug("Skipping ref %s: %s", ref.name, e)
                    continue

    return all_commits


def _process_commits(all_commits: set) -> dict[str, dict[str, Any]]:
    """Process commits into the final format."""
    commits_map = {}
    for commit in all_commits:
        parent_sha = ""
        if commit.parents:
            parent_sha = commit.parents[0].hexsha

        commits_map[commit.hexsha] = {
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
    return commits_map


class GitPythonAdapter:
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

    async def checkout_commit(self, local_path: Path, commit_sha: str) -> None:
        """Checkout a specific commit."""

        def _checkout() -> None:
            try:
                repo = Repo(local_path)
                self._log.debug(f"Checking out commit {commit_sha} in {local_path}")
                repo.git.checkout(commit_sha)
                self._log.debug(f"Successfully checked out {commit_sha}")
            except Exception as e:
                self._log.error(f"Failed to checkout {commit_sha}: {e}")
                raise

        await asyncio.get_event_loop().run_in_executor(self.executor, _checkout)

    async def checkout_branch(self, local_path: Path, branch_name: str) -> None:
        """Checkout a specific branch."""

        def _checkout() -> None:
            try:
                repo = Repo(local_path)
                repo.git.checkout(branch_name)
            except Exception as e:
                self._log.error(f"Failed to checkout {branch_name}: {e}")
                raise

        await asyncio.get_event_loop().run_in_executor(self.executor, _checkout)

    async def fetch_repository(self, local_path: Path) -> None:
        """Fetch latest changes for existing repository."""

        def _fetch() -> None:
            try:
                repo = Repo(local_path)
                origin = repo.remotes.origin
                origin.fetch()
            except Exception as e:
                self._log.error(f"Failed to fetch {local_path}: {e}")
                raise

        await asyncio.get_event_loop().run_in_executor(self.executor, _fetch)

    async def pull_repository(self, local_path: Path) -> None:
        """Pull latest changes for existing repository.

        First fetches all refs from remote, then attempts to pull (merge) into
        the current branch. If pull fails (e.g., detached HEAD state), the fetch
        still ensures remote refs are up-to-date for branch lookups.
        """

        def _pull() -> None:
            try:
                repo = Repo(local_path)
                origin = repo.remotes.origin

                # Always fetch first to ensure refs are up-to-date
                origin.fetch()

                # Try to pull (merge) if we're on a branch
                try:
                    origin.pull()
                    self._log.info(
                        f"Successfully pulled latest changes for {local_path}"
                    )
                except GitCommandError as e:
                    # Pull can fail in detached HEAD state or other conditions
                    # The fetch above ensures refs are still updated
                    self._log.debug(
                        f"Pull failed (possibly detached HEAD), "
                        f"but fetch succeeded for {local_path}: {e}"
                    )

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
                # Check if HEAD is detached
                try:
                    active_branch = repo.active_branch
                except TypeError:
                    # HEAD is detached, no active branch
                    active_branch = None

                branches = [
                    {
                        "name": branch.name,
                        "type": "local",
                        "head_commit_sha": branch.commit.hexsha,
                        "is_active": active_branch is not None
                        and branch == active_branch,
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

    async def get_all_commits_bulk(
        self, local_path: Path, since_date: datetime | None = None
    ) -> dict[str, dict[str, Any]]:
        """Get all commits from all branches in bulk for efficiency.

        Optimized to use git rev-list --all which is much faster than iterating
        through all branches.

        Args:
            local_path: Path to the git repository
            since_date: Optional date to get commits after (for incremental scanning)

        Returns:
            Dictionary mapping commit SHAs to commit data

        """

        def _get_all_commits() -> dict[str, dict[str, Any]]:
            try:
                repo = Repo(local_path)

                # Build git command with optional date filter
                if since_date:
                    # Format date for git (git uses ISO 8601 format)
                    # Use --since to get commits after the given date
                    since_str = since_date.strftime("%Y-%m-%d %H:%M:%S")
                    rev_list_output = repo.git.rev_list("--all", f"--since={since_str}")
                    commit_shas = rev_list_output.split("\n")
                    self._log.info(
                        "Getting commits since %s, found %d commits",
                        since_str,
                        len([s for s in commit_shas if s]),
                    )
                else:
                    # Get all commits
                    commit_shas = repo.git.rev_list("--all").split("\n")

                # Batch fetch commit objects
                commits_map = {}
                for sha in commit_shas:
                    if not sha:
                        continue
                    try:
                        commit = repo.commit(sha)
                        parent_sha = ""
                        if commit.parents:
                            parent_sha = commit.parents[0].hexsha

                        commits_map[commit.hexsha] = {
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
                    except Exception as e:  # noqa: BLE001
                        self._log.debug("Skipping commit %s: %s", sha, e)
                        continue

            except Exception as e:
                self._log.error("Failed to get bulk commits for %s: %s", local_path, e)
                raise
            else:
                return commits_map

        return await asyncio.get_event_loop().run_in_executor(
            self.executor, _get_all_commits
        )

    async def get_branch_commit_shas(
        self, local_path: Path, branch_name: str
    ) -> list[str]:
        """Get only commit SHAs for a branch (much faster than full commit data)."""

        def _get_commit_shas() -> list[str]:
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

                return [commit.hexsha for commit in repo.iter_commits(branch_ref)]

            except Exception as e:
                self._log.error(
                    f"Failed to get commit SHAs for branch {branch_name} in "
                    f"{local_path}: {e}"
                )
                raise

        return await asyncio.get_event_loop().run_in_executor(
            self.executor, _get_commit_shas
        )

    async def get_all_branch_head_shas(  # noqa: C901
        self, local_path: Path, branch_names: list[str]
    ) -> dict[str, str]:
        """Get head commit SHAs for all branches in one operation.

        This is much more efficient than calling get_branch_commit_shas for each
        branch individually.

        Args:
            local_path: Path to the repository
            branch_names: List of branch names to get head SHAs for

        Returns:
            Dictionary mapping branch names to their head commit SHAs

        """

        def _build_branch_maps(repo: Repo) -> tuple[dict, dict]:
            """Build lookup maps for local and remote branches."""
            local_branches = {branch.name: branch for branch in repo.branches}
            remote_branches = {}
            for remote in repo.remotes:
                for ref in remote.refs:
                    if ref.name != f"{remote.name}/HEAD":
                        branch_name = ref.name.replace(f"{remote.name}/", "")
                        remote_branches[branch_name] = ref
            return local_branches, remote_branches

        def _get_branch_head_sha(
            branch_name: str, local_branches: dict, remote_branches: dict
        ) -> str | None:
            """Get head SHA for a single branch."""
            try:
                if branch_name in local_branches:
                    return local_branches[branch_name].commit.hexsha
                if branch_name in remote_branches:
                    return remote_branches[branch_name].commit.hexsha
                self._log.warning(
                    "Branch %s not found in local or remote branches", branch_name
                )
            except Exception as e:  # noqa: BLE001
                self._log.debug(
                    "Failed to get head SHA for branch %s: %s", branch_name, e
                )
            return None

        def _get_all_head_shas() -> dict[str, str]:
            try:
                repo = Repo(local_path)
                local_branches, remote_branches = _build_branch_maps(repo)

                result = {}
                for branch_name in branch_names:
                    head_sha = _get_branch_head_sha(
                        branch_name, local_branches, remote_branches
                    )
                    if head_sha:
                        result[branch_name] = head_sha

            except Exception as e:
                self._log.error(
                    "Failed to get all branch head SHAs for %s: %s", local_path, e
                )
                raise
            else:
                return result

        return await asyncio.get_event_loop().run_in_executor(
            self.executor, _get_all_head_shas
        )

    async def get_commit_files(
        self, local_path: Path, commit_sha: str
    ) -> list[dict[str, Any]]:
        """Get all files in a specific commit from the git tree.

        Args:
            local_path: Path to the repository
            commit_sha: SHA of the commit to get files for
            repo: Repo object to reuse (avoids creating new Repo per commit)

        """

        def _get_files() -> list[dict[str, Any]]:
            try:
                _repo = Repo(local_path)
                commit = _repo.commit(commit_sha)

                files = []

                def process_tree(tree: Tree, _: str = "") -> None:
                    for item in tree.traverse():
                        if not item:
                            continue
                        if not isinstance(item, Blob):
                            continue
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
                                "created_at": commit.committed_datetime,
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

    async def get_default_branch(self, local_path: Path) -> str:
        """Get the default branch name from origin/HEAD."""
        repo = Repo(local_path)
        if not hasattr(repo.remotes, "origin"):
            raise ValueError(f"Repository {local_path} has no origin remote")

        origin = repo.remotes.origin
        try:
            return origin.refs.HEAD.ref.name.removeprefix("origin/")
        except (AttributeError, IndexError) as e:
            raise ValueError(f"Repository {local_path} has no default branch") from e

    async def get_latest_commit_sha(
        self, local_path: Path, branch_name: str = "HEAD"
    ) -> str:
        """Get the latest commit SHA for a branch."""

        def _get_latest_commit() -> str:
            repo = Repo(local_path)
            if branch_name == "HEAD":
                return repo.head.commit.hexsha

            branch_ref = None
            try:
                branch_ref = repo.branches[branch_name]
            except IndexError:
                for remote in repo.remotes:
                    try:
                        branch_ref = remote.refs[branch_name]
                        break
                    except IndexError:
                        continue

            if not branch_ref:
                self._raise_branch_not_found_error(branch_name)
                raise AssertionError("unreachable")

            return branch_ref.commit.hexsha

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

    async def get_commit_diff(self, local_path: Path, commit_sha: str) -> str:
        """Get the diff for a specific commit."""

        def _get_diff() -> str:
            try:
                repo = Repo(local_path)
                commit = repo.commit(commit_sha)

                # If this is the first commit (no parents), show diff against empty tree
                if not commit.parents:
                    diffs = commit.diff(None, create_patch=True)
                    if not diffs:
                        return ""
                    first_diff = diffs[0]
                    diff_bytes = first_diff.diff
                    if isinstance(diff_bytes, bytes):
                        return diff_bytes.decode("utf-8")
                    return str(diff_bytes) if diff_bytes is not None else ""

                # For commits with parents, show diff against first parent
                parent = commit.parents[0]
                diffs = parent.diff(commit, create_patch=True)

                # Combine all diffs into a single string
                diff_text = ""
                for diff in diffs:
                    diff_bytes = diff.diff
                    if diff_bytes and isinstance(diff_bytes, bytes):
                        diff_text += diff_bytes.decode("utf-8")
            except Exception as e:
                self._log.error(
                    f"Failed to get diff for commit {commit_sha} in {local_path}: {e}"
                )
                raise
            else:
                return diff_text

        return await asyncio.get_event_loop().run_in_executor(self.executor, _get_diff)
