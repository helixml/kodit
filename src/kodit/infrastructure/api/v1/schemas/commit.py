"""Request and response schemas for commit indexing management."""

from datetime import datetime

from pydantic import BaseModel, Field

from kodit.domain.entities import IndexStatus


class CommitAttributes(BaseModel):
    """Commit attributes in API responses."""

    commit_sha: str
    status: IndexStatus
    snippet_count: int
    indexed_at: datetime


class CommitIndexAttributes(BaseModel):
    """Attributes for indexing a commit."""

    commit_sha: str = Field(..., description="The commit SHA to index")


class CommitData(BaseModel):
    """Commit data in API responses."""

    type: str = "commit_index"
    id: str
    attributes: CommitAttributes


class CommitIndexData(BaseModel):
    """Commit data for index requests."""

    type: str = "commit_index"
    attributes: CommitIndexAttributes


class CommitResponse(BaseModel):
    """Single commit index response."""

    data: CommitData


class CommitListResponse(BaseModel):
    """Multiple commit indexes response."""

    data: list[CommitData]


class CommitIndexRequest(BaseModel):
    """Request to index a commit."""

    data: CommitIndexData


class CommitStatsAttributes(BaseModel):
    """Statistics about commit indexing for a repository."""

    total_indexed_commits: int
    completed_commits: int
    failed_commits: int
    total_snippets: int
    average_snippets_per_commit: float


class CommitStatsData(BaseModel):
    """Commit stats data in API responses."""

    type: str = "commit_stats"
    id: str
    attributes: CommitStatsAttributes


class CommitStatsResponse(BaseModel):
    """Commit indexing statistics response."""

    data: CommitStatsData


class CommitListRequestAttributes(BaseModel):
    """Attributes for listing commits."""

    repo_uri: str = Field(..., description="The repository URI")


class CommitListRequestData(BaseModel):
    """Data for listing commits request."""

    type: str = "commit_list_request"
    attributes: CommitListRequestAttributes


class CommitListRequest(BaseModel):
    """Request to list indexed commits."""

    data: CommitListRequestData


class CommitStatsRequestAttributes(BaseModel):
    """Attributes for getting commit stats."""

    repo_uri: str = Field(..., description="The repository URI")


class CommitStatsRequestData(BaseModel):
    """Data for commit stats request."""

    type: str = "commit_stats_request"
    attributes: CommitStatsRequestAttributes


class CommitStatsRequest(BaseModel):
    """Request to get commit statistics."""

    data: CommitStatsRequestData


class CommitGetRequestAttributes(BaseModel):
    """Attributes for getting a specific commit."""

    commit_sha: str = Field(..., description="The commit SHA")


class CommitGetRequestData(BaseModel):
    """Data for getting a commit request."""

    type: str = "commit_get_request"
    attributes: CommitGetRequestAttributes


class CommitGetRequest(BaseModel):
    """Request to get a specific commit index."""

    data: CommitGetRequestData
