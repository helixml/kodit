"""Factory for creating Git adapters."""

from typing import Literal

from kodit.domain.protocols import GitAdapter
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.cloning.git.pygit2_adaptor import PyGit2Adapter


def create_git_adapter(
    provider: Literal["pygit2", "gitpython"] = "pygit2",
) -> GitAdapter:
    """Create a GitAdapter based on the specified provider."""
    if provider == "gitpython":
        return GitPythonAdapter()
    return PyGit2Adapter()
