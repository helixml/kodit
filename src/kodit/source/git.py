"""Git utilities."""

import tempfile

import git


def is_valid_clone_target(target: str) -> bool:
    """Return True if the target is clonable."""
    with tempfile.TemporaryDirectory() as temp_dir:
        try:
            git.Repo.clone_from(target, temp_dir)
        except git.GitCommandError:
            return False
        else:
            return True
