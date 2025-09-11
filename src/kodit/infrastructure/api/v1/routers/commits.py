"""Commit management router for the REST API."""

from fastapi import APIRouter, Depends, HTTPException

from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import GitAppServiceDep
from kodit.infrastructure.api.v1.schemas.commit import (
    CommitAttributes,
    CommitData,
    CommitListResponse,
    CommitResponse,
    FileAttributes,
    FileData,
    FileListResponse,
    FileResponse,
)

router = APIRouter(
    prefix="/api/v1/repositories",
    tags=["commits"],
    dependencies=[Depends(api_key_auth)],
    responses={
        401: {"description": "Unauthorized"},
        422: {"description": "Invalid request"},
    },
)


@router.get("/{repo_id}/commits", summary="List repository commits")
async def list_repository_commits(
    repo_id: str,
    git_service: GitAppServiceDep,
) -> CommitListResponse:
    """List all commits for a repository."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Get all commits for the repository
    commits = repo.commits

    return CommitListResponse(
        data=[
            CommitData(
                type="commit",
                id=commit.commit_sha,
                attributes=CommitAttributes(
                    commit_sha=commit.commit_sha,
                    date=commit.date,
                    message=commit.message,
                    parent_commit_sha=commit.parent_commit_sha,
                    author=commit.author,
                ),
            )
            for commit in commits
        ]
    )


@router.get(
    "/{repo_id}/commits/{commit_sha}",
    summary="Get repository commit",
    responses={404: {"description": "Repository or commit not found"}},
)
async def get_repository_commit(
    repo_id: str,
    commit_sha: str,
    git_service: GitAppServiceDep,
) -> CommitResponse:
    """Get a specific commit for a repository."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Find the specific commit
    commit = next((c for c in repo.commits if c.commit_sha == commit_sha), None)
    if not commit:
        raise HTTPException(status_code=404, detail="Commit not found")

    return CommitResponse(
        data=CommitData(
            type="commit",
            id=commit.commit_sha,
            attributes=CommitAttributes(
                commit_sha=commit.commit_sha,
                date=commit.date,
                message=commit.message,
                parent_commit_sha=commit.parent_commit_sha,
                author=commit.author,
            ),
        )
    )


@router.get("/{repo_id}/commits/{commit_sha}/files", summary="List commit files")
async def list_commit_files(
    repo_id: str,
    commit_sha: str,
    git_service: GitAppServiceDep,
) -> FileListResponse:
    """List all files in a specific commit."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Find the specific commit
    commit = next((c for c in repo.commits if c.commit_sha == commit_sha), None)
    if not commit:
        raise HTTPException(status_code=404, detail="Commit not found")

    return FileListResponse(
        data=[
            FileData(
                type="file",
                id=file.blob_sha,
                attributes=FileAttributes(
                    blob_sha=file.blob_sha,
                    path=file.path,
                    mime_type=file.mime_type,
                    size=file.size,
                    extension=file.extension(),
                ),
            )
            for file in commit.files
        ]
    )


@router.get(
    "/{repo_id}/commits/{commit_sha}/files/{blob_sha}",
    summary="Get commit file",
    responses={404: {"description": "Repository, commit or file not found"}},
)
async def get_commit_file(
    repo_id: str,
    commit_sha: str,
    blob_sha: str,
    git_service: GitAppServiceDep,
) -> FileResponse:
    """Get a specific file from a commit."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Find the specific commit
    commit = next((c for c in repo.commits if c.commit_sha == commit_sha), None)
    if not commit:
        raise HTTPException(status_code=404, detail="Commit not found")

    # Find the specific file
    file = next((f for f in commit.files if f.blob_sha == blob_sha), None)
    if not file:
        raise HTTPException(status_code=404, detail="File not found")

    return FileResponse(
        data=FileData(
            type="file",
            id=file.blob_sha,
            attributes=FileAttributes(
                blob_sha=file.blob_sha,
                path=file.path,
                mime_type=file.mime_type,
                size=file.size,
                extension=file.extension(),
            ),
        )
    )
