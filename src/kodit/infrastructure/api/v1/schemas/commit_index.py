"""Commit Index JSON-API schemas."""

from datetime import datetime

from pydantic import BaseModel

from kodit.domain.entities import IndexStatus


class CommitIndexAttributes(BaseModel):
    """Commit index attributes following JSON-API spec."""

    commit_sha: str
    status: IndexStatus
    indexed_at: datetime | None = None
    error_message: str | None = None
    files_processed: int = 0
    processing_time_seconds: float = 0.0
    snippet_count: int = 0


class CommitIndexData(BaseModel):
    """Commit index data following JSON-API spec."""

    type: str = "commit_index"
    id: str
    attributes: CommitIndexAttributes


class CommitIndexResponse(BaseModel):
    """Single commit index response following JSON-API spec."""

    data: CommitIndexData


class CommitIndexListResponse(BaseModel):
    """Commit index list response following JSON-API spec."""

    data: list[CommitIndexData]


class CommitIndexCreateAttributes(BaseModel):
    """Commit index creation attributes."""

    commit_sha: str


class CommitIndexCreateData(BaseModel):
    """Commit index creation data."""

    type: str = "commit_index"
    attributes: CommitIndexCreateAttributes


class CommitIndexCreateRequest(BaseModel):
    """Commit index creation request."""

    data: CommitIndexCreateData
