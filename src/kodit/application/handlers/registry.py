"""Registry for task operation handlers."""

from kodit.application.handlers.base import TaskHandler
from kodit.domain.value_objects import TaskOperation


class TaskHandlerRegistry:
    """Registry mapping task operations to their handlers."""

    def __init__(self) -> None:
        """Initialize an empty handler registry."""
        self._handlers: dict[TaskOperation, TaskHandler] = {}

    def register(self, operation: TaskOperation, handler: TaskHandler) -> None:
        """Register a handler for a task operation."""
        self._handlers[operation] = handler

    def handler(self, operation: TaskOperation) -> TaskHandler:
        """Get the handler for a task operation."""
        if operation not in self._handlers:
            raise ValueError(f"No handler registered for operation: {operation}")
        return self._handlers[operation]
