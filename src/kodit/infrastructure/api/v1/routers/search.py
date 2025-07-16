"""Search router for the REST API."""

from fastapi import APIRouter

from kodit.domain.value_objects import MultiSearchRequest, SnippetSearchFilters
from kodit.infrastructure.api.v1.dependencies import (
    IndexingAppServiceDep,
)
from kodit.infrastructure.api.v1.schemas.search import (
    SearchRequest,
    SearchResponse,
    SnippetAttributes,
    SnippetData,
)

router = APIRouter(tags=["search"])


@router.post("/api/v1/search")
async def search_snippets(
    request: SearchRequest,
    app_service: IndexingAppServiceDep,
) -> SearchResponse:
    """Search code snippets with filters matching MCP tool."""
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
    results = await app_service.search(domain_request)

    return SearchResponse(
        data=[
            SnippetData(
                type="snippet",
                id=result.id,
                attributes=SnippetAttributes(
                    content=result.content,
                    created_at=result.created_at,
                    updated_at=result.created_at,  # Use created_at as fallback
                    original_scores=result.original_scores,
                    source_uri=result.source_uri,
                    relative_path=result.relative_path,
                    language=result.language,
                    authors=result.authors,
                    summary=result.summary,
                ),
            )
            for result in results
        ]
    )
