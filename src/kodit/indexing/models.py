"""Index models for managing code indexes.

This module defines the SQLAlchemy models used for storing and managing code indexes,
including files and snippets. It provides the data structures for tracking indexed
files and their content.
"""

from sqlalchemy import ForeignKey, UnicodeText
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


class Snippet(Base, CommonMixin):
    """Snippet model."""

    __tablename__ = "snippets"

    file_id: Mapped[int] = mapped_column(ForeignKey("files.id"))
    index_id: Mapped[int] = mapped_column(ForeignKey("indexes.id"))
    content: Mapped[str] = mapped_column(UnicodeText, default="")

    def __init__(self, file_id: int, index_id: int, content: str) -> None:
        """Initialize the snippet."""
        super().__init__()
        self.file_id = file_id
        self.index_id = index_id
        self.content = content
