"""Snippets models."""

from sqlalchemy import ForeignKey, UnicodeText
from sqlalchemy.orm import Mapped, mapped_column

from kodit.database import Base, CommonMixin


class Snippet(Base, CommonMixin):
    """Snippet model."""

    __tablename__ = "snippets"

    file_id: Mapped[int] = mapped_column(ForeignKey("files.id"), index=True)
    index_id: Mapped[int] = mapped_column(ForeignKey("indexes.id"), index=True)
    content: Mapped[str] = mapped_column(UnicodeText, default="")

    def __init__(self, file_id: int, index_id: int, content: str) -> None:
        """Initialize the snippet."""
        super().__init__()
        self.file_id = file_id
        self.index_id = index_id
        self.content = content
