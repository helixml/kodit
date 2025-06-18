"""Index models for managing code indexes.

This module defines the SQLAlchemy models used for storing and managing code indexes,
including files and snippets. It provides the data structures for tracking indexed
files and their content.
"""

from sqlalchemy import ForeignKey
from sqlalchemy.orm import Mapped, mapped_column

from kodit.database import Base, CommonMixin


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
