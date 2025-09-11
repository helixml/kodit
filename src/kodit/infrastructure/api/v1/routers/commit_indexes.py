"""Commit index management router for the REST API."""

from fastapi import APIRouter, Depends, HTTPException

from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import (
    CommitIndexingAppServiceDep,
    CommitIndexQueryServiceDep,
    GitAppServiceDep,
)
from kodit.infrastructure.api.v1.schemas.commit_index import (
    CommitIndexAttributes,
    CommitIndexCreateRequest,
    CommitIndexData,
    CommitIndexListResponse,
    CommitIndexResponse,
)
from kodit.infrastructure.api.v1.schemas.snippet import (
    GitFileSchema,
    SnippetAttributes,
    SnippetContentSchema,
    SnippetData,
    SnippetListResponse,
)

router = APIRouter(
    prefix="/api/v1/repositories",
    tags=["commit_indexes"],
    dependencies=[Depends(api_key_auth)],
    responses={
        401: {"description": "Unauthorized"},
        422: {"description": "Invalid request"},
    },
)


@router.post("/{repo_id}/indexes", status_code=201, summary="Create commit index")
async def create_commit_index(
    repo_id: str,
    request: CommitIndexCreateRequest,
    git_service: GitAppServiceDep,
    indexing_service: CommitIndexingAppServiceDep,
) -> CommitIndexResponse:
    """Create a new commit index for a repository."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    commit_sha = request.data.attributes.commit_sha

    # Check if commit exists in the repository
    commit = next((c for c in repo.commits if c.commit_sha == commit_sha), None)
    if not commit:
        raise HTTPException(status_code=404, detail="Commit not found in repository")

    try:
        # Create the index through the service
        commit_index = await indexing_service.index_commit(commit_sha)

        return CommitIndexResponse(
            data=CommitIndexData(
                type="commit_index",
                id=commit_index.commit_sha,
                attributes=CommitIndexAttributes(
                    commit_sha=commit_index.commit_sha,
                    status=commit_index.status,
                    indexed_at=commit_index.indexed_at,
                    error_message=commit_index.error_message,
                    files_processed=commit_index.files_processed,
                    processing_time_seconds=commit_index.processing_time_seconds,
                    snippet_count=commit_index.get_snippet_count(),
                ),
            )
        )
    except Exception as e:
        msg = f"Failed to create commit index: {e}"
        raise HTTPException(status_code=500, detail=msg) from e


@router.get("/{repo_id}/indexes", summary="List repository commit indexes")
async def list_repository_commit_indexes(
    repo_id: str,
    git_service: GitAppServiceDep,
    query_service: CommitIndexQueryServiceDep,
) -> CommitIndexListResponse:
    """List all commit indexes for a repository."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Get all commit indexes for the repository
    indexes = await query_service.get_indexed_commits(str(repo.sanitized_remote_uri))

    return CommitIndexListResponse(
        data=[
            CommitIndexData(
                type="commit_index",
                id=index.commit_sha,
                attributes=CommitIndexAttributes(
                    commit_sha=index.commit_sha,
                    status=index.status,
                    indexed_at=index.indexed_at,
                    error_message=index.error_message,
                    files_processed=index.files_processed,
                    processing_time_seconds=index.processing_time_seconds,
                    snippet_count=index.get_snippet_count(),
                ),
            )
            for index in indexes
        ]
    )


