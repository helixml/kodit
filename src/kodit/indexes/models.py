from sqlalchemy import ForeignKey, LargeBinary
from sqlalchemy.orm import Mapped, mapped_column

from kodit.database import Base, CommonMixin


class Index(Base, CommonMixin):
    """Index model."""

    __tablename__ = "indexes"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))


class Snippet(Base, CommonMixin):
    """Snippet model."""

    __tablename__ = "snippets"

    index_id: Mapped[int] = mapped_column(ForeignKey("indexes.id"))
    content: Mapped[bytes] = mapped_column(LargeBinary)
