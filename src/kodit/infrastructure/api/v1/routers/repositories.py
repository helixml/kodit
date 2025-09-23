"""Repository management router for the REST API."""

from fastapi import APIRouter, Depends, HTTPException

from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import (
    CommitIndexingAppServiceDep,
    GitBranchRepositoryDep,
    GitCommitRepositoryDep,
    GitRepositoryDep,
    GitTagRepositoryDep,
    TaskStatusQueryServiceDep,
)
from kodit.infrastructure.api.v1.schemas.repository import (
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
from kodit.infrastructure.api.v1.schemas.task_status import (
    TaskStatusAttributes,
    TaskStatusData,
    TaskStatusListResponse,
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
        data=[RepositoryData.from_git_repo(repo) for repo in repos]
    )


@router.post("", status_code=201, summary="Create repository")
async def create_repository(
    request: RepositoryCreateRequest,
    service: CommitIndexingAppServiceDep,
) -> RepositoryResponse:
    """Clone a new repository and perform initial mapping."""
    try:
        remote_uri = request.data.attributes.remote_uri

        repo = await service.create_git_repository(remote_uri)

        return RepositoryResponse(data=RepositoryData.from_git_repo(repo))
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
    git_commit_repository: GitCommitRepositoryDep,
    git_branch_repository: GitBranchRepositoryDep,
) -> RepositoryDetailsResponse:
    """Get repository details including branches and recent commits."""
    repo = await git_repository.get_by_id(int(repo_id))
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Get all commits for this repository from the commit repository
    repo_commits = await git_commit_repository.get_by_repo_id(int(repo_id))
    commits_by_sha = {commit.commit_sha: commit for commit in repo_commits}

    # Get recent commits from the tracking branch's head commit
    recent_commits = []
    if repo.tracking_branch and repo.tracking_branch.head_commit:
        # For simplicity, just show the head commit and traverse back if needed
        current_commit = repo.tracking_branch.head_commit
        recent_commits = [current_commit]

        # Traverse parent commits for more recent commits (up to 10)
        current_sha = current_commit.parent_commit_sha
        while current_sha and len(recent_commits) < 10:
            parent_commit = commits_by_sha.get(current_sha)
            if parent_commit:
                recent_commits.append(parent_commit)
                current_sha = parent_commit.parent_commit_sha
            else:
                break

    # Get commit count for the repository using the commit repository
    commit_count = await git_commit_repository.count_by_repo_id(int(repo_id))

    # Get branches for the repository using the branch repository
    repo_branches = await git_branch_repository.get_by_repo_id(int(repo_id))

    # Get commit counts for all branches using the commit repository
    branch_data = []
    for branch in repo_branches:
        # For simplicity, use the total commit count for all branches
        # In a more advanced implementation, we would traverse each branch's history
        branch_commit_count = commit_count

        branch_data.append(
            RepositoryBranchData(
                name=branch.name,
                is_default=branch.name == repo.tracking_branch.name
                if repo.tracking_branch
                else False,
                commit_count=branch_commit_count,
            )
        )

    return RepositoryDetailsResponse(
        data=RepositoryData.from_git_repo(repo),
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


@router.get(
    "/{repo_id}/status",
    responses={404: {"description": "Index not found"}},
)
async def get_index_status(
    repo_id: int,
    status_service: TaskStatusQueryServiceDep,
) -> TaskStatusListResponse:
    """Get the status of tasks for an index."""
    # Get all task statuses for this index
    progress_trackers = await status_service.get_index_status(repo_id)

    # Convert progress trackers to API response format
    task_statuses = []
    for _i, status in enumerate(progress_trackers):
        task_statuses.append(
            TaskStatusData(
                id=status.id,
                attributes=TaskStatusAttributes(
                    step=status.operation,
                    state=status.state,
                    progress=status.completion_percent,
                    total=status.total,
                    current=status.current,
                    created_at=status.created_at,
                    updated_at=status.updated_at,
                    error=status.error or "",
                    message=status.message,
                ),
            )
        )

    return TaskStatusListResponse(data=task_statuses)


@router.get(
    "/{repo_id}/tags",
    summary="List repository tags",
    responses={404: {"description": "Repository not found"}},
)
async def list_repository_tags(
    repo_id: str,
    git_repository: GitRepositoryDep,
    git_tag_repository: GitTagRepositoryDep,
) -> TagListResponse:
    """List all tags for a repository."""
    repo = await git_repository.get_by_id(int(repo_id))
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Tags are now stored in a dedicated repository
    tags = await git_tag_repository.get_by_repo_id(int(repo_id))

    return TagListResponse(
        data=[
            TagData(
                type="tag",
                id=tag.id,
                attributes=TagAttributes(
                    name=tag.name,
                    target_commit_sha=tag.target_commit.commit_sha,
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
    git_tag_repository: GitTagRepositoryDep,
) -> TagResponse:
    """Get a specific tag for a repository."""
    repo = await git_repository.get_by_id(int(repo_id))
    if not repo:
        raise HTTPException(status_code=404, detail="Repository not found")

    # Get all tags and find the specific one by ID
    tags = await git_tag_repository.get_by_repo_id(int(repo_id))
    tag = next((t for t in tags if t.id == tag_id), None)
    if not tag:
        raise HTTPException(status_code=404, detail="Tag not found")

    return TagResponse(
        data=TagData(
            type="tag",
            id=tag.id,
            attributes=TagAttributes(
                name=tag.name,
                target_commit_sha=tag.target_commit.commit_sha,
                is_version_tag=tag.is_version_tag,
            ),
        )
    )


@router.delete(
    "/{repo_id}",
    status_code=204,
    summary="Delete repository",
    responses={404: {"description": "Repository not found"}},
)
async def delete_repository(
    repo_id: str,
    service: CommitIndexingAppServiceDep,
) -> None:
    """Delete a repository and all its associated data."""
    try:
        repo_id_int = int(repo_id)
        deleted = await service.delete_git_repository(repo_id_int)
        if not deleted:
            _raise_not_found_error("Repository not found")
    except ValueError:
        raise HTTPException(status_code=400, detail="Invalid repository ID") from None
    except Exception as e:
        msg = f"Failed to delete repository: {e}"
        raise HTTPException(status_code=500, detail=msg) from e
