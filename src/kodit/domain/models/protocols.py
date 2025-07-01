"""Repository protocol interfaces for the domain layer."""

from typing import Protocol

from pydantic import AnyUrl

from .entities import File, Index, Snippet, WorkingCopy


class IndexRepository(Protocol):
    """Repository interface for Index entities."""

    async def create(self, uri: AnyUrl) -> Index:
        """Create an index for a source."""
        ...

    async def get(self, id: int) -> Index | None:
        """Get an index by ID."""
        ...

    async def get_by_uri(self, uri: AnyUrl) -> Index | None:
        """Get an index by source URI."""
        ...

    async def set_working_copy(self, index_id: int, working_copy: WorkingCopy) -> None:
        """Set the working copy for an index."""
        ...

    async def add_files(self, index_id: int, files: list[File]) -> None:
        """Add files to an index."""
        ...

    async def add_snippets(self, index_id: int, snippets: list[Snippet]) -> None:
        """Add snippets to an index."""
        ...
