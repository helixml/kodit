"""SQLAlchemy entities."""

from datetime import UTC, datetime
from enum import Enum
from pathlib import Path
from typing import Any

from sqlalchemy import (
    DateTime,
    Float,
    ForeignKey,
    Index,
    Integer,
    String,
    TypeDecorator,
    UnicodeText,
    UniqueConstraint,
)
from sqlalchemy import Enum as SQLAlchemyEnum
from sqlalchemy.ext.asyncio import AsyncAttrs
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column
from sqlalchemy.types import JSON


# See <https://docs.sqlalchemy.org/en/20/core/custom_types.html#store-timezone-aware-timestamps-as-timezone-naive-utc>
# And [this issue](https://github.com/sqlalchemy/sqlalchemy/issues/1985)
class TZDateTime(TypeDecorator):
    """Timezone-aware datetime type."""

    impl = DateTime
    cache_ok = True

    def process_bind_param(self, value: Any, dialect: Any) -> Any:  # noqa: ARG002
        """Process bind param."""
        if value is not None:
            if not value.tzinfo or value.tzinfo.utcoffset(value) is None:
                raise TypeError("tzinfo is required")
            value = value.astimezone(UTC).replace(tzinfo=None)
        return value

    def process_result_value(self, value: Any, dialect: Any) -> Any:  # noqa: ARG002
        """Process result value."""
        if value is not None:
            value = value.replace(tzinfo=UTC)
        return value


class PathType(TypeDecorator):
    """Path type that stores Path objects as strings."""

    impl = String
    cache_ok = True

    def process_bind_param(self, value: Any, dialect: Any) -> Any:  # noqa: ARG002
        """Process bind param - convert Path to string."""
        if value is not None:
            return str(value)
        return value

    def process_result_value(self, value: Any, dialect: Any) -> Any:  # noqa: ARG002
        """Process result value - convert string to Path."""
        if value is not None:
            return Path(value)
        return value


class Base(AsyncAttrs, DeclarativeBase):
    """Base class for all models."""


class CommonMixin:
    """Common mixin for all models."""

    id: Mapped[int] = mapped_column(primary_key=True, autoincrement=True)
    created_at: Mapped[datetime] = mapped_column(
        TZDateTime, nullable=False, default=lambda: datetime.now(UTC)
    )
    updated_at: Mapped[datetime] = mapped_column(
        TZDateTime,
        nullable=False,
        default=lambda: datetime.now(UTC),
        onupdate=lambda: datetime.now(UTC),
    )


class EmbeddingType(Enum):
    """Embedding type."""

    CODE = 1
    TEXT = 2


class Embedding(Base, CommonMixin):
    """Embedding model."""

    __tablename__ = "embeddings"

    snippet_id: Mapped[str] = mapped_column(String(64), index=True)
    type: Mapped[EmbeddingType] = mapped_column(
        SQLAlchemyEnum(EmbeddingType), index=True
    )
    embedding: Mapped[list[float]] = mapped_column(JSON)


class Task(Base, CommonMixin):
    """Queued tasks."""

    __tablename__ = "tasks"

    # dedup_key is used to deduplicate items in the queue
    dedup_key: Mapped[str] = mapped_column(String(255), index=True)
    # type represents what the task is meant to achieve
    type: Mapped[str] = mapped_column(String(255), index=True)
    # payload contains the task-specific payload data
    payload: Mapped[dict] = mapped_column(JSON)
    # priority is used to determine the order of the items in the queue
    priority: Mapped[int] = mapped_column(Integer)

    def __init__(
        self,
        dedup_key: str,
        type: str,  # noqa: A002
        payload: dict,
        priority: int,
    ) -> None:
        """Initialize the queue item."""
        super().__init__()
        self.dedup_key = dedup_key
        self.type = type
        self.payload = payload
        self.priority = priority


class TaskStatus(Base):
    """Task status model."""

    __tablename__ = "task_status"
    id: Mapped[str] = mapped_column(
        String(255), primary_key=True, index=True, nullable=False
    )
    created_at: Mapped[datetime] = mapped_column(
        TZDateTime, nullable=False, default=lambda: datetime.now(UTC)
    )
    updated_at: Mapped[datetime] = mapped_column(
        TZDateTime,
        nullable=False,
        default=lambda: datetime.now(UTC),
        onupdate=lambda: datetime.now(UTC),
    )
    operation: Mapped[str] = mapped_column(String(255), index=True, nullable=False)
    trackable_id: Mapped[int | None] = mapped_column(Integer, index=True, nullable=True)
    trackable_type: Mapped[str | None] = mapped_column(
        String(255), index=True, nullable=True
    )
    parent: Mapped[str | None] = mapped_column(
        ForeignKey("task_status.id"), index=True, nullable=True
    )
    message: Mapped[str] = mapped_column(UnicodeText, default="")
    state: Mapped[str] = mapped_column(String(255), default="")
    error: Mapped[str] = mapped_column(UnicodeText, default="")
    total: Mapped[int] = mapped_column(Integer, default=0)
    current: Mapped[int] = mapped_column(Integer, default=0)

    def __init__(  # noqa: PLR0913
        self,
        id: str,  # noqa: A002
        operation: str,
        created_at: datetime,
        updated_at: datetime,
        trackable_id: int | None,
        trackable_type: str | None,
        parent: str | None,
        state: str,
        error: str | None,
        total: int,
        current: int,
        message: str,
    ) -> None:
        """Initialize the task status."""
        super().__init__()
        self.id = id
        self.operation = operation
        self.created_at = created_at
        self.updated_at = updated_at
        self.trackable_id = trackable_id
        self.trackable_type = trackable_type
        self.parent = parent
        self.state = state
        self.error = error or ""
        self.total = total
        self.current = current
        self.message = message or ""