@router.get(
    "/{repo_id}/indexes/{commit_sha}",
    summary="Get repository commit index",
    responses={404: {"description": "Repository, commit or index not found"}},
)
async def get_repository_commit_index(
    repo_id: str,
    commit_sha: str,
    git_service: GitAppServiceDep,
    indexing_service: CommitIndexingAppServiceDep,
) -> CommitIndexResponse:
    """Get a specific commit index for a repository."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Check if commit exists in the repository
    commit = next((c for c in repo.commits if c.commit_sha == commit_sha), None)
    if not commit:
        raise HTTPException(status_code=404, detail="Commit not found in repository")

    # Get the commit index
    index = await indexing_service.commit_index_repository.get_by_commit(commit_sha)
    if not index:
        raise HTTPException(status_code=404, detail="Commit index not found")

    return CommitIndexResponse(
        data=CommitIndexData(
            type="commit_index",
            id=index.commit_sha,
            attributes=CommitIndexAttributes(
                commit_sha=index.commit_sha,
                status=index.status,
                indexed_at=index.indexed_at,
                error_message=index.error_message,
                files_processed=index.files_processed,
                processing_time_seconds=index.processing_time_seconds,
                snippet_count=index.get_snippet_count(),
            ),
        )
    )


@router.delete(
    "/{repo_id}/indexes/{commit_sha}",
    status_code=204,
    summary="Delete repository commit index",
    responses={404: {"description": "Repository, commit or index not found"}},
)
async def delete_repository_commit_index(
    repo_id: str,
    commit_sha: str,
    git_service: GitAppServiceDep,
    indexing_service: CommitIndexingAppServiceDep,
) -> None:
    """Delete a commit index for a repository."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Check if commit exists in the repository
    commit = next((c for c in repo.commits if c.commit_sha == commit_sha), None)
    if not commit:
        raise HTTPException(status_code=404, detail="Commit not found in repository")

    # Check if index exists
    index = await indexing_service.commit_index_repository.get_by_commit(commit_sha)
    if not index:
        raise HTTPException(status_code=404, detail="Commit index not found")

    try:
        await indexing_service.commit_index_repository.delete(commit_sha)
    except Exception as e:
        msg = f"Failed to delete commit index: {e}"
        raise HTTPException(status_code=500, detail=msg) from e


@router.get(
    "/{repo_id}/indexes/{commit_sha}/snippets",
    summary="List snippets for commit index",
    responses={404: {"description": "Repository, commit or index not found"}},
)
async def list_commit_index_snippets(
    repo_id: str,
    commit_sha: str,
    git_service: GitAppServiceDep,
    indexing_service: CommitIndexingAppServiceDep,
) -> SnippetListResponse:
    """List all snippets for a specific commit index."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Check if commit exists in the repository
    commit = next((c for c in repo.commits if c.commit_sha == commit_sha), None)
    if not commit:
        raise HTTPException(status_code=404, detail="Commit not found in repository")

    # Check if index exists
    index = await indexing_service.commit_index_repository.get_by_commit(commit_sha)
    if not index:
        raise HTTPException(status_code=404, detail="Commit index not found")

    # Get snippets for the commit
    snippets = await indexing_service.snippet_repository.get_snippets_for_commit(
        commit_sha
    )

    # Convert to API response format
    snippet_data: list[SnippetData] = []
    for snippet in snippets:
        # Convert git files
        git_files = [
            GitFileSchema(
                blob_sha=gf.blob_sha,
                path=gf.path,
                mime_type=gf.mime_type,
                size=gf.size,
            )
            for gf in snippet.derives_from
        ]

        # Convert content
        original_content = None
        if snippet.original_content:
            original_content = SnippetContentSchema(
                type=str(snippet.original_content.type.name.lower()),
                value=snippet.original_content.value,
                language=snippet.original_content.language,
            )

        summary_content = None
        if snippet.summary_content:
            summary_content = SnippetContentSchema(
                type=str(snippet.summary_content.type.name.lower()),
                value=snippet.summary_content.value,
                language=snippet.summary_content.language,
            )

        snippet_data.append(
            SnippetData(
                type="snippet",
                id=str(snippet.id) if snippet.id else f"snippet_{len(snippet_data)}",
                attributes=SnippetAttributes(
                    created_at=snippet.created_at,
                    updated_at=snippet.updated_at,
                    derives_from=git_files,
                    original_content=original_content,
                    summary_content=summary_content,
                ),
            )
        )

    return SnippetListResponse(data=snippet_data)
