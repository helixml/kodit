"""Working copy provider for folder-based sources."""

import shutil
from pathlib import Path


class FolderWorkingCopyProvider:
    """Working copy provider for folder-based sources."""

    def __init__(self, clone_dir: Path) -> None:
        """Initialize the provider."""
        self.clone_dir = clone_dir

    async def prepare(self, uri: str) -> Path:
        """Prepare a folder working copy."""
        # Handle file:// URIs
        if uri.startswith("file://"):
            from urllib.parse import urlparse

            parsed = urlparse(uri)
            directory = Path(parsed.path).expanduser().resolve()
        else:
            directory = Path(uri).expanduser().resolve()

        # Clone into a local directory
        clone_path = self.clone_dir / directory.as_posix().replace("/", "_")
        clone_path.mkdir(parents=True, exist_ok=True)

        # Copy all files recursively, preserving directory structure, ignoring
        # hidden files
        shutil.copytree(
            directory,
            clone_path,
            ignore=shutil.ignore_patterns(".*"),
            dirs_exist_ok=True,
        )

        return clone_path
