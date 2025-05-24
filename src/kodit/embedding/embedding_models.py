"""Embedding models."""

from enum import Enum

from sqlalchemy import JSON, ForeignKey
from sqlalchemy import Enum as SQLAlchemyEnum
from sqlalchemy.orm import Mapped, mapped_column

from kodit.database import Base, CommonMixin


class EmbeddingType(Enum):
    """Embedding type."""

    CODE = 1
    TEXT = 2


class Embedding(Base, CommonMixin):
    """Embedding model."""

    __tablename__ = "embeddings"

    snippet_id: Mapped[int] = mapped_column(ForeignKey("snippets.id"), index=True)
    type: Mapped[EmbeddingType] = mapped_column(
        SQLAlchemyEnum(EmbeddingType), index=True
    )
    embedding: Mapped[list[float]] = mapped_column(JSON)
