"""JSON:API schemas for queue operations."""

from datetime import datetime

from pydantic import BaseModel


class TaskAttributes(BaseModel):
    """Task attributes for JSON:API responses."""

    type: str
    priority: int
    payload: dict
    created_at: datetime | None
    updated_at: datetime | None


class TaskData(BaseModel):
    """Task data for JSON:API responses."""

    type: str = "task"
    id: str
    attributes: TaskAttributes


class TaskResponse(BaseModel):
    """JSON:API response for single task."""

    data: TaskData


class TaskListResponse(BaseModel):
    """JSON:API response for task list."""

    data: list[TaskData]
