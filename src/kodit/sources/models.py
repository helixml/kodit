"""Database models for kodit."""

from sqlalchemy import ForeignKey, LargeBinary, String
from sqlalchemy.orm import Mapped, mapped_column

from kodit.database import Base, CommonMixin


class Source(Base, CommonMixin):
    """Source model for tracking code sources."""

    __tablename__ = "sources"


class GitSource(Base, CommonMixin):
    """Git source model."""

    __tablename__ = "git_sources"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))
    uri: Mapped[str] = mapped_column(String(1024))


class FolderSource(Base, CommonMixin):
    """Folder source model."""

    __tablename__ = "folder_sources"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))
    path: Mapped[str] = mapped_column(String(1024))


class File(Base, CommonMixin):
    """File model."""

    __tablename__ = "files"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))
    mime_type: Mapped[str] = mapped_column(String(255))
    path: Mapped[str] = mapped_column(String(1024))
    content: Mapped[bytes] = mapped_column(LargeBinary)
