"""Search router for the REST API."""

from fastapi import APIRouter, HTTPException

from kodit.domain.value_objects import MultiSearchRequest, SnippetSearchFilters
from kodit.infrastructure.api.v1.dependencies import (
    CodeSearchAppServiceDep,
    RepositoryQueryServiceDep,
)
from kodit.infrastructure.api.v1.schemas.search import (
    SearchRequest,
    SearchResponse,
    SnippetAttributes,
    SnippetData,
)
from kodit.infrastructure.api.v1.schemas.snippet import (
    EnrichmentSchema,
    GitFileSchema,
    SnippetContentSchema,
)

router = APIRouter(tags=["search"])


@router.post("/api/v1/search")
async def search_snippets(
    request: SearchRequest,
    search_application_service: CodeSearchAppServiceDep,
    repository_query_service: RepositoryQueryServiceDep,
) -> SearchResponse:
    """Search code snippets with filters matching MCP tool."""
    # Validate source_repo if provided
    if request.sources:
        source_repo = request.sources[0]
        repo_id = await repository_query_service.find_repo_by_url(source_repo)
        if not repo_id:
            raise HTTPException(
                status_code=404, detail=f"Repository not found: {source_repo}"
            )

    # Convert API request to domain request
    domain_request = MultiSearchRequest(
        keywords=request.data.attributes.keywords,
        code_query=request.data.attributes.code,
        text_query=request.data.attributes.text,
        top_k=request.limit or 10,
        filters=SnippetSearchFilters(
            language=request.languages[0] if request.languages else None,
            author=request.authors[0] if request.authors else None,
            created_after=request.start_date,
            created_before=request.end_date,
            source_repo=request.sources[0] if request.sources else None,
            file_path=request.file_patterns[0] if request.file_patterns else None,
        )
        if any(
            [
                request.languages,
                request.authors,
                request.start_date,
                request.end_date,
                request.sources,
                request.file_patterns,
            ]
        )
        else None,
    )

    # Execute search using application service
    results = await search_application_service.search(domain_request)

    return SearchResponse(
        data=[
            SnippetData(
                type="snippet",
                id=result.snippet.id,
                attributes=SnippetAttributes(
                    created_at=result.snippet.created_at,
                    updated_at=result.snippet.updated_at,
                    derives_from=[
                        GitFileSchema(
                            blob_sha=file.blob_sha,
                            path=file.path,
                            mime_type=file.mime_type,
                            size=file.size,
                        )
                        for file in result.snippet.derives_from
                    ],
                    content=SnippetContentSchema(
                        value=result.snippet.content,
                        language=result.snippet.extension,
                    ),
                    enrichments=[
                        EnrichmentSchema(
                            type=enrichment.type,
                            content=enrichment.content,
                        )
                        for enrichment in result.snippet.enrichments
                    ],
                    original_scores=result.original_scores,
                ),
            )
            for result in results
        ]
    )
