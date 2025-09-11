"""Request and response schemas for repository management."""

from datetime import datetime
from pathlib import Path

from pydantic import AnyUrl, BaseModel, Field


class RepositoryAttributes(BaseModel):
    """Repository attributes in API responses."""

    remote_uri: AnyUrl
    sanitized_remote_uri: AnyUrl
    cloned_path: Path
    created_at: datetime
    updated_at: datetime | None = None
    default_branch: str | None = None
    total_commits: int = 0
    total_branches: int = 0


class RepositoryCreateAttributes(BaseModel):
    """Attributes for creating a repository."""

    remote_uri: AnyUrl = Field(..., description="The Git repository URL to clone")


class RepositoryUpdateAttributes(BaseModel):
    """Attributes for updating a repository."""

    pull_latest: bool = Field(
        default=True, description="Whether to pull latest changes before rescanning"
    )


class RepositoryData(BaseModel):
    """Repository data in API responses."""

    type: str = "repository"
    id: str
    attributes: RepositoryAttributes


class RepositoryCreateData(BaseModel):
    """Repository data for create requests."""

    type: str = "repository"
    attributes: RepositoryCreateAttributes


class RepositoryUpdateData(BaseModel):
    """Repository data for update requests."""

    type: str = "repository"
    attributes: RepositoryUpdateAttributes


class RepositoryResponse(BaseModel):
    """Single repository response."""

    data: RepositoryData


class RepositoryListResponse(BaseModel):
    """Multiple repositories response."""

    data: list[RepositoryData]


class RepositoryCreateRequest(BaseModel):
    """Request to create a new repository."""

    data: RepositoryCreateData


class RepositoryUpdateRequest(BaseModel):
    """Request to update a repository."""

    data: RepositoryUpdateData


class RepositoryBranchData(BaseModel):
    """Branch information for a repository."""

    name: str
    is_default: bool
    commit_count: int


class RepositoryCommitData(BaseModel):
    """Commit information for a repository."""

    sha: str
    message: str
    author: str
    timestamp: datetime


class RepositoryDetailsResponse(BaseModel):
    """Detailed repository response with branches and recent commits."""

    data: RepositoryData
    branches: list[RepositoryBranchData]
    recent_commits: list[RepositoryCommitData]
