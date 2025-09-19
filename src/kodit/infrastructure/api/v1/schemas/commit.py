"""Commit JSON-API schemas."""

from datetime import datetime

from pydantic import BaseModel


class GitFileData(BaseModel):
    """Git file data."""

    blob_sha: str
    path: str
    mime_type: str
    size: int


class CommitAttributes(BaseModel):
    """Commit attributes following JSON-API spec."""

    commit_sha: str
    date: datetime
    message: str
    parent_commit_sha: str
    author: str


class CommitData(BaseModel):
    """Commit data following JSON-API spec."""

    type: str = "commit"
    id: str
    attributes: CommitAttributes


class CommitResponse(BaseModel):
    """Single commit response following JSON-API spec."""

    data: CommitData


class CommitListResponse(BaseModel):
    """Commit list response following JSON-API spec."""

    data: list[CommitData]


class FileAttributes(BaseModel):
    """File attributes following JSON-API spec."""

    blob_sha: str
    path: str
    mime_type: str
    size: int
    extension: str


class FileData(BaseModel):
    """File data following JSON-API spec."""

    type: str = "file"
    id: str
    attributes: FileAttributes


class FileResponse(BaseModel):
    """Single file response following JSON-API spec."""

    data: FileData


class FileListResponse(BaseModel):
    """File list response following JSON-API spec."""

    data: list[FileData]


class EmbeddingAttributes(BaseModel):
    """Embedding attributes following JSON-API spec."""

    snippet_sha: str
    embedding_type: str
    embedding: list[float]


class EmbeddingData(BaseModel):
    """Embedding data following JSON-API spec."""

    type: str = "embedding"
    id: str
    attributes: EmbeddingAttributes


class EmbeddingListResponse(BaseModel):
    """Embedding list response following JSON-API spec."""

    data: list[EmbeddingData]
