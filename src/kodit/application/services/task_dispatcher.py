"""Task dispatcher for routing tasks to appropriate handlers."""

from kodit.application.services.task_handlers import (
    CommitTaskHandler,
    RepositoryTaskHandler,
    TaskHandler,
)
from kodit.domain.entities import Task


class TaskDispatcher:
    """Dispatches tasks to appropriate handlers using the Command Pattern."""

    def __init__(
        self,
        repository_handler: RepositoryTaskHandler,
        commit_handler: CommitTaskHandler,
    ) -> None:
        """Initialize the task dispatcher."""
        self.repository_handler = repository_handler
        self.commit_handler = commit_handler

    async def dispatch(self, task: Task) -> None:
        """Dispatch a task to the appropriate handler."""
        handler = self._get_handler(task)
        await handler.handle(task)

    def _get_handler(self, task: Task) -> TaskHandler:
        """Get the appropriate handler for a task."""
        if task.type.is_repository_operation():
            return self.repository_handler
        if task.type.is_commit_operation():
            return self.commit_handler
        raise ValueError(f"Unknown task type: {task.type}")
