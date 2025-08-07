"""Repository protocol interfaces for the domain layer."""

from collections.abc import Sequence
from typing import Protocol

from pydantic import AnyUrl

from kodit.domain.entities import Index, Snippet, SnippetWithContext, Task, WorkingCopy
from kodit.domain.value_objects import MultiSearchRequest, TaskType


class TaskRepository(Protocol):
    """Repository interface for Task entities."""

    async def add(
        self,
        task: Task,
    ) -> None:
        """Add a task."""
        ...

    async def get(self, task_id: str) -> Task | None:
        """Get a task by ID."""
        ...

    async def take(self) -> Task | None:
        """Take a task for processing."""
        ...

    async def update(self, task: Task) -> None:
        """Update a task."""
        ...

    async def list(self, task_type: TaskType | None = None) -> list[Task]:
        """List tasks with optional status filter."""
        ...


class IndexRepository(Protocol):
    """Repository interface for Index entities."""

    async def create(self, uri: AnyUrl, working_copy: WorkingCopy) -> Index:
        """Create an index for a source."""
        ...

    async def update(self, index: Index) -> None:
        """Update an index."""
        ...

    async def get(self, index_id: int) -> Index | None:
        """Get an index by ID."""
        ...

    async def delete(self, index: Index) -> None:
        """Delete an index."""
        ...

    async def all(self) -> list[Index]:
        """List all indexes."""
        ...

    async def get_by_uri(self, uri: AnyUrl) -> Index | None:
        """Get an index by source URI."""
        ...

    async def update_index_timestamp(self, index_id: int) -> None:
        """Update the timestamp of an index."""
        ...

    async def add_snippets(self, index_id: int, snippets: list[Snippet]) -> None:
        """Add snippets to an index."""
        ...

    async def update_snippets(self, index_id: int, snippets: list[Snippet]) -> None:
        """Update snippets for an index."""
        ...

    async def delete_snippets(self, index_id: int) -> None:
        """Delete all snippets from an index."""
        ...

    async def delete_snippets_by_file_ids(self, file_ids: list[int]) -> None:
        """Delete snippets by file IDs."""
        ...

    async def search(self, request: MultiSearchRequest) -> Sequence[SnippetWithContext]:
        """Search snippets with filters."""
        ...

    async def get_snippets_by_ids(self, ids: list[int]) -> list[SnippetWithContext]:
        """Get snippets by their IDs."""
        ...
