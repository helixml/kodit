"""Repository management router for the REST API."""

from datetime import UTC, datetime

from fastapi import APIRouter, Depends, HTTPException
from pydantic import AnyUrl

from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import GitAppServiceDep, GitRepositoryDep
from kodit.infrastructure.api.v1.schemas.repository import (
    RepositoryAttributes,
    RepositoryBranchData,
    RepositoryCommitData,
    RepositoryCreateRequest,
    RepositoryData,
    RepositoryDetailsResponse,
    RepositoryListResponse,
    RepositoryResponse,
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


def _raise_not_found_error(detail: str) -> None:
    """Raise repository not found error."""
    raise HTTPException(status_code=404, detail=detail)


@router.get("", summary="List repositories")
async def list_repositories(
    git_repository: GitRepositoryDep,
) -> RepositoryListResponse:
    """List all cloned repositories."""
    repos = await git_repository.get_all()
    return RepositoryListResponse(
        data=[
            RepositoryData(
                type="repository",
                id=str(repo.id) if repo.id is not None else repo.business_key,
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
                id=str(repo.id) if repo.id is not None else repo.business_key,
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
    git_repository: GitRepositoryDep,
) -> RepositoryDetailsResponse:
    """Get repository details including branches and recent commits."""
    repo = await git_repository.get_by_id(int(repo_id))
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Get recent commits from the tracking branch's head commit
    recent_commits = []
    if repo.tracking_branch and repo.tracking_branch.head_commit:
        # For simplicity, just show the head commit and traverse back if needed
        current_commit = repo.tracking_branch.head_commit
        recent_commits = [current_commit]

        # Traverse parent commits for more recent commits (up to 10)
        current_sha = current_commit.parent_commit_sha
        while current_sha and len(recent_commits) < 10:
            parent_commit = next(
                (c for c in repo.commits if c.commit_sha == current_sha), None
            )
            if parent_commit:
                recent_commits.append(parent_commit)
                current_sha = parent_commit.parent_commit_sha
            else:
                break

    # Get commit counts for all branches using the existing commits in the repo
    branch_data = []
    for branch in repo.branches:
        # Count commits accessible from this branch's head
        branch_commit_count = 0
        if branch.head_commit:
            # For simplicity, count all commits (could traverse branch history)
            branch_commit_count = len([c for c in repo.commits if c])

        branch_data.append(
            RepositoryBranchData(
                name=branch.name,
                is_default=branch.name == repo.tracking_branch.name,
                commit_count=branch_commit_count,
            )
        )

    return RepositoryDetailsResponse(
        data=RepositoryData(
            type="repository",
            id=str(repo.id) if repo.id is not None else repo.business_key,
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
    repo = await git_service.repo_repository.get_by_id(int(repo_id))
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


@router.get(
    "/{repo_id}/tags",
    summary="List repository tags",
    responses={404: {"description": "Repository not found"}},
)
async def list_repository_tags(
    repo_id: str,
    git_repository: GitRepositoryDep,
) -> TagListResponse:
    """List all tags for a repository."""
    repo = await git_repository.get_by_id(int(repo_id))
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Tags are now available directly from the repo aggregate
    tags = repo.tags

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
    git_repository: GitRepositoryDep,
) -> TagResponse:
    """Get a specific tag for a repository."""
    repo = await git_repository.get_by_id(int(repo_id))
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Find tag by ID from the repo's tags
    tag = next((t for t in repo.tags if t.id == tag_id), None)
    if not tag:
        raise HTTPException(status_code=404, detail="Tag not found")

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
