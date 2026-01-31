"""Repository preparation for SWE-bench benchmarking."""

import shutil
import subprocess
import time
from pathlib import Path

import httpx
import structlog

from benchmark.swebench.instance import SWEBenchInstance

DEFAULT_REPOS_DIR = Path("benchmarks/repos")
DEFAULT_CLONE_TIMEOUT = 600  # 10 minutes for large repos
DEFAULT_INDEX_TIMEOUT = 7200  # 2 hours for indexing


class RepositoryPreparer:
    """Prepares repositories for SWE-bench benchmarking."""

    def __init__(
        self,
        kodit_base_url: str,
        repos_dir: Path = DEFAULT_REPOS_DIR,
        clone_timeout: int = DEFAULT_CLONE_TIMEOUT,
        index_timeout: int = DEFAULT_INDEX_TIMEOUT,
    ) -> None:
        """Initialize repository preparer."""
        self._kodit_base_url = kodit_base_url.rstrip("/")
        self._repos_dir = repos_dir
        self._clone_timeout = clone_timeout
        self._index_timeout = index_timeout
        self._log = structlog.get_logger(__name__)

    def prepare(self, instance: SWEBenchInstance) -> int:
        """Prepare a repository for benchmarking.

        Clones the repository at the exact commit and indexes it with Kodit.
        Returns the Kodit repository ID.
        """
        repo_path = self._clone_at_commit(instance)
        repo_id = self._index_repository(repo_path, instance.instance_id)
        self._wait_for_indexing(repo_id)
        return repo_id

    def _clone_at_commit(self, instance: SWEBenchInstance) -> Path:
        """Clone repository at exact commit."""
        repo_path = self._repos_dir / instance.instance_id

        if repo_path.exists():
            self._log.info("Removing existing repository", path=str(repo_path))
            shutil.rmtree(repo_path)

        repo_path.parent.mkdir(parents=True, exist_ok=True)

        github_url = f"https://github.com/{instance.repo}"

        self._log.info(
            "Cloning repository",
            url=github_url,
            commit=instance.base_commit,
            path=str(repo_path),
        )

        # Clone with full history to ensure we can checkout any commit
        result = subprocess.run(  # noqa: S603
            ["git", "clone", github_url, str(repo_path)],  # noqa: S607
            capture_output=True,
            text=True,
            timeout=self._clone_timeout,
            check=False,
        )

        if result.returncode != 0:
            msg = f"Failed to clone repository: {result.stderr}"
            raise RepositoryCloneError(msg)

        # Create a branch at the exact commit (not detached HEAD).
        # This ensures dulwich fetches all objects when Kodit clones from file:// URL.
        result = subprocess.run(  # noqa: S603
            ["git", "checkout", "-b", "kodit-index", instance.base_commit],  # noqa: S607
            cwd=str(repo_path),
            capture_output=True,
            text=True,
            check=False,
        )

        if result.returncode != 0:
            msg = f"Failed to create branch at commit {instance.base_commit}: {result.stderr}"
            raise RepositoryCloneError(msg)

        self._log.info(
            "Repository cloned at commit",
            instance_id=instance.instance_id,
            commit=instance.base_commit[:12],
        )

        return repo_path

    def _index_repository(self, repo_path: Path, instance_id: str) -> int:
        """Index repository with Kodit and return repository ID."""
        self._log.info(
            "Indexing repository with Kodit",
            path=str(repo_path),
            instance_id=instance_id,
        )

        # Use file:// URI for local repository
        absolute_path = repo_path.resolve()
        remote_uri = f"file://{absolute_path}"

        payload = {
            "data": {
                "type": "repository",
                "attributes": {"remote_uri": remote_uri},
            }
        }

        url = f"{self._kodit_base_url}/api/v1/repositories"

        response = httpx.post(url, json=payload, timeout=30.0)

        if response.status_code not in (200, 201):
            msg = (
                f"Failed to create repository: {response.status_code} - {response.text}"
            )
            raise RepositoryIndexError(msg)

        data = response.json()
        repo_id = int(data["data"]["id"])

        self._log.info(
            "Repository created in Kodit",
            repo_id=repo_id,
            instance_id=instance_id,
            created=response.status_code == 201,
        )

        return repo_id

    def _wait_for_indexing(self, repo_id: int) -> None:
        """Wait for repository indexing to complete."""
        url = f"{self._kodit_base_url}/api/v1/repositories/{repo_id}/status/summary"
        deadline = time.monotonic() + self._index_timeout
        poll_interval = 5.0  # seconds
        status_timeout = 60.0  # longer timeout for busy servers

        self._log.info(
            "Waiting for indexing to complete",
            repo_id=repo_id,
            timeout=self._index_timeout,
        )

        last_status = None
        while time.monotonic() < deadline:
            try:
                response = httpx.get(url, timeout=status_timeout)
            except httpx.TimeoutException:
                self._log.warning(
                    "Status check timed out, retrying",
                    repo_id=repo_id,
                )
                time.sleep(poll_interval)
                continue

            if response.status_code != 200:
                self._log.warning(
                    "Failed to get status",
                    repo_id=repo_id,
                    status_code=response.status_code,
                )
                time.sleep(poll_interval)
                continue

            data = response.json()
            status = data["data"]["attributes"]["status"]

            if status != last_status:
                self._log.info(
                    "Indexing status",
                    repo_id=repo_id,
                    status=status,
                )
                last_status = status

            if status == "completed":
                self._log.info("Indexing completed", repo_id=repo_id)
                return

            if status == "failed":
                message = data["data"]["attributes"].get("message", "Unknown error")
                msg = f"Indexing failed: {message}"
                raise RepositoryIndexError(msg)

            time.sleep(poll_interval)

        msg = f"Indexing timed out after {self._index_timeout} seconds"
        raise RepositoryIndexError(msg)


class RepositoryCloneError(Exception):
    """Raised when repository cloning fails."""


class RepositoryIndexError(Exception):
    """Raised when repository indexing fails."""
