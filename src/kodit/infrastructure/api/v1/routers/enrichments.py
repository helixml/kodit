"""Enrichment management router for the REST API."""

from dataclasses import replace
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Query, Request

from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import EnrichmentV2RepositoryDep
from kodit.infrastructure.api.v1.query_params import PaginationParamsDep
from kodit.infrastructure.api.v1.schemas.enrichment import (
    EnrichmentAttributes,
    EnrichmentData,
    EnrichmentLinks,
    EnrichmentListResponse,
    EnrichmentResponse,
    EnrichmentUpdateRequest,
)
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder

router = APIRouter(
    prefix="/api/v1/enrichments",
    tags=["enrichments"],
    dependencies=[Depends(api_key_auth)],
    responses={
        401: {"description": "Unauthorized"},
        422: {"description": "Invalid request"},
    },
)


@router.get("", summary="List enrichments")
async def list_enrichments(
    request: Request,
    enrichment_repository: EnrichmentV2RepositoryDep,
    pagination: PaginationParamsDep,
    enrichment_type: Annotated[
        str | None,
        Query(description="Filter by enrichment type"),
    ] = None,
    enrichment_subtype: Annotated[
        str | None,
        Query(description="Filter by enrichment subtype"),
    ] = None,
) -> EnrichmentListResponse:
    """List all enrichments with optional filtering."""
    query_builder = QueryBuilder()

    if enrichment_type:
        query_builder.filter("type", FilterOperator.EQ, enrichment_type)

    if enrichment_subtype:
        query_builder.filter("subtype", FilterOperator.EQ, enrichment_subtype)

    query_builder.paginate(pagination)

    enrichments = await enrichment_repository.find(query_builder)

    base_url = str(request.base_url).rstrip("/")

    return EnrichmentListResponse(
        data=[
            EnrichmentData(
                type="enrichment",
                id=str(enrichment.id) if enrichment.id else "0",
                attributes=EnrichmentAttributes(
                    type=enrichment.type,
                    subtype=enrichment.subtype,
                    content=enrichment.content,
                    created_at=enrichment.created_at,
                    updated_at=enrichment.updated_at,
                ),
                links=EnrichmentLinks.model_validate(
                    {"self": f"{base_url}/api/v1/enrichments/{enrichment.id}"}
                ),
            )
            for enrichment in enrichments
        ]
    )


@router.get(
    "/{enrichment_id}",
    summary="Get enrichment",
    responses={404: {"description": "Enrichment not found"}},
)
async def get_enrichment(
    enrichment_id: str,
    request: Request,
    enrichment_repository: EnrichmentV2RepositoryDep,
) -> EnrichmentResponse:
    """Get a specific enrichment by ID."""
    try:
        enrichment_id_int = int(enrichment_id)
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid enrichment ID") from None

    enrichment = await enrichment_repository.get(enrichment_id_int)
    if not enrichment:
        raise HTTPException(status_code=404, detail="Enrichment not found")

    base_url = str(request.base_url).rstrip("/")

    return EnrichmentResponse(
        data=EnrichmentData(
            type="enrichment",
            id=str(enrichment.id) if enrichment.id else "0",
            attributes=EnrichmentAttributes(
                type=enrichment.type,
                subtype=enrichment.subtype,
                content=enrichment.content,
                created_at=enrichment.created_at,
                updated_at=enrichment.updated_at,
            ),
            links=EnrichmentLinks.model_validate(
                {"self": f"{base_url}/api/v1/enrichments/{enrichment.id}"}
            ),
        )
    )


@router.patch(
    "/{enrichment_id}",
    summary="Update enrichment",
    responses={404: {"description": "Enrichment not found"}},
)
async def update_enrichment(
    enrichment_id: str,
    update_request: EnrichmentUpdateRequest,
    request: Request,
    enrichment_repository: EnrichmentV2RepositoryDep,
) -> EnrichmentResponse:
    """Update an enrichment's content."""
    try:
        enrichment_id_int = int(enrichment_id)
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid enrichment ID") from None

    enrichment = await enrichment_repository.get(enrichment_id_int)
    if not enrichment:
        raise HTTPException(status_code=404, detail="Enrichment not found")

    updated_enrichment = replace(
        enrichment, content=update_request.data.attributes.content
    )
    saved_enrichment = await enrichment_repository.save(updated_enrichment)

    base_url = str(request.base_url).rstrip("/")

    return EnrichmentResponse(
        data=EnrichmentData(
            type="enrichment",
            id=str(saved_enrichment.id) if saved_enrichment.id else "0",
            attributes=EnrichmentAttributes(
                type=saved_enrichment.type,
                subtype=saved_enrichment.subtype,
                content=saved_enrichment.content,
                created_at=saved_enrichment.created_at,
                updated_at=saved_enrichment.updated_at,
            ),
            links=EnrichmentLinks.model_validate(
                {"self": f"{base_url}/api/v1/enrichments/{saved_enrichment.id}"}
            ),
        )
    )


@router.delete(
    "/{enrichment_id}",
    status_code=204,
    summary="Delete enrichment",
    responses={404: {"description": "Enrichment not found"}},
)
async def delete_enrichment(
    enrichment_id: str,
    enrichment_repository: EnrichmentV2RepositoryDep,
) -> None:
    """Delete an enrichment."""
    try:
        enrichment_id_int = int(enrichment_id)
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid enrichment ID") from None

    enrichment = await enrichment_repository.get(enrichment_id_int)
    if not enrichment:
        raise HTTPException(status_code=404, detail="Enrichment not found")

    await enrichment_repository.delete(enrichment)
