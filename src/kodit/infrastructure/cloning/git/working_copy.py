"""Working copy provider for git-based sources."""

import asyncio
import hashlib
import shutil
from pathlib import Path

import structlog

import git
from kodit.application.factories.reporting_factory import create_noop_operation
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities import WorkingCopy


class GitWorkingCopyProvider:
    """Working copy provider for git-based sources."""

    def __init__(self, clone_dir: Path) -> None:
        """Initialize the provider."""
        self.clone_dir = clone_dir
        self.log = structlog.get_logger(__name__)

    def get_clone_path(self, uri: str) -> Path:
        """Get the clone path for a Git working copy."""
        sanitized_uri = WorkingCopy.sanitize_git_url(uri)
        dir_hash = hashlib.sha256(str(sanitized_uri).encode("utf-8")).hexdigest()[:16]
        dir_name = f"repo-{dir_hash}"
        return self.clone_dir / dir_name

    async def prepare(
        self,
        uri: str,
        step: ProgressTracker | None = None,
    ) -> Path:
        """Prepare a Git working copy."""
        step = step or create_noop_operation()
        sanitized_uri = WorkingCopy.sanitize_git_url(uri)
        clone_path = self.get_clone_path(uri)
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

        return clone_path

    async def sync(self, uri: str, step: ProgressTracker | None = None) -> Path:
        """Refresh a Git working copy."""
        step = step or create_noop_operation()
        clone_path = self.get_clone_path(uri)

        # Check if the clone directory exists and is a valid Git repository
        if not clone_path.exists() or not (clone_path / ".git").exists():
            self.log.info(
                "Clone directory does not exist or is not a Git repository, "
                "preparing...",
                uri=uri,
                clone_path=str(clone_path),
            )
            return await self.prepare(uri, step)

        try:
            repo = git.Repo(clone_path)
            repo.remotes.origin.pull()
        except git.InvalidGitRepositoryError:
            self.log.warning(
                "Invalid Git repository found, re-cloning...",
                uri=uri,
                clone_path=str(clone_path),
            )
            # Remove the invalid directory and re-clone
            shutil.rmtree(clone_path)
            return await self.prepare(uri, step)

        return clone_path
