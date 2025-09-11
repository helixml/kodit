"""Repository management router for the REST API."""

from datetime import UTC, datetime

from fastapi import APIRouter, Depends, HTTPException
from pydantic import AnyUrl

from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import GitAppServiceDep
from kodit.infrastructure.api.v1.schemas.repository import (
    RepositoryAttributes,
    RepositoryBranchData,
    RepositoryCommitData,
    RepositoryCreateRequest,
    RepositoryData,
    RepositoryDetailsResponse,
    RepositoryListResponse,
    RepositoryResponse,
    RepositoryUpdateRequest,
)
from kodit.infrastructure.api.v1.schemas.tag import (
    TagAttributes,
    TagData,
    TagListResponse,
    TagResponse,
)

router = APIRouter(
    prefix="/api/v1/repositories",
    tags=["repositories"],
    dependencies=[Depends(api_key_auth)],
    responses={
        401: {"description": "Unauthorized"},
        422: {"description": "Invalid request"},
    },
)


@router.get("", summary="List repositories")
async def list_repositories(
    git_service: GitAppServiceDep,
) -> RepositoryListResponse:
    """List all cloned repositories."""
    repos = await git_service.repo_repository.get_all()
    return RepositoryListResponse(
        data=[
            RepositoryData(
                type="repository",
                id=repo.id,
                attributes=RepositoryAttributes(
                    remote_uri=repo.remote_uri,
                    sanitized_remote_uri=repo.sanitized_remote_uri,
                    cloned_path=repo.cloned_path,
                    created_at=repo.last_scanned_at or datetime.now(UTC),
                    updated_at=repo.last_scanned_at,
                    default_branch=repo.tracking_branch.name,
                    total_commits=repo.total_unique_commits,
                    total_branches=len(repo.branches),
                ),
            )
            for repo in repos
        ]
    )


@router.post("", status_code=201, summary="Create repository")
async def create_repository(
    request: RepositoryCreateRequest,
    git_service: GitAppServiceDep,
) -> RepositoryResponse:
    """Clone a new repository and perform initial mapping."""
    try:
        remote_uri = request.data.attributes.remote_uri

        repo = await git_service.clone_and_map_repository(remote_uri)

        return RepositoryResponse(
            data=RepositoryData(
                type="repository",
                id=repo.id,
                attributes=RepositoryAttributes(
                    remote_uri=repo.remote_uri,
                    sanitized_remote_uri=repo.sanitized_remote_uri,
                    cloned_path=repo.cloned_path,
                    created_at=repo.last_scanned_at or datetime.now(UTC),
                    updated_at=repo.last_scanned_at,
                    default_branch=repo.tracking_branch.name,
                    total_commits=repo.total_unique_commits,
                    total_branches=len(repo.branches),
                ),
            )
        )
    except ValueError as e:
        if "already exists" in str(e):
            raise HTTPException(status_code=409, detail=str(e)) from e
        raise HTTPException(status_code=400, detail=str(e)) from e
    except Exception as e:
        msg = f"Failed to clone repository: {e}"
        raise HTTPException(status_code=500, detail=msg) from e