# Git-related entities for new GitRepo domain


class GitRepo(Base, CommonMixin):
    """Git repository model."""

    __tablename__ = "git_repos"

    sanitized_remote_uri: Mapped[str] = mapped_column(
        String(1024), index=True, unique=True
    )
    remote_uri: Mapped[str] = mapped_column(String(1024))
    cloned_path: Mapped[Path | None] = mapped_column(PathType(1024), nullable=True)
    last_scanned_at: Mapped[datetime | None] = mapped_column(TZDateTime, nullable=True)
    num_commits: Mapped[int] = mapped_column(Integer, default=0)
    num_branches: Mapped[int] = mapped_column(Integer, default=0)
    num_tags: Mapped[int] = mapped_column(Integer, default=0)
    tracking_type: Mapped[str] = mapped_column(String(255), index=True)
    tracking_name: Mapped[str] = mapped_column(String(255), index=True)

    def __init__(  # noqa: PLR0913
        self,
        sanitized_remote_uri: str,
        remote_uri: str,
        tracking_type: str,
        tracking_name: str,
        cloned_path: Path | None,
        last_scanned_at: datetime | None = None,
        num_commits: int = 0,
        num_branches: int = 0,
        num_tags: int = 0,
    ) -> None:
        """Initialize Git repository."""
        super().__init__()
        self.sanitized_remote_uri = sanitized_remote_uri
        self.remote_uri = remote_uri
        self.tracking_type = tracking_type
        self.tracking_name = tracking_name
        self.cloned_path = cloned_path
        self.last_scanned_at = last_scanned_at
        self.num_commits = num_commits
        self.num_branches = num_branches
        self.num_tags = num_tags


class GitCommit(Base):
    """Git commit model."""

    __tablename__ = "git_commits"

    commit_sha: Mapped[str] = mapped_column(String(64), primary_key=True)
    created_at: Mapped[datetime] = mapped_column(
        TZDateTime, nullable=False, default=lambda: datetime.now(UTC)
    )
    updated_at: Mapped[datetime] = mapped_column(
        TZDateTime,
        nullable=False,
        default=lambda: datetime.now(UTC),
        onupdate=lambda: datetime.now(UTC),
    )
    repo_id: Mapped[int] = mapped_column(ForeignKey("git_repos.id"), index=True)
    date: Mapped[datetime] = mapped_column(TZDateTime)
    message: Mapped[str] = mapped_column(UnicodeText)
    parent_commit_sha: Mapped[str | None] = mapped_column(String(64), index=True)
    author: Mapped[str] = mapped_column(String(255), index=True)

    def __init__(  # noqa: PLR0913
        self,
        commit_sha: str,
        repo_id: int,
        date: datetime,
        message: str,
        parent_commit_sha: str | None,
        author: str,
    ) -> None:
        """Initialize Git commit."""
        super().__init__()
        self.commit_sha = commit_sha
        self.repo_id = repo_id
        self.date = date
        self.message = message
        self.parent_commit_sha = parent_commit_sha
        self.author = author


class GitBranch(Base):
    """Git branch model."""

    __tablename__ = "git_branches"
    repo_id: Mapped[int] = mapped_column(
        ForeignKey("git_repos.id"), index=True, primary_key=True
    )
    name: Mapped[str] = mapped_column(String(255), index=True, primary_key=True)
    created_at: Mapped[datetime] = mapped_column(
        TZDateTime, nullable=False, default=lambda: datetime.now(UTC)
    )
    updated_at: Mapped[datetime] = mapped_column(
        TZDateTime,
        nullable=False,
        default=lambda: datetime.now(UTC),
        onupdate=lambda: datetime.now(UTC),
    )
    head_commit_sha: Mapped[str] = mapped_column(String(64), index=True)

    __table_args__ = (UniqueConstraint("repo_id", "name", name="uix_repo_branch"),)

    def __init__(
        self,
        repo_id: int,
        name: str,
        head_commit_sha: str,
    ) -> None:
        """Initialize Git branch."""
        super().__init__()
        self.repo_id = repo_id
        self.name = name
        self.head_commit_sha = head_commit_sha


