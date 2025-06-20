"""Working copy provider for git-based sources."""

import hashlib
from pathlib import Path

import git
import structlog

from kodit.infrastructure.git.git_utils import sanitize_git_url


class GitWorkingCopyProvider:
    """Working copy provider for git-based sources."""

    def __init__(self, clone_dir: Path) -> None:
        """Initialize the provider."""
        self.clone_dir = clone_dir
        self.log = structlog.get_logger(__name__)

    async def prepare(self, uri: str) -> Path:
        """Prepare a Git working copy."""
        # Sanitize the URI for directory name to prevent credential leaks
        sanitized_uri = sanitize_git_url(uri)

        # Use a repeatable, short sha256 hash of the sanitized URI for the directory
        dir_hash = hashlib.sha256(sanitized_uri.encode("utf-8")).hexdigest()[:16]
        dir_name = f"repo-{dir_hash}"
        clone_path = self.clone_dir / dir_name
        clone_path.mkdir(parents=True, exist_ok=True)

        try:
            self.log.info(
                "Cloning repository", uri=sanitized_uri, clone_path=str(clone_path)
            )
            # Use the original URI for cloning (with credentials if present)
            git.Repo.clone_from(uri, clone_path)
        except git.GitCommandError as e:
            if "already exists and is not an empty directory" not in str(e):
                msg = f"Failed to clone repository: {e}"
                raise ValueError(msg) from e
            self.log.info("Repository already exists, reusing...", uri=sanitized_uri)

        return clone_path
