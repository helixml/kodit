"""Repository JSON-API schemas."""

from datetime import datetime
from pathlib import Path

from pydantic import AnyUrl, BaseModel


class RepositoryAttributes(BaseModel):
    """Repository attributes following JSON-API spec."""

    remote_uri: AnyUrl
    sanitized_remote_uri: AnyUrl
    cloned_path: Path
    created_at: datetime
    updated_at: datetime | None = None
    default_branch: str
    total_commits: int = 0
    total_branches: int = 0


class RepositoryData(BaseModel):
    """Repository data following JSON-API spec."""

    type: str = "repository"
    id: str
    attributes: RepositoryAttributes


class RepositoryResponse(BaseModel):
    """Single repository response following JSON-API spec."""

    data: RepositoryData


class RepositoryListResponse(BaseModel):
    """Repository list response following JSON-API spec."""

    data: list[RepositoryData]


class RepositoryCreateAttributes(BaseModel):
    """Repository creation attributes."""

    remote_uri: AnyUrl


class RepositoryCreateData(BaseModel):
    """Repository creation data."""

    type: str = "repository"
    attributes: RepositoryCreateAttributes


class RepositoryCreateRequest(BaseModel):
    """Repository creation request."""

    data: RepositoryCreateData


class RepositoryUpdateAttributes(BaseModel):
    """Repository update attributes."""

    pull_latest: bool = False


class RepositoryUpdateData(BaseModel):
    """Repository update data."""

    type: str = "repository"
    attributes: RepositoryUpdateAttributes


class RepositoryUpdateRequest(BaseModel):
    """Repository update request."""

    data: RepositoryUpdateData


class RepositoryBranchData(BaseModel):
    """Repository branch data."""

    name: str
    is_default: bool
    commit_count: int


class RepositoryCommitData(BaseModel):
    """Repository commit data for repository details."""

    sha: str
    message: str
    author: str
    timestamp: datetime


class RepositoryDetailsResponse(BaseModel):
    """Repository details response with branches and commits."""

    data: RepositoryData
    branches: list[RepositoryBranchData]
    recent_commits: list[RepositoryCommitData]
