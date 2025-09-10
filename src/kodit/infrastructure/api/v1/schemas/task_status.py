"""JSON:API schemas for task status operations."""

from datetime import datetime

from pydantic import BaseModel, Field


class TaskStatusAttributes(BaseModel):
    """Task status attributes for JSON:API responses."""

    step: str = Field(..., description="Name of the task/operation")
    state: str = Field(..., description="Current state of the task")
    progress: float = Field(
        default=0.0, ge=0.0, le=100.0, description="Progress percentage (0-100)"
    )
    total: int = Field(default=0, description="Total number of items to process")
    current: int = Field(default=0, description="Current number of items processed")
    created_at: datetime | None = Field(default=None, description="Task start time")
    updated_at: datetime | None = Field(default=None, description="Last update time")
    error: str = Field(default="", description="Error message")
    message: str = Field(default="", description="Message")


class TaskStatusData(BaseModel):
    """Task status data for JSON:API responses."""

    type: str = "task_status"
    id: str
    attributes: TaskStatusAttributes


class TaskStatusResponse(BaseModel):
    """JSON:API response for single task status."""

    data: TaskStatusData


class TaskStatusListResponse(BaseModel):
    """JSON:API response for task status list."""

    data: list[TaskStatusData]
