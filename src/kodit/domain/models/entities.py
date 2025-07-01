"""Pure domain entities using Pydantic."""

from datetime import datetime
from pathlib import Path

from pydantic import AnyUrl, BaseModel

from kodit.domain.models.value_objects import SnippetContent, SourceType


class Author(BaseModel):
    """Author domain entity."""

    id: int | None = None
    name: str
    email: str

    class Config:
        """Pydantic model configuration."""

        frozen = True


class File(BaseModel):
    """File domain entity."""

    id: int
    created_at: datetime
    updated_at: datetime
    uri: AnyUrl
    sha256: str
    authors: list[Author]

    class Config:
        """Pydantic model configuration."""

        frozen = True


class WorkingCopy(BaseModel):
    """Working copy value object representing cloned source location."""

    created_at: datetime
    updated_at: datetime
    remote_uri: AnyUrl
    cloned_path: Path
    source_type: SourceType
    files: list[File]

    class Config:
        """Pydantic model configuration."""

        frozen = True


class Source(BaseModel):
    """Source domain entity."""

    id: int
    created_at: datetime
    updated_at: datetime
    working_copy: WorkingCopy

    class Config:
        """Pydantic model configuration."""

        frozen = True


class Snippet(BaseModel):
    """Snippet domain entity."""

    id: int
    created_at: datetime
    updated_at: datetime
    contents: list[SnippetContent]

    class Config:
        """Pydantic model configuration."""

        frozen = True


class Index(BaseModel):
    """Index domain entity."""

    id: int
    created_at: datetime
    updated_at: datetime
    source: Source

    class Config:
        """Pydantic model configuration."""

        frozen = True
