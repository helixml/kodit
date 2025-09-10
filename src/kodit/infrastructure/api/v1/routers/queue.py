"""Queue management router for the REST API."""

from fastapi import APIRouter, Depends, HTTPException

from kodit.domain.value_objects import TaskOperation
from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import QueueServiceDep
from kodit.infrastructure.api.v1.schemas.queue import (
    TaskAttributes,
    TaskData,
    TaskListResponse,
    TaskResponse,
)

router = APIRouter(
    prefix="/api/v1/queue",
    tags=["queue"],
    dependencies=[Depends(api_key_auth)],
    responses={
        401: {"description": "Unauthorized"},
        422: {"description": "Invalid request"},
    },
)


@router.get("")
async def list_queue_tasks(
    queue_service: QueueServiceDep,
    task_type: TaskOperation | None = None,
) -> TaskListResponse:
    """List all tasks in the queue.

    Optionally filter by task type.
    """
    tasks = await queue_service.list_tasks(task_type)
    return TaskListResponse(
        data=[
            TaskData(
                type="task",
                id=task.id,
                attributes=TaskAttributes(
                    type=str(task.type),
                    priority=task.priority,
                    payload=task.payload,
                    created_at=task.created_at,
                    updated_at=task.updated_at,
                ),
            )
            for task in tasks
        ]
    )


@router.get("/{task_id}", responses={404: {"description": "Task not found"}})
async def get_queue_task(
    task_id: str,
    queue_service: QueueServiceDep,
) -> TaskResponse:
    """Get details of a specific task in the queue."""
    task = await queue_service.get_task(task_id)
    if not task:
        raise HTTPException(status_code=404, detail="Task not found")

    return TaskResponse(
        data=TaskData(
            type="task",
            id=task.id,
            attributes=TaskAttributes(
                type=str(task.type),
                priority=task.priority,
                payload=task.payload,
                created_at=task.created_at,
                updated_at=task.updated_at,
            ),
        )
    )
