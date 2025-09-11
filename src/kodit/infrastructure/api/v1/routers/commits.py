"""Commit indexing router for the REST API."""

from datetime import UTC, datetime

from fastapi import APIRouter, Depends, HTTPException

from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import (
    ServerFactoryDep,
)
from kodit.infrastructure.api.v1.schemas.commit import (
    CommitAttributes,
    CommitData,
    CommitGetRequest,
    CommitIndexRequest,
    CommitListRequest,
    CommitListResponse,
    CommitResponse,
    CommitStatsAttributes,
    CommitStatsData,
    CommitStatsRequest,
    CommitStatsResponse,
)

router = APIRouter(
    prefix="/api/v1/commits",
    tags=["commit-indexing"],
    dependencies=[Depends(api_key_auth)],
    responses={
        401: {"description": "Unauthorized"},
        422: {"description": "Invalid request"},
    },
)


@router.post("/index", status_code=201)
async def index_commit(
    request: CommitIndexRequest,
    server_factory: ServerFactoryDep,
) -> CommitResponse:
    """Index a specific commit in a repository."""
    try:
        commit_sha = request.data.attributes.commit_sha
        commit_index = (
            await server_factory.commit_indexing_application_service().index_commit(
                commit_sha
            )
        )

        return CommitResponse(
            data=CommitData(
                type="commit_index",
                id=f"{commit_sha}",
                attributes=CommitAttributes(
                    commit_sha=commit_index.commit_sha,
                    status=commit_index.status,
                    snippet_count=commit_index.get_snippet_count(),
                    indexed_at=commit_index.indexed_at or datetime.now(UTC),
                ),
            )
        )
    except ValueError as e:
        if "not found" in str(e):
            raise HTTPException(status_code=404, detail=str(e)) from e
        raise HTTPException(status_code=400, detail=str(e)) from e
    except Exception as e:
        msg = f"Failed to index commit: {e}"
        raise HTTPException(status_code=500, detail=msg) from e


@router.post("/list")
async def list_indexed_commits(
    request: CommitListRequest,
    server_factory: ServerFactoryDep,
) -> CommitListResponse:
    """List all indexed commits for a repository."""
    try:
        repo_uri = request.data.attributes.repo_uri

        indexed_commits = (
            await server_factory.commit_index_query_service().get_indexed_commits(
                repo_uri
            )
        )

        return CommitListResponse(
            data=[
                CommitData(
                    type="commit_index",
                    id=f"{repo_uri}#{commit.commit_sha}",
                    attributes=CommitAttributes(
                        commit_sha=commit.commit_sha,
                        status=commit.status,
                        snippet_count=commit.get_snippet_count(),
                        indexed_at=commit.indexed_at or datetime.now(UTC),
                    ),
                )
                for commit in indexed_commits
            ]
        )
    except Exception as e:
        msg = f"Failed to list indexed commits: {e}"
        raise HTTPException(status_code=500, detail=msg) from e


@router.post("/stats")
async def get_commit_stats(
    request: CommitStatsRequest,
    server_factory: ServerFactoryDep,
) -> CommitStatsResponse:
    """Get commit indexing statistics for a repository."""
    try:
        repo_uri = request.data.attributes.repo_uri

        stats = (
            await server_factory.commit_index_query_service().get_commit_index_stats(
                repo_uri
            )
        )

        return CommitStatsResponse(
            data=CommitStatsData(
                type="commit_stats",
                id=repo_uri,
                attributes=CommitStatsAttributes(
                    total_indexed_commits=stats["total_indexed_commits"],
                    completed_commits=stats["completed_commits"],
                    failed_commits=stats["failed_commits"],
                    total_snippets=stats["total_snippets"],
                    average_snippets_per_commit=stats["average_snippets_per_commit"],
                ),
            )
        )
    except Exception as e:
        msg = f"Failed to get commit stats: {e}"
        raise HTTPException(status_code=500, detail=msg) from e


@router.post("/get")
async def get_commit_index(
    request: CommitGetRequest,
    server_factory: ServerFactoryDep,
) -> CommitResponse:
    """Get the index status for a specific commit."""
    try:
        commit_sha = request.data.attributes.commit_sha

        commit_index = await server_factory.commit_index_repository().get_by_commit(
            commit_sha
        )

        if not commit_index:
            raise HTTPException(status_code=404, detail="Commit index not found")

        return CommitResponse(
            data=CommitData(
                type="commit_index",
                id=commit_sha,
                attributes=CommitAttributes(
                    commit_sha=commit_index.commit_sha,
                    status=commit_index.status,
                    snippet_count=commit_index.get_snippet_count(),
                    indexed_at=commit_index.indexed_at or datetime.now(UTC),
                ),
            )
        )
    except HTTPException:
        raise
    except Exception as e:
        msg = f"Failed to get commit index: {e}"
        raise HTTPException(status_code=500, detail=msg) from e
