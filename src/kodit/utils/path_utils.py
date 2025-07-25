"""Path utilities for Python compatibility."""

import sys
from pathlib import Path
from urllib.parse import urlparse
from urllib.request import url2pathname


def path_from_uri(uri: str) -> Path:
    """Convert a file URI to a Path object.

    This provides backwards compatibility for Path.from_uri which is only
    available in Python 3.13+.

    Args:
        uri: File URI string (e.g., "file:///path/to/file")

    Returns:
        Path object representing the file path

    Raises:
        ValueError: If the URI is not a valid file URI

    """
    if sys.version_info >= (3, 13):
        # For Python 3.13+, delegate to the standard library but catch its ValueError
        # and re-raise with our format for consistency
        try:
            return Path.from_uri(uri)
        except ValueError as e:
            # Re-parse to get our own error format
            parsed = urlparse(uri)
            if not parsed.scheme:
                raise ValueError("Expected file URI, got scheme: ") from e
            if parsed.scheme != "file":
                raise ValueError(
                    f"Expected file URI, got scheme: {parsed.scheme}"
                ) from e
            # Re-raise original error if it's something else
            raise

    # Manual implementation for Python 3.12 and earlier
    parsed = urlparse(uri)

    if not parsed.scheme:
        raise ValueError("Expected file URI, got scheme: ")

    if parsed.scheme != "file":
        raise ValueError(f"Expected file URI, got scheme: {parsed.scheme}")

    # Convert URL path to local path
    path_str = url2pathname(parsed.path)

    return Path(path_str)