@router.get(
    "/{repo_id}",
    summary="Get repository",
    responses={404: {"description": "Repository not found"}},
)
async def get_repository(
    repo_id: str,
    git_service: GitAppServiceDep,
) -> RepositoryDetailsResponse:
    """Get repository details including branches and recent commits."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Get commits for the tracking branch
    commits = await git_service.commit_repository.get_commits_for_branch(
        repo.sanitized_remote_uri, repo.tracking_branch.name
    )
    recent_commits = commits[:10] if commits else []

    # Get commit counts for all branches
    branch_data = []
    for branch in repo.branches:
        branch_commits = await git_service.commit_repository.get_commits_for_branch(
            repo.sanitized_remote_uri, branch.name
        )
        branch_data.append(
            RepositoryBranchData(
                name=branch.name,
                is_default=branch.name == repo.tracking_branch.name,
                commit_count=len(branch_commits) if branch_commits else 0,
            )
        )

    return RepositoryDetailsResponse(
        data=RepositoryData(
            type="repository",
            id=repo.id,
            attributes=RepositoryAttributes(
                remote_uri=repo.remote_uri,
                sanitized_remote_uri=repo.sanitized_remote_uri,
                cloned_path=repo.cloned_path,
                created_at=repo.last_scanned_at or datetime.now(UTC),
                updated_at=repo.last_scanned_at,
                default_branch=repo.tracking_branch.name,
                total_commits=repo.total_unique_commits,
                total_branches=len(repo.branches),
            ),
        ),
        branches=branch_data,
        recent_commits=[
            RepositoryCommitData(
                sha=commit.commit_sha,
                message=commit.message,
                author=commit.author,
                timestamp=commit.date,
            )
            for commit in recent_commits
        ],
    )


@router.put("/{repo_id}", status_code=200, summary="Update repository")
async def update_repository(
    repo_id: str,
    request: RepositoryUpdateRequest,
    git_service: GitAppServiceDep,
) -> RepositoryResponse:
    """Update an existing repository with latest changes."""
    try:
        existing_repo = await git_service.repo_repository.get_by_id(repo_id)
        if not existing_repo:
            msg = "Repository not found"
            raise HTTPException(status_code=404, detail=msg)

        if request.data.attributes.pull_latest:
            repo = await git_service.update_repository(
                existing_repo.sanitized_remote_uri
            )
        else:
            repo = await git_service.rescan_existing_repository(existing_repo)

        return RepositoryResponse(
            data=RepositoryData(
                type="repository",
                id=repo.id,
                attributes=RepositoryAttributes(
                    remote_uri=repo.remote_uri,
                    sanitized_remote_uri=repo.sanitized_remote_uri,
                    cloned_path=repo.cloned_path,
                    created_at=repo.last_scanned_at or datetime.now(UTC),
                    updated_at=repo.last_scanned_at,
                    default_branch=repo.tracking_branch.name,
                    total_commits=repo.total_unique_commits,
                    total_branches=len(repo.branches),
                ),
            )
        )
    except ValueError as e:
        if "not found" in str(e):
            raise HTTPException(status_code=404, detail=str(e)) from e
        raise HTTPException(status_code=400, detail=str(e)) from e
    except Exception as e:
        msg = f"Failed to update repository: {e}"
        raise HTTPException(status_code=500, detail=msg) from e


@router.delete(
    "/{repo_id}",
    status_code=204,
    summary="Delete repository",
    responses={404: {"description": "Repository not found"}},
)
async def delete_repository(
    repo_id: str,
    git_service: GitAppServiceDep,
) -> None:
    """Delete a repository."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    try:
        await git_service.repo_repository.delete(AnyUrl(repo_id))

        if repo.cloned_path.exists():
            import shutil

            shutil.rmtree(repo.cloned_path, ignore_errors=True)
    except Exception as e:
        msg = f"Failed to delete repository: {e}"
        raise HTTPException(status_code=500, detail=msg) from e


@router.post("/{repo_id}/rescan", status_code=200, summary="Rescan repository")
async def rescan_repository(
    repo_id: str,
    git_service: GitAppServiceDep,
) -> RepositoryResponse:
    """Rescan a repository without pulling changes."""
    try:
        repo = await git_service.repo_repository.get_by_id(repo_id)
        if not repo:
            msg = "Repository not found"
            raise HTTPException(status_code=404, detail=msg)

        updated_repo = await git_service.rescan_existing_repository(repo)

        return RepositoryResponse(
            data=RepositoryData(
                type="repository",
                id=updated_repo.id,
                attributes=RepositoryAttributes(
                    remote_uri=updated_repo.remote_uri,
                    sanitized_remote_uri=updated_repo.sanitized_remote_uri,
                    cloned_path=updated_repo.cloned_path,
                    created_at=updated_repo.last_scanned_at or datetime.now(UTC),
                    updated_at=updated_repo.last_scanned_at,
                    default_branch=updated_repo.tracking_branch.name,
                    total_commits=updated_repo.total_unique_commits,
                    total_branches=len(updated_repo.branches),
                ),
            )
        )
    except Exception as e:
        msg = f"Failed to rescan repository: {e}"
        raise HTTPException(status_code=500, detail=msg) from e


@router.get(
    "/{repo_id}/tags",
    summary="List repository tags",
    responses={404: {"description": "Repository not found"}},
)
async def list_repository_tags(
    repo_id: str,
    git_service: GitAppServiceDep,
) -> TagListResponse:
    """List all tags for a repository."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    tags = await git_service.tag_repository.get_tags_for_repo(repo.sanitized_remote_uri)

    return TagListResponse(
        data=[
            TagData(
                type="tag",
                id=tag.id,
                attributes=TagAttributes(
                    name=tag.name,
                    target_commit_sha=tag.target_commit_sha,
                    is_version_tag=tag.is_version_tag,
                ),
            )
            for tag in tags
        ]
    )


@router.get(
    "/{repo_id}/tags/{tag_id}",
    summary="Get repository tag",
    responses={404: {"description": "Repository or tag not found"}},
)
async def get_repository_tag(
    repo_id: str,
    tag_id: str,
    git_service: GitAppServiceDep,
) -> TagResponse:
    """Get a specific tag for a repository."""
    repo = await git_service.repo_repository.get_by_id(repo_id)
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    try:
        tag = await git_service.tag_repository.get_tag_by_id(tag_id)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e)) from e

    return TagResponse(
        data=TagData(
            type="tag",
            id=tag.id,
            attributes=TagAttributes(
                name=tag.name,
                target_commit_sha=tag.target_commit_sha,
                is_version_tag=tag.is_version_tag,
            ),
        )
    )
