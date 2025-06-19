"""SQLAlchemy models."""

from datetime import UTC, datetime
from enum import Enum
from pathlib import Path

from git import Actor
from sqlalchemy import (
    DateTime,
    ForeignKey,
    Integer,
    String,
    UnicodeText,
    UniqueConstraint,
)
from sqlalchemy import Enum as SQLAlchemyEnum
from sqlalchemy.ext.asyncio import AsyncAttrs
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column
from sqlalchemy.types import JSON


class Base(AsyncAttrs, DeclarativeBase):
    """Base class for all models."""


class CommonMixin:
    """Common mixin for all models."""

    id: Mapped[int] = mapped_column(primary_key=True, autoincrement=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(UTC)
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        default=lambda: datetime.now(UTC),
        onupdate=lambda: datetime.now(UTC),
    )


class SourceType(Enum):
    """The type of source."""

    UNKNOWN = 0
    FOLDER = 1
    GIT = 2


class Source(Base, CommonMixin):
    """Base model for tracking code sources.

    This model serves as the parent table for different types of sources.
    It provides common fields and relationships for all source types.

    Attributes:
        id: The unique identifier for the source.
        created_at: Timestamp when the source was created.
        updated_at: Timestamp when the source was last updated.
        cloned_uri: A URI to a copy of the source on the local filesystem.
        uri: The URI of the source.

    """

    __tablename__ = "sources"
    uri: Mapped[str] = mapped_column(String(1024), index=True, unique=True)
    cloned_path: Mapped[str] = mapped_column(String(1024), index=True)
    type: Mapped[SourceType] = mapped_column(
        SQLAlchemyEnum(SourceType), default=SourceType.UNKNOWN, index=True
    )

    def __init__(self, uri: str, cloned_path: str, source_type: SourceType) -> None:
        """Initialize a new Source instance for typing purposes."""
        super().__init__()
        self.uri = uri
        self.cloned_path = cloned_path
        self.type = source_type


class Author(Base, CommonMixin):
    """Author model."""

    __tablename__ = "authors"

    __table_args__ = (UniqueConstraint("name", "email", name="uix_author"),)

    name: Mapped[str] = mapped_column(String(255), index=True)
    email: Mapped[str] = mapped_column(String(255), index=True)

    @staticmethod
    def from_actor(actor: Actor) -> "Author":
        """Create an Author from an Actor."""
        return Author(name=actor.name, email=actor.email)


class AuthorFileMapping(Base, CommonMixin):
    """Author file mapping model."""

    __tablename__ = "author_file_mappings"

    __table_args__ = (
        UniqueConstraint("author_id", "file_id", name="uix_author_file_mapping"),
    )

    author_id: Mapped[int] = mapped_column(ForeignKey("authors.id"), index=True)
    file_id: Mapped[int] = mapped_column(ForeignKey("files.id"), index=True)


class File(Base, CommonMixin):
    """File model."""

    __tablename__ = "files"

    source_id: Mapped[int] = mapped_column(ForeignKey("sources.id"))
    mime_type: Mapped[str] = mapped_column(String(255), default="", index=True)
    uri: Mapped[str] = mapped_column(String(1024), default="", index=True)
    cloned_path: Mapped[str] = mapped_column(String(1024), index=True)
    sha256: Mapped[str] = mapped_column(String(64), default="", index=True)
    size_bytes: Mapped[int] = mapped_column(Integer, default=0)
    extension: Mapped[str] = mapped_column(String(255), default="", index=True)

    def __init__(  # noqa: PLR0913
        self,
        created_at: datetime,
        updated_at: datetime,
        source_id: int,
        cloned_path: str,
        mime_type: str = "",
        uri: str = "",
        sha256: str = "",
        size_bytes: int = 0,
    ) -> None:
        """Initialize a new File instance for typing purposes."""
        super().__init__()
        self.created_at = created_at
        self.updated_at = updated_at
        self.source_id = source_id
        self.cloned_path = cloned_path
        self.mime_type = mime_type
        self.uri = uri
        self.sha256 = sha256
        self.size_bytes = size_bytes


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

    file_id: Mapped[int] = mapped_column(ForeignKey("files.id"), index=True)
    index_id: Mapped[int] = mapped_column(ForeignKey("indexes.id"), index=True)
    content: Mapped[str] = mapped_column(UnicodeText, default="")

    def __init__(self, file_id: int, index_id: int, content: str) -> None:
        """Initialize the snippet."""
        super().__init__()
        self.file_id = file_id
        self.index_id = index_id
        self.content = content


class SnippetExtractionStrategy(str, Enum):
    """Different strategies for extracting snippets from files."""

    METHOD_BASED = "method_based"


class SnippetExtractionRequest:
    """Domain model for snippet extraction request."""

    def __init__(self, file_path: Path, strategy: SnippetExtractionStrategy):
        self.file_path = file_path
        self.strategy = strategy


class SnippetExtractionResult:
    """Domain model for snippet extraction result."""

    def __init__(self, snippets: list[str], language: str):
        self.snippets = snippets
        self.language = language


class BM25Document:
    """Domain model for BM25 document."""

    def __init__(self, snippet_id: int, text: str):
        self.snippet_id = snippet_id
        self.text = text


class BM25SearchResult:
    """Domain model for BM25 search result."""

    def __init__(self, snippet_id: int, score: float):
        self.snippet_id = snippet_id
        self.score = score


class BM25IndexRequest:
    """Domain model for BM25 indexing request."""

    def __init__(self, documents: list[BM25Document]):
        self.documents = documents


class BM25SearchRequest:
    """Domain model for BM25 search request."""

    def __init__(self, query: str, top_k: int = 10):
        self.query = query
        self.top_k = top_k


class BM25DeleteRequest:
    """Domain model for BM25 deletion request."""

    def __init__(self, snippet_ids: list[int]):
        self.snippet_ids = snippet_ids


class VectorSearchRequest:
    """Domain model for vector search request."""

    def __init__(self, snippet_id: int, text: str):
        self.snippet_id = snippet_id
        self.text = text


class VectorSearchResult:
    """Domain model for vector search result."""

    def __init__(self, snippet_id: int, score: float):
        self.snippet_id = snippet_id
        self.score = score


class EmbeddingRequest:
    """Domain model for embedding request."""

    def __init__(self, id: int, text: str):
        self.id = id
        self.text = text


class EmbeddingResponse:
    """Domain model for embedding response."""

    def __init__(self, id: int, embedding: list[float]):
        self.id = id
        self.embedding = embedding


class IndexResult:
    """Domain model for indexing result."""

    def __init__(self, snippet_id: int):
        self.snippet_id = snippet_id


class VectorIndexRequest:
    """Domain model for vector indexing request."""

    def __init__(self, documents: list[VectorSearchRequest]):
        self.documents = documents


class VectorSearchQueryRequest:
    """Domain model for vector search query request."""

    def __init__(self, query: str, top_k: int = 10):
        self.query = query
        self.top_k = top_k
