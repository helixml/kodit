"""Git utilities for infrastructure operations."""

import tempfile

import git
import structlog


# FUTURE: move to clone dir
def is_valid_clone_target(target: str) -> bool:
    """Return True if the target is clonable.

    Args:
        target: The git repository URL or path to validate.

    Returns:
        True if the target can be cloned, False otherwise.

    """
    with tempfile.TemporaryDirectory() as temp_dir:
        try:
            git.Repo.clone_from(target, temp_dir)
        except git.GitCommandError as e:
            structlog.get_logger(__name__).warning(
                "Failed to clone git repository",
                target=target,
                error=e,
            )
            return False
        else:
            return True
