"""Repository protocol interfaces for the domain layer."""

from collections.abc import Generator, Sequence
from contextlib import contextmanager
from types import TracebackType
from typing import Protocol

from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Index, Snippet, SnippetWithContext, Task, WorkingCopy
from kodit.domain.value_objects import (
    MultiSearchRequest,
    OperationAggregate,
    Step,
    TaskType,
)


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


class ReportingStep(Protocol):
    """Reporting step."""

    def update_step_progress(self, step: Step) -> None:
        """Update the progress of the current step."""
        ...


class ReportingService(Protocol):
    """Reporting service."""

    def start_operation(self, operation: OperationAggregate) -> None:
        """Start tracking a new operation with steps."""
        ...

    def update_step(self, operation: OperationAggregate, step: Step) -> None:
        """Update the current step of an operation."""
        ...

    def complete_operation(self, operation: OperationAggregate) -> None:
        """Mark the current operation as completed."""
        ...

    def fail_operation(self, operation: OperationAggregate, error: Exception) -> None:
        """Mark the current operation as failed."""
        ...

    @contextmanager
    def reporting_step_context(
        self, operation: OperationAggregate
    ) -> Generator[ReportingStep, None, None]:
        """Context manager for a reporting step."""
        ...


class OperationRepository(Protocol):
    """Repository interface for Task status entities."""

    async def get_by_index_id(self, index_id: int) -> OperationAggregate:
        """Get a task status by index ID. Raises exception if not found."""
        ...

    async def save(self, operation: OperationAggregate) -> None:
        """Save a task status."""
        ...


class UnitOfWork(Protocol):
    """Unit of Work protocol for managing database lifecycle only.

    Repository creation should be handled externally.
    """

    @property
    def session(self) -> AsyncSession:
        """Get the current database session."""
        ...

    async def __aenter__(self) -> "UnitOfWork":
        """Enter the unit of work context."""
        ...

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc_val: BaseException | None,
        exc_tb: TracebackType | None,
    ) -> None:
        """Exit the unit of work context."""
        ...

    async def commit(self) -> None:
        """Commit the current transaction."""
        ...

    async def rollback(self) -> None:
        """Rollback the current transaction."""
        ...

    async def flush(self) -> None:
        """Flush pending changes to the database without committing."""
        ...
