"""PyGit2 adapter for Git operations."""

import asyncio
import mimetypes
import shutil
from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import pygit2
import structlog


class PyGit2Adapter:
    """PyGit2 implementation of Git operations."""

    def __init__(self, max_workers: int = 4) -> None:
        """Initialize PyGit2 adapter."""
        self._log = structlog.getLogger(__name__)
        self._executor = ThreadPoolExecutor(max_workers=max_workers)

    def _raise_branch_not_found_error(self, branch_name: str) -> None:
        """Raise branch not found error."""
        raise ValueError(f"Branch {branch_name} not found")

    def _create_callbacks(self, remote_uri: str) -> pygit2.RemoteCallbacks:
        """Create remote callbacks for authentication."""
        callbacks = pygit2.RemoteCallbacks()

        def credentials_callback(
            _url: str, username_from_url: str | None, allowed_types: int
        ) -> Any:
            if allowed_types & pygit2.enums.CredentialType.SSH_KEY:
                return pygit2.KeypairFromAgent(username_from_url or "git")
            is_userpass = allowed_types & pygit2.enums.CredentialType.USERPASS_PLAINTEXT
            if is_userpass and "://" in remote_uri:
                # Extract credentials from URL if present
                from urllib.parse import urlparse

                parsed = urlparse(remote_uri)
                if parsed.username and parsed.password:
                    return pygit2.UserPass(parsed.username, parsed.password)
            return None

        callbacks.credentials = credentials_callback  # type: ignore[assignment]
        return callbacks

    async def clone_repository(self, remote_uri: str, local_path: Path) -> None:
        """Clone a repository to local path."""

        def _clone() -> None:
            if local_path.exists():
                self._log.warning(
                    "Local path %s already exists, removing and re-cloning...",
                    local_path,
                )
                shutil.rmtree(local_path)
            local_path.mkdir(parents=True, exist_ok=True)
            self._log.debug(f"Cloning {remote_uri} to {local_path}")

            callbacks = self._create_callbacks(remote_uri)
            repo = pygit2.clone_repository(
                remote_uri, str(local_path), callbacks=callbacks
            )

            tag_count = sum(
                1 for ref in repo.references if ref.startswith("refs/tags/")
            )
            self._log.debug(f"Successfully cloned {remote_uri} with {tag_count} tags")

        await asyncio.get_event_loop().run_in_executor(self._executor, _clone)

    async def checkout_commit(self, local_path: Path, commit_sha: str) -> None:
        """Checkout a specific commit."""

        def _checkout() -> None:
            repo = pygit2.Repository(str(local_path))
            self._log.debug(f"Checking out commit {commit_sha} in {local_path}")
            commit = repo.get(commit_sha)
            if commit is None:
                raise ValueError(f"Commit {commit_sha} not found")
            repo.checkout_tree(commit)
            repo.set_head(commit.id)
            self._log.debug(f"Successfully checked out {commit_sha}")

        await asyncio.get_event_loop().run_in_executor(self._executor, _checkout)

    async def checkout_branch(self, local_path: Path, branch_name: str) -> None:
        """Checkout a specific branch."""

        def _checkout() -> None:
            repo = pygit2.Repository(str(local_path))

            # Try local branch first
            branch = repo.branches.get(branch_name)
            if branch:
                repo.checkout(branch)
                return

            # Try remote branch
            remote_ref_name = f"refs/remotes/origin/{branch_name}"
            if remote_ref_name in repo.references:
                remote_ref = repo.references[remote_ref_name]
                commit = repo.get(remote_ref.target)
                if commit is None or not isinstance(commit, pygit2.Commit):
                    raise ValueError(f"Could not resolve remote branch {branch_name}")
                repo.checkout_tree(commit)
                # Create local branch tracking remote
                repo.branches.local.create(branch_name, commit)
                repo.set_head(f"refs/heads/{branch_name}")
                return

            self._raise_branch_not_found_error(branch_name)

        await asyncio.get_event_loop().run_in_executor(self._executor, _checkout)

    async def fetch_repository(self, local_path: Path) -> None:
        """Fetch latest changes for existing repository."""

        def _fetch() -> None:
            repo = pygit2.Repository(str(local_path))
            origin = repo.remotes["origin"]
            url = origin.url or ""
            callbacks = self._create_callbacks(url)
            origin.fetch(callbacks=callbacks)

        await asyncio.get_event_loop().run_in_executor(self._executor, _fetch)

    async def pull_repository(self, local_path: Path) -> None:
        """Pull latest changes for existing repository."""

        def _pull() -> None:
            repo = pygit2.Repository(str(local_path))
            origin = repo.remotes["origin"]
            url = origin.url or ""
            callbacks = self._create_callbacks(url)

            # Always fetch first
            origin.fetch(callbacks=callbacks)

            # Try to merge if we're on a branch
            if repo.head_is_detached:
                self._log.debug(
                    f"Repository {local_path} is in detached HEAD state, "
                    "skipping merge"
                )
                return

            branch_name = repo.head.shorthand
            remote_ref_name = f"refs/remotes/origin/{branch_name}"

            if remote_ref_name not in repo.references:
                self._log.debug(
                    f"No remote tracking branch for {branch_name}, skipping merge"
                )
                return

            remote_ref = repo.references[remote_ref_name]
            remote_commit_id = remote_ref.target

            # Perform merge analysis
            merge_result, _ = repo.merge_analysis(remote_commit_id)

            if merge_result & pygit2.enums.MergeAnalysis.UP_TO_DATE:
                self._log.debug(f"Repository {local_path} is already up to date")
                return

            if merge_result & pygit2.enums.MergeAnalysis.FASTFORWARD:
                # Fast-forward merge
                repo.checkout_tree(repo.get(remote_commit_id))
                branch_ref = repo.references[f"refs/heads/{branch_name}"]
                branch_ref.set_target(remote_commit_id)
                repo.head.set_target(remote_commit_id)
                self._log.info(f"Fast-forward merged {branch_name} in {local_path}")
            else:
                self._log.debug(
                    f"Non-fast-forward merge needed for {local_path}, skipping"
                )

        await asyncio.get_event_loop().run_in_executor(self._executor, _pull)

    async def get_all_branches(self, local_path: Path) -> list[dict[str, Any]]:
        """Get all branches in repository."""

        def _get_branches() -> list[dict[str, Any]]:
            repo = pygit2.Repository(str(local_path))
            branches: list[dict[str, Any]] = []

            # Get active branch if not detached
            active_branch_name: str | None = None
            if not repo.head_is_detached:
                active_branch_name = repo.head.shorthand

            # Get local branches
            for branch_name in repo.branches.local:
                branch = repo.branches.local[branch_name]
                branches.append(
                    {
                        "name": branch_name,
                        "type": "local",
                        "head_commit_sha": str(branch.target),
                        "is_active": branch_name == active_branch_name,
                    }
                )

            # Get remote branches
            local_names = {b["name"] for b in branches}
            for ref_name in repo.branches.remote:
                if ref_name == "origin/HEAD":
                    continue
                branch_name = ref_name.replace("origin/", "")
                if branch_name in local_names:
                    continue

                branch = repo.branches.remote[ref_name]
                branches.append(
                    {
                        "name": branch_name,
                        "type": "remote",
                        "head_commit_sha": str(branch.target),
                        "is_active": False,
                        "remote": "origin",
                    }
                )

            return branches

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_branches
        )

    def _commit_to_dict(self, commit: pygit2.Commit) -> dict[str, Any]:
        """Convert a pygit2 commit to a dictionary."""
        parent_sha = ""
        if commit.parents:
            parent_sha = str(commit.parents[0].id)

        return {
            "sha": str(commit.id),
            "date": datetime.fromtimestamp(commit.commit_time, UTC),
            "message": commit.message.strip(),
            "parent_sha": parent_sha,
            "author_name": commit.author.name,
            "author_email": commit.author.email,
            "committer_name": commit.committer.name,
            "committer_email": commit.committer.email,
            "tree_sha": str(commit.tree_id),
        }

    async def get_branch_commits(
        self, local_path: Path, branch_name: str
    ) -> list[dict[str, Any]]:
        """Get commit history for a specific branch."""

        def _get_commits() -> list[dict[str, Any]]:
            repo = pygit2.Repository(str(local_path))

            # Find the branch target
            target: Any = None

            # Try local branch
            if branch_name in repo.branches.local:
                target = repo.branches.local[branch_name].target

            # Try remote branch
            if target is None:
                remote_name = f"origin/{branch_name}"
                if remote_name in repo.branches.remote:
                    target = repo.branches.remote[remote_name].target

            if target is None:
                self._raise_branch_not_found_error(branch_name)
                raise AssertionError("unreachable")

            return [
                self._commit_to_dict(commit)
                for commit in repo.walk(target, pygit2.enums.SortMode.TIME)
            ]

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_commits
        )

    async def get_all_commits_bulk(
        self, local_path: Path, since_date: datetime | None = None
    ) -> dict[str, dict[str, Any]]:
        """Get all commits from all branches in bulk for efficiency."""

        def _get_all_commits() -> dict[str, dict[str, Any]]:
            repo = pygit2.Repository(str(local_path))

            # Collect all commit oids from all refs
            commit_oids: set[Any] = set()

            for ref_name in repo.references:
                ref = repo.references[ref_name]
                target = ref.resolve().target
                # Walk commits from each ref
                for commit in repo.walk(target, pygit2.enums.SortMode.TIME):
                    if since_date is not None:
                        commit_time = datetime.fromtimestamp(commit.commit_time, UTC)
                        if commit_time < since_date:
                            break
                    commit_oids.add(commit.id)

            if since_date:
                self._log.info(
                    "Getting commits since %s, found %d commits",
                    since_date.strftime("%Y-%m-%d %H:%M:%S"),
                    len(commit_oids),
                )

            commits_map: dict[str, dict[str, Any]] = {}
            for oid in commit_oids:
                obj = repo.get(oid)
                if isinstance(obj, pygit2.Commit):
                    commits_map[str(oid)] = self._commit_to_dict(obj)

            return commits_map

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_all_commits
        )

    async def get_branch_commit_shas(
        self, local_path: Path, branch_name: str
    ) -> list[str]:
        """Get only commit SHAs for a branch."""

        def _get_commit_shas() -> list[str]:
            repo = pygit2.Repository(str(local_path))

            # Find the branch target
            target: Any = None

            if branch_name in repo.branches.local:
                target = repo.branches.local[branch_name].target

            if target is None:
                remote_name = f"origin/{branch_name}"
                if remote_name in repo.branches.remote:
                    target = repo.branches.remote[remote_name].target

            if target is None:
                self._raise_branch_not_found_error(branch_name)
                raise AssertionError("unreachable")

            return [str(commit.id) for commit in repo.walk(target)]

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_commit_shas
        )

    async def get_all_branch_head_shas(
        self, local_path: Path, branch_names: list[str]
    ) -> dict[str, str]:
        """Get head commit SHAs for all branches in one operation."""

        def _get_all_head_shas() -> dict[str, str]:
            repo = pygit2.Repository(str(local_path))
            result: dict[str, str] = {}

            for branch_name in branch_names:
                # Try local branch
                if branch_name in repo.branches.local:
                    result[branch_name] = str(repo.branches.local[branch_name].target)
                    continue

                # Try remote branch
                remote_name = f"origin/{branch_name}"
                if remote_name in repo.branches.remote:
                    result[branch_name] = str(repo.branches.remote[remote_name].target)
                    continue

                self._log.warning(
                    "Branch %s not found in local or remote branches", branch_name
                )

            return result

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_all_head_shas
        )

    async def get_commit_files(
        self, local_path: Path, commit_sha: str
    ) -> list[dict[str, Any]]:
        """Get all files in a specific commit from the git tree."""

        def _get_files() -> list[dict[str, Any]]:
            repo = pygit2.Repository(str(local_path))
            commit = repo.get(commit_sha)

            if not isinstance(commit, pygit2.Commit):
                raise TypeError(f"Object {commit_sha} is not a commit")

            files: list[dict[str, Any]] = []

            def process_tree(tree: pygit2.Tree, prefix: str = "") -> None:
                for entry in tree:
                    entry_path = f"{prefix}{entry.name}" if prefix else entry.name
                    if entry.type_str == "tree":
                        subtree = repo.get(entry.id)
                        if isinstance(subtree, pygit2.Tree):
                            process_tree(subtree, f"{entry_path}/")
                    elif entry.type_str == "blob":
                        blob = repo.get(entry.id)
                        if isinstance(blob, pygit2.Blob):
                            mime_type = mimetypes.guess_type(entry_path)[0]
                            if not mime_type:
                                mime_type = "application/octet-stream"
                            files.append(
                                {
                                    "path": entry_path,
                                    "blob_sha": str(entry.id),
                                    "size": blob.size,
                                    "mode": oct(entry.filemode),
                                    "mime_type": mime_type,
                                    "created_at": datetime.fromtimestamp(
                                        commit.commit_time, UTC
                                    ),
                                }
                            )

            process_tree(commit.tree)
            return files

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_files
        )

    async def repository_exists(self, local_path: Path) -> bool:
        """Check if repository exists at local path."""

        def _check_exists() -> bool:
            try:
                pygit2.Repository(str(local_path))
            except pygit2.GitError:
                return False
            else:
                return True

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _check_exists
        )

    async def get_commit_details(
        self, local_path: Path, commit_sha: str
    ) -> dict[str, Any]:
        """Get detailed information about a specific commit."""

        def _get_commit_details() -> dict[str, Any]:
            repo = pygit2.Repository(str(local_path))
            commit = repo.get(commit_sha)

            if not isinstance(commit, pygit2.Commit):
                raise TypeError(f"Object {commit_sha} is not a commit")

            result = self._commit_to_dict(commit)

            # Calculate stats by diffing with parent
            insertions = 0
            deletions = 0
            files_changed = 0

            if commit.parents:
                diff = repo.diff(commit.parents[0], commit)
                stats = diff.stats
                insertions = stats.insertions
                deletions = stats.deletions
                files_changed = stats.files_changed
            else:
                # First commit - diff against empty tree
                diff = commit.tree.diff_to_tree()
                stats = diff.stats
                insertions = stats.insertions
                deletions = stats.deletions
                files_changed = stats.files_changed

            result["stats"] = {
                "insertions": insertions,
                "deletions": deletions,
                "files": files_changed,
                "lines": insertions + deletions,
            }

            return result

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_commit_details
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
            repo = pygit2.Repository(str(local_path))
            commit = repo.get(commit_sha)

            if not isinstance(commit, pygit2.Commit):
                raise TypeError(f"Object {commit_sha} is not a commit")

            # Navigate to the file in the tree
            tree = commit.tree
            for part in file_path.split("/"):
                entry = tree[part]
                if entry.type_str == "tree":
                    tree = repo.get(entry.id)
                elif entry.type_str == "blob":
                    blob = repo.get(entry.id)
                    if isinstance(blob, pygit2.Blob):
                        return blob.data
                    raise ValueError(f"Could not read blob for {file_path}")
            raise ValueError(f"File {file_path} not found in commit {commit_sha}")

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_file_content
        )

    def _get_remote_branch_names(self, repo: pygit2.Repository) -> list[str]:
        """Get list of branch names from remote refs."""
        branches = []
        for ref_name in repo.branches.remote:
            if ref_name != "origin/HEAD":
                branch_name = ref_name.replace("origin/", "")
                branches.append(branch_name)
        return branches

    def _get_default_branch_sync(self, local_path: Path) -> str:
        """Detect the default branch with fallback strategies."""
        repo = pygit2.Repository(str(local_path))

        if "origin" not in [r.name for r in repo.remotes]:
            raise ValueError(f"Repository {local_path} has no origin remote")

        # Strategy 1: Try origin/HEAD symbolic reference
        head_ref_name = "refs/remotes/origin/HEAD"
        if head_ref_name in repo.references:
            head_ref = repo.references[head_ref_name]
            if head_ref.type == pygit2.enums.ReferenceType.SYMBOLIC:
                target_name = str(head_ref.target)
                return target_name.replace("refs/remotes/origin/", "")

        # Strategy 2: Look for common default branch names
        remote_branches = self._get_remote_branch_names(repo)
        for candidate in ["main", "master"]:
            if candidate in remote_branches:
                self._log.info(
                    "origin/HEAD not set, falling back to branch",
                    branch=candidate,
                )
                return candidate

        # Strategy 3: Use first available branch
        if remote_branches:
            first_branch = remote_branches[0]
            self._log.info(
                "origin/HEAD not set and no main/master, using first branch",
                branch=first_branch,
            )
            return first_branch

        raise ValueError(f"Repository {local_path} has no branches")

    async def get_default_branch(self, local_path: Path) -> str:
        """Get the default branch name with fallback strategies."""
        return await asyncio.get_event_loop().run_in_executor(
            self._executor, self._get_default_branch_sync, local_path
        )

    async def get_latest_commit_sha(
        self, local_path: Path, branch_name: str = "HEAD"
    ) -> str:
        """Get the latest commit SHA for a branch."""

        def _get_latest_commit() -> str:
            repo = pygit2.Repository(str(local_path))

            if branch_name == "HEAD":
                return str(repo.head.target)

            # Try local branch
            if branch_name in repo.branches.local:
                return str(repo.branches.local[branch_name].target)

            # Try remote branch
            remote_name = f"origin/{branch_name}"
            if remote_name in repo.branches.remote:
                return str(repo.branches.remote[remote_name].target)

            self._raise_branch_not_found_error(branch_name)
            raise AssertionError("unreachable")

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_latest_commit
        )

    def __del__(self) -> None:
        """Cleanup executor on deletion."""
        if hasattr(self, "_executor"):
            self._executor.shutdown(wait=True)

    async def get_all_tags(self, local_path: Path) -> list[dict[str, Any]]:
        """Get all tags in repository."""

        def _get_tags() -> list[dict[str, Any]]:
            repo = pygit2.Repository(str(local_path))
            tags: list[dict[str, Any]] = []

            for ref_name in repo.references:
                if ref_name.startswith("refs/tags/"):
                    tag_name = ref_name.replace("refs/tags/", "")
                    ref = repo.references[ref_name]

                    # Resolve the target (might be annotated tag or direct commit)
                    target = repo.get(ref.target)
                    if isinstance(target, pygit2.Tag):
                        # Annotated tag - get the commit it points to
                        commit_sha = str(target.target)
                    elif isinstance(target, pygit2.Commit):
                        # Lightweight tag - points directly to commit
                        commit_sha = str(target.id)
                    else:
                        continue

                    tags.append(
                        {
                            "name": tag_name,
                            "target_commit_sha": commit_sha,
                        }
                    )

            self._log.info(f"Getting all tags for {local_path}: {len(tags)}")
            return tags

        return await asyncio.get_event_loop().run_in_executor(self._executor, _get_tags)

    async def get_commit_diff(self, local_path: Path, commit_sha: str) -> str:
        """Get the diff for a specific commit."""

        def _get_diff() -> str:
            repo = pygit2.Repository(str(local_path))
            commit = repo.get(commit_sha)

            if not isinstance(commit, pygit2.Commit):
                raise TypeError(f"Object {commit_sha} is not a commit")

            if not commit.parents:
                # First commit - diff against empty tree
                diff = commit.tree.diff_to_tree(swap=True)
            else:
                # Diff against first parent
                diff = repo.diff(commit.parents[0], commit)

            return diff.patch or ""

        return await asyncio.get_event_loop().run_in_executor(self._executor, _get_diff)
