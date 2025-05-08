"""Source models for managing code sources.

This module defines the SQLAlchemy models used for storing and managing code sources.
It includes models for tracking different types of sources (git repositories and local
folders) and their relationships.
"""

from sqlalchemy import ForeignKey, Integer, String
from sqlalchemy.orm import Mapped, mapped_column

from kodit.database import Base, CommonMixin

# Enable proper type hints for SQLAlchemy models
__all__ = ["File", "Source"]


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
    cloned_path: Mapped[str] = mapped_column(String(1024))

    def __init__(self, uri: str, cloned_path: str) -> None:
        """Initialize a new Source instance for typing purposes."""
        super().__init__()
        self.uri = uri
        self.cloned_path = cloned_path


class File(Base, CommonMixin):
    """File model."""

    __tablename__ = "files"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))
    mime_type: Mapped[str] = mapped_column(String(255), default="")
    uri: Mapped[str] = mapped_column(String(1024), default="")
    cloned_path: Mapped[str] = mapped_column(String(1024))
    sha256: Mapped[str] = mapped_column(String(64), default="", index=True)
    size_bytes: Mapped[int] = mapped_column(Integer, default=0)

    def __init__(  # noqa: PLR0913
        self,
        source_id: int,
        cloned_path: str,
        mime_type: str = "",
        uri: str = "",
        sha256: str = "",
        size_bytes: int = 0,
    ) -> None:
        """Initialize a new File instance for typing purposes."""
        super().__init__()
        self.source_id = source_id
        self.cloned_path = cloned_path
        self.mime_type = mime_type
        self.uri = uri
        self.sha256 = sha256
        self.size_bytes = size_bytes
