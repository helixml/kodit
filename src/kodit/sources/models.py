"""Source models for managing code sources.

This module defines the SQLAlchemy models used for storing and managing code sources.
It includes models for tracking different types of sources (git repositories and local
folders) and their relationships.
"""

from sqlalchemy import String
from sqlalchemy.orm import Mapped, mapped_column

from kodit.database import Base, CommonMixin


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
    uri: Mapped[str] = mapped_column(String(1024), index=True)
    cloned_path: Mapped[str] = mapped_column(String(1024))
