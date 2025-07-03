"""Infrastructure implementation of ignore pattern provider."""

from pathlib import Path

import git
import pathspec

from kodit.infrastructure.git.git_utils import is_valid_clone_target


class GitIgnorePatternProvider:
    """Ignore pattern provider for git repositories."""

    def __init__(self, base_dir: Path) -> None:
        """Initialize the ignore pattern provider.

        Args:
            base_dir: The base directory to check for ignore patterns.

        Raises:
            ValueError: If the base directory is not a directory.

        """
        if not base_dir.is_dir():
            msg = f"Base directory is not a directory: {base_dir}"
            raise ValueError(msg)

        self.base_dir = base_dir

        # Check if the base_dir is a valid git repository
        self.git_repo = None
        if is_valid_clone_target(str(base_dir)):
            self.git_repo = git.Repo(base_dir)

    def should_ignore(self, path: Path) -> bool:
        """Check if a path should be ignored.

        Args:
            path: The path to check.

        Returns:
            True if the path should be ignored, False otherwise.

        """
        if path.is_dir():
            return False

        # Get the path relative to the base_dir
        relative_path = path.relative_to(self.base_dir)

        # If this file is _part_ of a .git directory, then it should be ignored
        if relative_path.as_posix().startswith(".git"):
            return True

        # If it is a git repository, then we need to check if the file is ignored
        if self.git_repo and len(self.git_repo.ignored(path)) > 0:
            return True

        # If the repo has a .noindex file
        noindex_path = Path(self.base_dir / ".noindex")
        if noindex_path.exists():
            with noindex_path.open() as f:
                patterns = [line.strip() for line in f if line.strip()]
                if patterns:
                    spec = pathspec.PathSpec.from_lines("gitwildmatch", patterns)
                    if spec.match_file(relative_path.as_posix()):
                        return True

        return False
