"""Git utilities for infrastructure operations."""

import tempfile
from urllib.parse import urlparse, urlunparse

import git


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
        except git.GitCommandError:
            return False
        else:
            return True


def sanitize_git_url(url: str) -> str:
    """Remove credentials from a git URL while preserving the rest of the URL structure.

    This function handles various git URL formats:
    - HTTPS URLs with username:password@host
    - HTTPS URLs with username@host (no password)
    - SSH URLs (left unchanged)
    - File URLs (left unchanged)

    Args:
        url: The git URL that may contain credentials.

    Returns:
        The sanitized URL with credentials removed.

    Examples:
        >>> sanitize_git_url("https://phil:token@dev.azure.com/org/project/_git/repo")
        "https://dev.azure.com/org/project/_git/repo"
        >>> sanitize_git_url("https://username@github.com/user/repo.git")
        "https://github.com/user/repo.git"
        >>> sanitize_git_url("git@github.com:user/repo.git")
        "git@github.com:user/repo.git"

    """
    # Handle SSH URLs (they don't have credentials in the URL format)
    if url.startswith(("git@", "ssh://")):
        return url

    # Handle file URLs
    if url.startswith("file://"):
        return url

    try:
        # Parse the URL
        parsed = urlparse(url)

        # If there are no credentials, return the URL as-is
        if not parsed.username:
            return url

        # Reconstruct the URL without credentials
        # Keep scheme, netloc (without username/password), path, params, query, fragment
        sanitized_netloc = parsed.hostname
        if parsed.port:
            sanitized_netloc = f"{parsed.hostname}:{parsed.port}"

        return urlunparse(
            (
                parsed.scheme,
                sanitized_netloc,
                parsed.path,
                parsed.params,
                parsed.query,
                parsed.fragment,
            )
        )

    except Exception:  # noqa: BLE001
        # If URL parsing fails, return the original URL
        return url
