"""Dulwich adapter for Git operations."""

import asyncio
import io
import mimetypes
import shutil
from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import structlog
from dulwich import porcelain
from dulwich.diff_tree import tree_changes
from dulwich.objects import Blob, Commit, Tag, Tree
from dulwich.repo import Repo


class DulwichAdapter:
    """Dulwich implementation of Git operations using porcelain methods."""

    def __init__(self, max_workers: int = 4) -> None:
        """Initialize Dulwich adapter."""
        self._log = structlog.getLogger(__name__)
        self._executor = ThreadPoolExecutor(max_workers=max_workers)

    def _raise_branch_not_found_error(self, branch_name: str) -> None:
        """Raise branch not found error."""
        raise ValueError(f"Branch {branch_name} not found")

    def _refs_contains(self, repo: Repo, ref: bytes) -> bool:
        """Check if ref exists in repository."""
        return ref in repo.refs  # type: ignore[operator]

    def _refs_get(self, repo: Repo, ref: bytes) -> bytes:
        """Get ref target from repository."""
        return repo.refs[ref]  # type: ignore[index]

    def _get_symref(self, repo: Repo, ref: bytes) -> bytes:
        """Get symbolic ref target."""
        symrefs = repo.refs.get_symrefs()
        if ref in symrefs:
            return symrefs[ref]  # type: ignore[index]
        raise KeyError(f"Symbolic ref {ref!r} not found")

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

            porcelain.clone(remote_uri, str(local_path))

            tags = list(porcelain.tag_list(str(local_path)))
            self._log.debug(f"Successfully cloned {remote_uri} with {len(tags)} tags")

        await asyncio.get_event_loop().run_in_executor(self._executor, _clone)

    async def checkout_commit(self, local_path: Path, commit_sha: str) -> None:
        """Checkout a specific commit."""

        def _checkout() -> None:
            self._log.debug(f"Checking out commit {commit_sha} in {local_path}")
            porcelain.reset(str(local_path), "hard", treeish=commit_sha.encode())
            self._log.debug(f"Successfully checked out {commit_sha}")

        await asyncio.get_event_loop().run_in_executor(self._executor, _checkout)

    async def checkout_branch(self, local_path: Path, branch_name: str) -> None:
        """Checkout a specific branch."""

        def _checkout() -> None:
            porcelain.checkout_branch(str(local_path), branch_name.encode())

        await asyncio.get_event_loop().run_in_executor(self._executor, _checkout)

    async def fetch_repository(self, local_path: Path) -> None:
        """Fetch latest changes for existing repository."""

        def _fetch() -> None:
            porcelain.fetch(str(local_path))

        await asyncio.get_event_loop().run_in_executor(self._executor, _fetch)

    async def pull_repository(self, local_path: Path) -> None:
        """Pull latest changes for existing repository."""

        def _pull() -> None:
            try:
                porcelain.pull(str(local_path))
                self._log.debug(f"Successfully pulled {local_path}")
            except porcelain.DivergedBranches:
                self._log.debug(
                    f"Non-fast-forward merge needed for {local_path}, skipping"
                )
            except (KeyError, ValueError, OSError) as e:
                self._log.debug(f"Pull skipped for {local_path}: {e}")

        await asyncio.get_event_loop().run_in_executor(self._executor, _pull)

    async def get_all_branches(self, local_path: Path) -> list[dict[str, Any]]:
        """Get all branches in repository."""

        def _get_branches() -> list[dict[str, Any]]:
            repo = Repo(str(local_path))
            branches: list[dict[str, Any]] = []

            # Get active branch using porcelain
            active_branch_name: str | None = None
            try:
                active = porcelain.active_branch(repo)
                if active:
                    active_branch_name = active.decode()
            except (KeyError, ValueError):
                pass

            # Get local branches using porcelain
            local_names: set[str] = set()
            for branch in porcelain.branch_list(repo):
                branch_name = branch.decode()
                local_names.add(branch_name)
                ref = f"refs/heads/{branch_name}".encode()
                branches.append(
                    {
                        "name": branch_name,
                        "type": "local",
                        "head_commit_sha": self._refs_get(repo, ref).hex(),
                        "is_active": branch_name == active_branch_name,
                    }
                )

            # Get remote branches - iterate refs directly
            for ref_name in repo.refs:
                if ref_name.startswith(b"refs/remotes/origin/"):
                    branch_name = ref_name[len(b"refs/remotes/origin/") :].decode()
                    if branch_name == "HEAD" or branch_name in local_names:
                        continue

                    branches.append(
                        {
                            "name": branch_name,
                            "type": "remote",
                            "head_commit_sha": self._refs_get(repo, ref_name).hex(),
                            "is_active": False,
                            "remote": "origin",
                        }
                    )

            return branches

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_branches
        )

    def _commit_to_dict(self, commit: Commit) -> dict[str, Any]:
        """Convert a dulwich commit to a dictionary."""
        parent_sha = ""
        if commit.parents:
            parent_sha = commit.parents[0].hex()

        author = commit.author.decode(errors="replace")
        committer = commit.committer.decode(errors="replace")

        return {
            "sha": commit.id.hex(),
            "date": datetime.fromtimestamp(commit.commit_time, UTC),
            "message": commit.message.decode(errors="replace").strip(),
            "parent_sha": parent_sha,
            "author_name": author.split(" <")[0],
            "author_email": self._extract_email(author),
            "committer_name": committer.split(" <")[0],
            "committer_email": self._extract_email(committer),
            "tree_sha": commit.tree.hex(),
        }

    def _extract_email(self, identity: str) -> str:
        """Extract email from git identity string."""
        if "<" in identity and ">" in identity:
            start = identity.index("<") + 1
            end = identity.index(">")
            return identity[start:end]
        return ""

    async def get_branch_commits(
        self, local_path: Path, branch_name: str
    ) -> list[dict[str, Any]]:
        """Get commit history for a specific branch."""

        def _get_commits() -> list[dict[str, Any]]:
            repo = Repo(str(local_path))

            target = self._resolve_branch(repo, branch_name)
            if target is None:
                self._raise_branch_not_found_error(branch_name)
                raise AssertionError("unreachable")

            return [
                self._commit_to_dict(entry.commit)
                for entry in repo.get_walker(include=[target])  # type: ignore[list-item]
            ]

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_commits
        )

    def _resolve_branch(self, repo: Repo, branch_name: str) -> bytes | None:
        """Resolve branch name to commit SHA."""
        local_ref = f"refs/heads/{branch_name}".encode()
        if self._refs_contains(repo, local_ref):
            return self._refs_get(repo, local_ref)

        remote_ref = f"refs/remotes/origin/{branch_name}".encode()
        if self._refs_contains(repo, remote_ref):
            return self._refs_get(repo, remote_ref)

        return None

    async def get_all_commits_bulk(
        self, local_path: Path, since_date: datetime | None = None
    ) -> dict[str, dict[str, Any]]:
        """Get all commits from all branches in bulk for efficiency."""

        def _get_all_commits() -> dict[str, dict[str, Any]]:
            repo = Repo(str(local_path))
            head_commits = self._collect_head_commits(repo)

            commits_map: dict[str, dict[str, Any]] = {}
            seen: set[bytes] = set()

            for head in head_commits:
                for entry in repo.get_walker(include=[head]):  # type: ignore[list-item]
                    commit = entry.commit
                    if commit.id in seen:
                        continue
                    seen.add(commit.id)

                    if since_date is not None:
                        commit_time = datetime.fromtimestamp(commit.commit_time, UTC)
                        if commit_time < since_date:
                            continue

                    commits_map[commit.id.hex()] = self._commit_to_dict(commit)

            if since_date:
                self._log.info(
                    "Getting commits since %s, found %d commits",
                    since_date.strftime("%Y-%m-%d %H:%M:%S"),
                    len(commits_map),
                )

            return commits_map

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_all_commits
        )

    def _collect_head_commits(self, repo: Repo) -> list[bytes]:
        """Collect all head commits from refs."""
        head_commits: list[bytes] = []
        for ref_name in repo.refs:
            if ref_name == b"HEAD":
                continue
            target = self._refs_get(repo, ref_name)
            obj = repo[target]
            while isinstance(obj, Tag):
                obj = repo[obj.object[1]]
            if isinstance(obj, Commit):
                head_commits.append(target)
        return head_commits

    async def get_branch_commit_shas(
        self, local_path: Path, branch_name: str
    ) -> list[str]:
        """Get only commit SHAs for a branch."""

        def _get_commit_shas() -> list[str]:
            repo = Repo(str(local_path))

            target = self._resolve_branch(repo, branch_name)
            if target is None:
                self._raise_branch_not_found_error(branch_name)
                raise AssertionError("unreachable")

            return [
                entry.commit.id.hex()
                for entry in repo.get_walker(include=[target])  # type: ignore[list-item]
            ]

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_commit_shas
        )

    async def get_all_branch_head_shas(
        self, local_path: Path, branch_names: list[str]
    ) -> dict[str, str]:
        """Get head commit SHAs for all branches in one operation."""

        def _get_all_head_shas() -> dict[str, str]:
            repo = Repo(str(local_path))
            result: dict[str, str] = {}

            for branch_name in branch_names:
                target = self._resolve_branch(repo, branch_name)
                if target is not None:
                    result[branch_name] = target.hex()
                else:
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
            repo = Repo(str(local_path))
            commit_bytes = bytes.fromhex(commit_sha)
            commit = repo[commit_bytes]

            if not isinstance(commit, Commit):
                raise TypeError(f"Object {commit_sha} is not a commit")

            files: list[dict[str, Any]] = []

            def process_tree(tree: Tree, prefix: str = "") -> None:
                for entry in tree.items():
                    name = entry.path.decode(errors="replace")
                    entry_path = f"{prefix}{name}" if prefix else name
                    mode = entry.mode
                    sha = entry.sha

                    obj = repo[sha]
                    if isinstance(obj, Tree):
                        process_tree(obj, f"{entry_path}/")
                    elif isinstance(obj, Blob):
                        mime_type = mimetypes.guess_type(entry_path)[0]
                        if not mime_type:
                            mime_type = "application/octet-stream"
                        files.append(
                            {
                                "path": entry_path,
                                "blob_sha": sha.hex(),
                                "size": len(obj.data),
                                "mode": oct(mode),
                                "mime_type": mime_type,
                                "created_at": datetime.fromtimestamp(
                                    commit.commit_time, UTC
                                ),
                            }
                        )

            tree = repo[commit.tree]
            if isinstance(tree, Tree):
                process_tree(tree)
            return files

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_files
        )

    async def repository_exists(self, local_path: Path) -> bool:
        """Check if repository exists at local path."""

        def _check_exists() -> bool:
            try:
                Repo(str(local_path))
            except (FileNotFoundError, OSError, ValueError):
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
            repo = Repo(str(local_path))
            commit_bytes = bytes.fromhex(commit_sha)
            commit = repo[commit_bytes]

            if not isinstance(commit, Commit):
                raise TypeError(f"Object {commit_sha} is not a commit")

            result = self._commit_to_dict(commit)

            # Use tree_changes for stats
            if commit.parents:
                parent = repo[commit.parents[0]]
                if isinstance(parent, Commit):
                    changes = list(
                        tree_changes(repo.object_store, parent.tree, commit.tree)
                    )
                else:
                    changes = []
            else:
                changes = list(tree_changes(repo.object_store, None, commit.tree))

            insertions = 0
            deletions = 0
            files_changed = len(changes)

            for change in changes:
                old_sha = change.old.sha if change.old else None
                new_sha = change.new.sha if change.new else None

                old_lines = self._get_blob_lines(repo, old_sha)
                new_lines = self._get_blob_lines(repo, new_sha)

                if old_sha is None:
                    insertions += len(new_lines)
                elif new_sha is None:
                    deletions += len(old_lines)
                else:
                    old_set = set(old_lines)
                    new_set = set(new_lines)
                    insertions += len(new_set - old_set)
                    deletions += len(old_set - new_set)

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

    def _get_blob_lines(self, repo: Repo, sha: bytes | None) -> list[bytes]:
        """Get lines from a blob."""
        if sha is None:
            return []
        try:
            obj = repo[sha]
            if isinstance(obj, Blob):
                return obj.data.splitlines()
        except (KeyError, ValueError):
            pass
        return []

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
            repo = Repo(str(local_path))
            # Use porcelain.get_object_by_path with committish parameter
            obj = porcelain.get_object_by_path(
                repo, file_path, committish=commit_sha.encode()
            )
            if isinstance(obj, Blob):
                return obj.data
            msg = f"File {file_path} not found in commit {commit_sha}"
            raise FileNotFoundError(msg)

        return await asyncio.get_event_loop().run_in_executor(
            self._executor, _get_file_content
        )

    def _get_default_branch_sync(self, local_path: Path) -> str:
        """Detect the default branch with fallback strategies."""
        repo = Repo(str(local_path))

        # Check if origin remote exists
        config = repo.get_config()
        try:
            config.get((b"remote", b"origin"), b"url")
        except KeyError as err:
            raise ValueError(f"Repository {local_path} has no origin remote") from err

        # Strategy 1: Try origin/HEAD symbolic reference
        head_ref = b"refs/remotes/origin/HEAD"
        if self._refs_contains(repo, head_ref):
            try:
                target = self._get_symref(repo, head_ref)
                return target.decode().replace("refs/remotes/origin/", "")
            except KeyError:
                pass

        # Strategy 2: Look for common default branch names
        remote_branches: list[str] = []
        for ref_name in repo.refs:
            if ref_name.startswith(b"refs/remotes/origin/"):
                branch_name = ref_name[len(b"refs/remotes/origin/") :].decode()
                if branch_name != "HEAD":
                    remote_branches.append(branch_name)

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
            repo = Repo(str(local_path))

            if branch_name == "HEAD":
                return repo.head().hex()

            target = self._resolve_branch(repo, branch_name)
            if target is not None:
                return target.hex()

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
            repo = Repo(str(local_path))
            tags: list[dict[str, Any]] = []

            # Use porcelain.tag_list
            for tag_name in porcelain.tag_list(repo):
                tag_ref = b"refs/tags/" + tag_name
                target_sha = self._refs_get(repo, tag_ref)

                obj = repo[target_sha]
                if isinstance(obj, Tag):
                    commit_sha = obj.object[1].hex()
                elif isinstance(obj, Commit):
                    commit_sha = obj.id.hex()
                else:
                    continue

                tags.append(
                    {
                        "name": tag_name.decode(),
                        "target_commit_sha": commit_sha,
                    }
                )

            self._log.info(f"Getting all tags for {local_path}: {len(tags)}")
            return tags

        return await asyncio.get_event_loop().run_in_executor(self._executor, _get_tags)

    async def get_commit_diff(self, local_path: Path, commit_sha: str) -> str:
        """Get the diff for a specific commit."""

        def _get_diff() -> str:
            repo = Repo(str(local_path))
            commit_bytes = bytes.fromhex(commit_sha)
            commit = repo[commit_bytes]

            if not isinstance(commit, Commit):
                raise TypeError(f"Object {commit_sha} is not a commit")

            # Use porcelain.diff_tree to get formatted diff output
            output = io.BytesIO()
            if not commit.parents:
                # For first commit, diff against empty tree
                empty_tree = Tree()
                porcelain.diff_tree(
                    repo,
                    empty_tree,
                    commit_bytes,
                    outstream=output,
                )
            else:
                porcelain.diff_tree(
                    repo, commit.parents[0], commit_bytes, outstream=output
                )

            return output.getvalue().decode(errors="replace")

        return await asyncio.get_event_loop().run_in_executor(self._executor, _get_diff)
