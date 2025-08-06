"""SQLAlchemy entities."""

from datetime import UTC, datetime
from enum import Enum

from git import Actor
from sqlalchemy import (
    DateTime,
    ForeignKey,
    Integer,
    String,
    UnicodeText,
    UniqueConstraint,
)
from sqlalchemy import Enum as SQLAlchemyEnum
from sqlalchemy.ext.asyncio import AsyncAttrs
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column
from sqlalchemy.types import JSON


class Base(AsyncAttrs, DeclarativeBase):
    """Base class for all models."""


class CommonMixin:
    """Common mixin for all models."""

    id: Mapped[int] = mapped_column(primary_key=True, autoincrement=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(UTC)
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        default=lambda: datetime.now(UTC),
        onupdate=lambda: datetime.now(UTC),
    )


class SourceType(Enum):
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
        SQLAlchemyEnum(SourceType), default=SourceType.UNKNOWN, index=True
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
    file_processing_status: Mapped[int] = mapped_column(Integer, default=0)

    def __init__(  # noqa: PLR0913
        self,
        created_at: datetime,
        updated_at: datetime,
        source_id: int,
        mime_type: str,
        uri: str,
        cloned_path: str,
        sha256: str,
        size_bytes: int,
        extension: str,
        file_processing_status: int,
    ) -> None:
        """Initialize a new File instance for typing purposes."""
        super().__init__()
        self.created_at = created_at
        self.updated_at = updated_at
        self.source_id = source_id
        self.mime_type = mime_type
        self.uri = uri
        self.cloned_path = cloned_path
        self.sha256 = sha256
        self.size_bytes = size_bytes
        self.extension = extension
        self.file_processing_status = file_processing_status


class EmbeddingType(Enum):
    """Embedding type."""

    CODE = 1
    TEXT = 2


class Embedding(Base, CommonMixin):
    """Embedding model."""

    __tablename__ = "embeddings"

    snippet_id: Mapped[int] = mapped_column(ForeignKey("snippets.id"), index=True)
    type: Mapped[EmbeddingType] = mapped_column(
        SQLAlchemyEnum(EmbeddingType), index=True
    )
    embedding: Mapped[list[float]] = mapped_column(JSON)


class Index(Base, CommonMixin):
    """Index model."""

    __tablename__ = "indexes"

    source_id: Mapped[int] = mapped_column(
        ForeignKey("sources.id"), unique=True, index=True
    )

    def __init__(self, source_id: int) -> None:
        """Initialize the index."""
        super().__init__()
        self.source_id = source_id


class Snippet(Base, CommonMixin):
    """Snippet model."""

    __tablename__ = "snippets"

    file_id: Mapped[int] = mapped_column(ForeignKey("files.id"), index=True)
    index_id: Mapped[int] = mapped_column(ForeignKey("indexes.id"), index=True)
    content: Mapped[str] = mapped_column(UnicodeText, default="")
    summary: Mapped[str] = mapped_column(UnicodeText, default="")

    def __init__(
        self,
        file_id: int,
        index_id: int,
        content: str,
        summary: str = "",
    ) -> None:
        """Initialize the snippet."""
        super().__init__()
        self.file_id = file_id
        self.index_id = index_id
        self.content = content
        self.summary = summary


class TaskType(Enum):
    """Task type."""

    INDEX_UPDATE = 1


class Task(Base, CommonMixin):
    """Queued tasks."""

    __tablename__ = "tasks"

    # dedup_key is used to deduplicate items in the queue
    dedup_key: Mapped[str] = mapped_column(String(255), index=True)
    # type represents what the task is meant to achieve
    type: Mapped[TaskType] = mapped_column(SQLAlchemyEnum(TaskType), index=True)
    # payload contains the task-specific payload data
    payload: Mapped[dict] = mapped_column(JSON)
    # priority is used to determine the order of the items in the queue
    priority: Mapped[int] = mapped_column(Integer)

    def __init__(
        self,
        dedup_key: str,
        type: TaskType,  # noqa: A002
        payload: dict,
        priority: int,
    ) -> None:
        """Initialize the queue item."""
        super().__init__()
        self.dedup_key = dedup_key
        self.type = type
        self.payload = payload
        self.priority = priority
