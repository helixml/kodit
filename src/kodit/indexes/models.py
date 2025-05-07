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

    index_id: Mapped[int] = mapped_column(ForeignKey("indexes.id"))
    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))
    mime_type: Mapped[str] = mapped_column(String(255), default="")
    path: Mapped[str] = mapped_column(String(1024), default="")
    sha256: Mapped[str] = mapped_column(String(64), default="", index=True)
    size_bytes: Mapped[int] = mapped_column(Integer, default=0)
