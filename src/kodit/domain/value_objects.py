"""Domain value objects and DTOs."""

from dataclasses import dataclass
from datetime import datetime
from pathlib import Path

from kodit.domain.enums import SnippetExtractionStrategy


class SnippetExtractionRequest:
    """Domain model for snippet extraction request."""

    def __init__(self, file_path: Path, strategy: SnippetExtractionStrategy) -> None:
        """Initialize the snippet extraction request."""
        self.file_path = file_path
        self.strategy = strategy


class SnippetExtractionResult:
    """Domain model for snippet extraction result."""

    def __init__(self, snippets: list[str], language: str) -> None:
        """Initialize the snippet extraction result."""
        self.snippets = snippets
        self.language = language


class BM25Document:
    """Domain model for BM25 document."""

    def __init__(self, snippet_id: int, text: str) -> None:
        """Initialize the BM25 document."""
        self.snippet_id = snippet_id
        self.text = text


class BM25SearchResult:
    """Domain model for BM25 search result."""

    def __init__(self, snippet_id: int, score: float) -> None:
        """Initialize the BM25 search result."""
        self.snippet_id = snippet_id
        self.score = score


class BM25IndexRequest:
    """Domain model for BM25 indexing request."""

    def __init__(self, documents: list[BM25Document]) -> None:
        """Initialize the BM25 indexing request."""
        self.documents = documents


class BM25SearchRequest:
    """Domain model for BM25 search request."""

    def __init__(self, query: str, top_k: int = 10) -> None:
        """Initialize the BM25 search request."""
        self.query = query
        self.top_k = top_k


class BM25DeleteRequest:
    """Domain model for BM25 deletion request."""

    def __init__(self, snippet_ids: list[int]) -> None:
        """Initialize the BM25 deletion request."""
        self.snippet_ids = snippet_ids


class VectorSearchRequest:
    """Domain model for vector search request."""

    def __init__(self, snippet_id: int, text: str) -> None:
        """Initialize the vector search request."""
        self.snippet_id = snippet_id
        self.text = text


class VectorSearchResult:
    """Domain model for vector search result."""

    def __init__(self, snippet_id: int, score: float) -> None:
        """Initialize the vector search result."""
        self.snippet_id = snippet_id
        self.score = score


class EmbeddingRequest:
    """Domain model for embedding request."""

    def __init__(self, snippet_id: int, text: str) -> None:
        """Initialize the embedding request."""
        self.snippet_id = snippet_id
        self.text = text


class EmbeddingResponse:
    """Domain model for embedding response."""

    def __init__(self, snippet_id: int, embedding: list[float]) -> None:
        """Initialize the embedding response."""
        self.snippet_id = snippet_id
        self.embedding = embedding


class IndexResult:
    """Domain model for indexing result."""

    def __init__(self, snippet_id: int) -> None:
        """Initialize the indexing result."""
        self.snippet_id = snippet_id


class VectorIndexRequest:
    """Domain model for vector indexing request."""

    def __init__(self, documents: list[VectorSearchRequest]) -> None:
        """Initialize the vector indexing request."""
        self.documents = documents


class VectorSearchQueryRequest:
    """Domain model for vector search query request."""

    def __init__(self, query: str, top_k: int = 10) -> None:
        """Initialize the vector search query request."""
        self.query = query
        self.top_k = top_k


@dataclass
class EnrichmentRequest:
    """Domain model for enrichment request."""

    snippet_id: int
    text: str


@dataclass
class EnrichmentResponse:
    """Domain model for enrichment response."""

    snippet_id: int
    text: str


@dataclass
class EnrichmentIndexRequest:
    """Domain model for enrichment index request."""

    requests: list[EnrichmentRequest]


@dataclass
class EnrichmentSearchRequest:
    """Domain model for enrichment search request."""

    query: str
    top_k: int = 10


@dataclass
class IndexView:
    """Domain model for index information."""

    id: int
    created_at: datetime
    num_snippets: int
    updated_at: datetime | None = None
    source: str | None = None


@dataclass
class SearchRequest:
    """Domain model for search request."""

    top_k: int = 10
    text_query: str | None = None
    code_query: str | None = None
    keywords: list[str] | None = None


@dataclass
class SearchResult:
    """Domain model for search result."""

    id: int
    uri: str
    content: str
    original_scores: list[float]


@dataclass
class FusionRequest:
    """Domain model for fusion request."""

    id: int
    score: float


@dataclass
class FusionResult:
    """Domain model for fusion result."""

    id: int
    score: float
    original_scores: list[float]


@dataclass
class IndexCreateRequest:
    """Domain model for index creation request."""

    source_id: int


@dataclass
class IndexRunRequest:
    """Domain model for index run request."""

    index_id: int


@dataclass
class ProgressEvent:
    """Domain model for progress events."""

    operation: str
    current: int
    total: int
    message: str | None = None

    @property
    def percentage(self) -> float:
        """Calculate the percentage of completion."""
        return (self.current / self.total * 100) if self.total > 0 else 0.0
