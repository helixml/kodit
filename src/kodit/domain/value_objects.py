"""Domain value objects and DTOs."""

from dataclasses import dataclass
from datetime import datetime
from enum import Enum
from pathlib import Path

from kodit.domain.enums import SnippetExtractionStrategy


class SearchType(Enum):
    """Type of search to perform."""

    BM25 = "bm25"
    VECTOR = "vector"
    HYBRID = "hybrid"


@dataclass
class SnippetExtractionRequest:
    """Domain model for snippet extraction request."""

    file_path: Path
    strategy: SnippetExtractionStrategy = SnippetExtractionStrategy.METHOD_BASED


@dataclass
class SnippetExtractionResult:
    """Domain model for snippet extraction result."""

    snippets: list[str]
    language: str


@dataclass
class Document:
    """Generic document model for indexing."""

    snippet_id: int
    text: str


@dataclass
class SearchResult:
    """Generic search result model."""

    snippet_id: int
    score: float


@dataclass
class IndexRequest:
    """Generic indexing request."""

    documents: list[Document]


@dataclass
class SimpleSearchRequest:
    """Generic search request (single query string)."""

    query: str
    top_k: int = 10
    search_type: SearchType = SearchType.BM25


@dataclass
class DeleteRequest:
    """Generic deletion request."""

    snippet_ids: list[int]


@dataclass
class IndexResult:
    """Generic indexing result."""

    snippet_id: int


# Legacy aliases for backward compatibility
BM25Document = Document
BM25SearchResult = SearchResult
BM25IndexRequest = IndexRequest
BM25SearchRequest = SimpleSearchRequest
BM25DeleteRequest = DeleteRequest

VectorSearchRequest = Document
VectorSearchResult = SearchResult
VectorIndexRequest = IndexRequest
VectorSearchQueryRequest = SimpleSearchRequest


@dataclass
class MultiSearchRequest:
    """Domain model for multi-modal search request."""

    top_k: int = 10
    text_query: str | None = None
    code_query: str | None = None
    keywords: list[str] | None = None


@dataclass
class MultiSearchResult:
    """Domain model for multi-modal search result."""

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


@dataclass
class EmbeddingRequest:
    """Domain model for embedding request."""

    snippet_id: int
    text: str


@dataclass
class EmbeddingResponse:
    """Domain model for embedding response."""

    snippet_id: int
    embedding: list[float]


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
class SnippetListItem:
    """Domain model for snippet list item with file information."""

    id: int
    file_path: str
    content: str
    source_uri: str
