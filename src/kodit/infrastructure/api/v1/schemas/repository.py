"""Repository JSON-API schemas."""

from datetime import datetime
from pathlib import Path

from pydantic import AnyUrl, BaseModel

from kodit.domain.entities.git import GitRepo


class RepositoryAttributes(BaseModel):
    """Repository attributes following JSON-API spec."""

    remote_uri: AnyUrl
    created_at: datetime | None = None
    updated_at: datetime | None = None
    last_scanned_at: datetime | None = None
    cloned_path: Path | None = None
    tracking_branch: str | None = None
    num_commits: int = 0
    num_branches: int = 0
    num_tags: int = 0

    @staticmethod
    def from_git_repo(repo: GitRepo) -> "RepositoryAttributes":
        """Create a repository attributes from a Git repository."""
        return RepositoryAttributes(
            remote_uri=repo.sanitized_remote_uri,
            cloned_path=repo.cloned_path,
            created_at=repo.created_at,
            updated_at=repo.updated_at,
            last_scanned_at=repo.last_scanned_at,
            tracking_branch=repo.tracking_config.name if repo.tracking_config else None,
            num_commits=repo.num_commits,
            num_branches=repo.num_branches,
            num_tags=repo.num_tags,
        )


class RepositoryData(BaseModel):
    """Repository data following JSON-API spec."""

    type: str = "repository"
    id: str
    attributes: RepositoryAttributes

    @staticmethod
    def from_git_repo(repo: GitRepo) -> "RepositoryData":
        """Create a repository data from a Git repository."""
        return RepositoryData(
            id=str(repo.id) or "",
            attributes=RepositoryAttributes.from_git_repo(repo),
        )


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
