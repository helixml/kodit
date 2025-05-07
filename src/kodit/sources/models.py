"""Source models for managing code sources.

This module defines the SQLAlchemy models used for storing and managing code sources.
It includes models for tracking different types of sources (git repositories and local
folders) and their relationships.
"""

from sqlalchemy import ForeignKey, String
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

    """

    __tablename__ = "sources"


class GitSource(Base, CommonMixin):
    """Model for tracking Git repository sources.

    This model stores information about Git repositories that are being tracked.
    It is linked to the Source model through a foreign key relationship.

    Attributes:
        id: The unique identifier for the git source.
        source_id: Foreign key reference to the parent Source model.
        uri: The URI of the Git repository (e.g., https://github.com/user/repo.git).
        created_at: Timestamp when the git source was created.
        updated_at: Timestamp when the git source was last updated.

    """

    __tablename__ = "git_sources"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))
    uri: Mapped[str] = mapped_column(String(1024))


class FolderSource(Base, CommonMixin):
    """Model for tracking local folder sources.

    This model stores information about local directories that are being tracked.
    It is linked to the Source model through a foreign key relationship.

    Attributes:
        id: The unique identifier for the folder source.
        source_id: Foreign key reference to the parent Source model.
        path: The absolute path to the local directory being tracked.
        created_at: Timestamp when the folder source was created.
        updated_at: Timestamp when the folder source was last updated.

    """

    __tablename__ = "folder_sources"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))
    path: Mapped[str] = mapped_column(String(1024))
