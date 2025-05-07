"""Database models for kodit."""

from sqlalchemy import ForeignKey, String
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
