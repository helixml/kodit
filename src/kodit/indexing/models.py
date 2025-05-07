"""Index models for managing code indexes.

This module defines the SQLAlchemy models used for storing and managing code indexes,
including files and snippets. It provides the data structures for tracking indexed
files and their content.
"""

from sqlalchemy import ForeignKey, Integer, String, UnicodeText
from sqlalchemy.orm import Mapped, mapped_column

from kodit.database import Base, CommonMixin


class Index(Base, CommonMixin):
    """Index model."""

    __tablename__ = "indexes"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))


class Snippet(Base, CommonMixin):
    """Snippet model."""

    __tablename__ = "snippets"

    file_id: Mapped[int] = mapped_column(ForeignKey("files.id"))
    index_id: Mapped[int] = mapped_column(ForeignKey("indexes.id"))
    content: Mapped[str] = mapped_column(UnicodeText, default="")


class File(Base, CommonMixin):
    """File model."""

    __tablename__ = "files"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))
    mime_type: Mapped[str] = mapped_column(String(255), default="")
    uri: Mapped[str] = mapped_column(String(1024), default="")
    cloned_path: Mapped[str] = mapped_column(String(1024))
    sha256: Mapped[str] = mapped_column(String(64), default="", index=True)
    size_bytes: Mapped[int] = mapped_column(Integer, default=0)
