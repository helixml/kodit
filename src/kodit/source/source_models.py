"""Source models for managing code sources.

This module defines the SQLAlchemy models used for storing and managing code sources.
It includes models for tracking different types of sources (git repositories and local
folders) and their relationships.
"""

import datetime
from enum import Enum as EnumType

from git import Actor
from sqlalchemy import Enum, ForeignKey, Integer, String, UniqueConstraint
from sqlalchemy.orm import Mapped, mapped_column

from kodit.database import Base, CommonMixin

# Enable proper type hints for SQLAlchemy models
__all__ = ["File", "Source"]


class SourceType(EnumType):
    """The type of source."""

    UNKNOWN = 0
    FOLDER = 1
    GIT = 2


class Source(Base, CommonMixin):
    """Base model for tracking code sources.

    This model serves as the parent table for different types of sources.
    It provides common fields and relationships for all source types.

    Attributes:
        id: The unique identifier for the source.
        created_at: Timestamp when the source was created.
        updated_at: Timestamp when the source was last updated.
        cloned_uri: A URI to a copy of the source on the local filesystem.
        uri: The URI of the source.

    """

    __tablename__ = "sources"
    uri: Mapped[str] = mapped_column(String(1024), index=True, unique=True)
    cloned_path: Mapped[str] = mapped_column(String(1024), index=True)
    type: Mapped[SourceType] = mapped_column(
        Enum(SourceType), default=SourceType.UNKNOWN, index=True
    )

    def __init__(self, uri: str, cloned_path: str, source_type: SourceType) -> None:
        """Initialize a new Source instance for typing purposes."""
        super().__init__()
        self.uri = uri
        self.cloned_path = cloned_path
        self.type = source_type


class Author(Base, CommonMixin):
    """Author model."""

    __tablename__ = "authors"

    __table_args__ = (UniqueConstraint("name", "email", name="uix_author"),)

    name: Mapped[str] = mapped_column(String(255), index=True)
    email: Mapped[str] = mapped_column(String(255), index=True)

    @staticmethod
    def from_actor(actor: Actor) -> "Author":
        """Create an Author from an Actor."""
        return Author(name=actor.name, email=actor.email)


class AuthorFileMapping(Base, CommonMixin):
    """Author file mapping model."""

    __tablename__ = "author_file_mappings"

    __table_args__ = (
        UniqueConstraint("author_id", "file_id", name="uix_author_file_mapping"),
    )

    author_id: Mapped[int] = mapped_column(ForeignKey("authors.id"), index=True)
    file_id: Mapped[int] = mapped_column(ForeignKey("files.id"), index=True)


class File(Base, CommonMixin):
    """File model."""

    __tablename__ = "files"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))
    mime_type: Mapped[str] = mapped_column(String(255), default="", index=True)
    uri: Mapped[str] = mapped_column(String(1024), default="", index=True)
    cloned_path: Mapped[str] = mapped_column(String(1024), index=True)
    sha256: Mapped[str] = mapped_column(String(64), default="", index=True)
    size_bytes: Mapped[int] = mapped_column(Integer, default=0)
    extension: Mapped[str] = mapped_column(String(255), default="", index=True)

    def __init__(  # noqa: PLR0913
        self,
        created_at: datetime.datetime,
        updated_at: datetime.datetime,
        source_id: int,
        cloned_path: str,
        mime_type: str = "",
        uri: str = "",
        sha256: str = "",
        size_bytes: int = 0,
    ) -> None:
        """Initialize a new File instance for typing purposes."""
        super().__init__()
        self.created_at = created_at
        self.updated_at = updated_at
        self.source_id = source_id
        self.cloned_path = cloned_path
        self.mime_type = mime_type
        self.uri = uri
        self.sha256 = sha256
        self.size_bytes = size_bytes
