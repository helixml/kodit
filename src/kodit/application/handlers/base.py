"""Base protocol for task handlers."""

from typing import Any, Protocol


class TaskHandler(Protocol):
    """Protocol for task operation handlers."""

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute the task operation with the given payload."""
        ...
