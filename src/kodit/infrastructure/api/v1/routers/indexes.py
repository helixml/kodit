"""Index management router for the REST API."""

from fastapi import APIRouter, Depends, HTTPException

from kodit.domain.entities import Task
from kodit.domain.value_objects import QueuePriority
from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import (
    IndexingAppServiceDep,
    IndexQueryServiceDep,
    QueueServiceDep,
)
from kodit.infrastructure.api.v1.schemas.index import (
    IndexAttributes,
    IndexCreateRequest,
    IndexData,
    IndexDetailResponse,
    IndexListResponse,
    IndexResponse,
)

router = APIRouter(
    prefix="/api/v1/indexes",
    tags=["indexes"],
    dependencies=[Depends(api_key_auth)],
    responses={
        401: {"description": "Unauthorized"},
        422: {"description": "Invalid request"},
    },
)


@router.get("")
async def list_indexes(
    query_service: IndexQueryServiceDep,
) -> IndexListResponse:
    """List all indexes."""
    indexes = await query_service.list_indexes()
    return IndexListResponse(
        data=[
            IndexData(
                type="index",
                id=str(idx.id),
                attributes=IndexAttributes(
                    created_at=idx.created_at,
                    updated_at=idx.updated_at,
                    uri=str(idx.source.working_copy.remote_uri),
                ),
            )
            for idx in indexes
        ]
    )


@router.post("", status_code=202)
async def create_index(
    request: IndexCreateRequest,
    app_service: IndexingAppServiceDep,
    queue_service: QueueServiceDep,
) -> IndexResponse:
    """Create a new index and start async indexing."""
    # Create index using the application service
    index = await app_service.create_index_from_uri(request.data.attributes.uri)

    # Add the indexing task to the queue
    await queue_service.enqueue_task(
        Task.create_index_update_task(index.id, QueuePriority.USER_INITIATED)
    )

    return IndexResponse(
        data=IndexData(
            type="index",
            id=str(index.id),
            attributes=IndexAttributes(
                created_at=index.created_at,
                updated_at=index.updated_at,
                uri=str(index.source.working_copy.remote_uri),
            ),
        )
    )


@router.get("/{index_id}", responses={404: {"description": "Index not found"}})
async def get_index(
    index_id: int,
    query_service: IndexQueryServiceDep,
) -> IndexDetailResponse:
    """Get index details."""
    index = await query_service.get_index_by_id(index_id)
    if not index:
        raise HTTPException(status_code=404, detail="Index not found")

    return IndexDetailResponse(
        data=IndexData(
            type="index",
            id=str(index.id),
            attributes=IndexAttributes(
                created_at=index.created_at,
                updated_at=index.updated_at,
                uri=str(index.source.working_copy.remote_uri),
            ),
        ),
    )


@router.delete(
    "/{index_id}", status_code=204, responses={404: {"description": "Index not found"}}
)
async def delete_index(
    index_id: int,
    query_service: IndexQueryServiceDep,
    app_service: IndexingAppServiceDep,
) -> None:
    """Delete an index."""
    index = await query_service.get_index_by_id(index_id)
    if not index:
        raise HTTPException(status_code=404, detail="Index not found")

    await app_service.delete_index(index)