class GitTag(Base):
    """Git tag model."""

    __tablename__ = "git_tags"
    repo_id: Mapped[int] = mapped_column(
        ForeignKey("git_repos.id"), index=True, primary_key=True
    )
    name: Mapped[str] = mapped_column(String(255), index=True, primary_key=True)
    created_at: Mapped[datetime] = mapped_column(
        TZDateTime, nullable=False, default=lambda: datetime.now(UTC)
    )
    updated_at: Mapped[datetime] = mapped_column(
        TZDateTime,
        nullable=False,
        default=lambda: datetime.now(UTC),
        onupdate=lambda: datetime.now(UTC),
    )
    target_commit_sha: Mapped[str] = mapped_column(String(64), index=True)

    __table_args__ = (UniqueConstraint("repo_id", "name", name="uix_repo_tag"),)

    def __init__(self, repo_id: int, name: str, target_commit_sha: str) -> None:
        """Initialize Git tag."""
        super().__init__()
        self.repo_id = repo_id
        self.name = name
        self.target_commit_sha = target_commit_sha


class GitCommitFile(Base):
    """Files in a git commit (tree entries)."""

    __tablename__ = "git_commit_files"

    commit_sha: Mapped[str] = mapped_column(
        ForeignKey("git_commits.commit_sha"), primary_key=True
    )
    path: Mapped[str] = mapped_column(String(1024), primary_key=True)
    blob_sha: Mapped[str] = mapped_column(String(64), index=True)
    mime_type: Mapped[str] = mapped_column(String(255), index=True)
    extension: Mapped[str] = mapped_column(String(255), index=True)
    size: Mapped[int] = mapped_column(Integer)
    created_at: Mapped[datetime] = mapped_column(TZDateTime, nullable=False)

    __table_args__ = (UniqueConstraint("commit_sha", "path", name="uix_commit_file"),)

    def __init__(  # noqa: PLR0913
        self,
        commit_sha: str,
        path: str,
        blob_sha: str,
        mime_type: str,
        extension: str,
        size: int,
        created_at: datetime,
    ) -> None:
        """Initialize Git commit file."""
        super().__init__()
        self.commit_sha = commit_sha
        self.path = path
        self.blob_sha = blob_sha
        self.mime_type = mime_type
        self.size = size
        self.created_at = created_at
        self.extension = extension


class CommitIndex(Base):
    """Commit index model."""

    __tablename__ = "commit_indexes"

    created_at: Mapped[datetime] = mapped_column(
        TZDateTime, nullable=False, default=lambda: datetime.now(UTC)
    )
    updated_at: Mapped[datetime] = mapped_column(
        TZDateTime,
        nullable=False,
        default=lambda: datetime.now(UTC),
        onupdate=lambda: datetime.now(UTC),
    )
    commit_sha: Mapped[str] = mapped_column(String(64), primary_key=True)
    status: Mapped[str] = mapped_column(String(255), index=True)
    indexed_at: Mapped[datetime | None] = mapped_column(TZDateTime, nullable=True)
    error_message: Mapped[str | None] = mapped_column(UnicodeText, nullable=True)
    files_processed: Mapped[int] = mapped_column(Integer, default=0)
    processing_time_seconds: Mapped[float] = mapped_column(Float, default=0.0)

    def __init__(  # noqa: PLR0913
        self,
        commit_sha: str,
        status: str,
        indexed_at: datetime | None = None,
        error_message: str | None = None,
        files_processed: int = 0,
        processing_time_seconds: float = 0.0,
    ) -> None:
        """Initialize commit index."""
        super().__init__()
        self.commit_sha = commit_sha
        self.status = status
        self.indexed_at = indexed_at
        self.error_message = error_message
        self.files_processed = files_processed
        self.processing_time_seconds = processing_time_seconds


class EnrichmentV2(Base, CommonMixin):
    """Generic enrichment entity."""

    __tablename__ = "enrichments_v2"

    type: Mapped[str] = mapped_column(String, nullable=False, index=True)
    subtype: Mapped[str] = mapped_column(String, nullable=False, index=True)
    content: Mapped[str] = mapped_column(UnicodeText, nullable=False)

    __table_args__ = (Index("idx_type_subtype", "type", "subtype"),)


class EnrichmentAssociation(Base, CommonMixin):
    """Polymorphic association between enrichments and entities."""

    __tablename__ = "enrichment_associations"

    enrichment_id: Mapped[int] = mapped_column(
        ForeignKey("enrichments_v2.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
    )
    entity_type: Mapped[str] = mapped_column(
        String(50),
        nullable=False,
        index=True,
    )
    entity_id: Mapped[str] = mapped_column(
        String(255),
        nullable=False,
        index=True,
    )

    __table_args__ = (
        UniqueConstraint(
            "entity_type",
            "entity_id",
            "enrichment_id",
            name="uix_entity_enrichment",
        ),
        Index("idx_entity_lookup", "entity_type", "entity_id"),
        {"sqlite_autoincrement": True},
    )
