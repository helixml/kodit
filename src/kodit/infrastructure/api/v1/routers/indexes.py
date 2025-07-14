"""Index management router for the REST API."""

from datetime import UTC, datetime

from fastapi import APIRouter, BackgroundTasks, HTTPException

from kodit.infrastructure.api.v1.dependencies import (
    IndexingAppServiceDep,
    IndexQueryServiceDep,
)
from kodit.infrastructure.api.v1.schemas.index import (
    IndexAttributes,
    IndexCreateRequest,
    IndexData,
    IndexDetailResponse,
    IndexListResponse,
    IndexResponse,
    SourceAttributes,
    SourceIncluded,
)

router = APIRouter(prefix="/api/v1/indexes", tags=["indexes"])


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
    background_tasks: BackgroundTasks,
    app_service: IndexingAppServiceDep,
) -> IndexResponse:
    """Create a new index and start async indexing."""
    # Create index using the application service
    index = await app_service.create_index_from_uri(request.data.attributes.source_uri)

    # Start async indexing in background
    background_tasks.add_task(app_service.run_index, index)

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


@router.get("/{index_id}")
async def get_index(
    index_id: str,
    query_service: IndexQueryServiceDep,
) -> IndexDetailResponse:
    """Get index details."""
    try:
        index_id_int = int(index_id)
    except ValueError as e:
        raise HTTPException(status_code=400, detail="Invalid index ID") from e

    index = await query_service.get_index_by_id(index_id_int)
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
        included=[
            SourceIncluded(
                type="source",
                id=str(index.source.id),
                attributes=SourceAttributes(
                    created_at=index.source.created_at or datetime.now(UTC),
                    updated_at=index.source.updated_at or datetime.now(UTC),
                    remote_uri=str(index.source.working_copy.remote_uri),
                    cloned_path=str(index.source.working_copy.cloned_path),
                    source_type=index.source.working_copy.source_type.name.lower(),
                ),
            )
        ],
    )


# TODO: Add delete endpoint
