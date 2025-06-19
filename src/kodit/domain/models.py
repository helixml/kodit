"""Domain models - re-exports for backward compatibility."""

# Re-export SQLAlchemy entities
from kodit.domain.entities import (
    Author,
    AuthorFileMapping,
    Base,
    CommonMixin,
    Embedding,
    EmbeddingType,
    File,
    Index,
    Snippet,
    Source,
    SourceType,
)

# Re-export enums
from kodit.domain.enums import SnippetExtractionStrategy

# Re-export interfaces
from kodit.domain.interfaces import NullProgressCallback, ProgressCallback

# Re-export value objects and DTOs
from kodit.domain.value_objects import (
    BM25DeleteRequest,
    BM25Document,
    BM25IndexRequest,
    BM25SearchRequest,
    BM25SearchResult,
    EmbeddingRequest,
    EmbeddingResponse,
    EnrichmentIndexRequest,
    EnrichmentRequest,
    EnrichmentResponse,
    EnrichmentSearchRequest,
    FusionRequest,
    FusionResult,
    IndexCreateRequest,
    IndexResult,
    IndexRunRequest,
    IndexView,
    ProgressEvent,
    SearchRequest,
    SearchResult,
    SnippetExtractionRequest,
    SnippetExtractionResult,
    VectorIndexRequest,
    VectorSearchQueryRequest,
    VectorSearchRequest,
    VectorSearchResult,
)

__all__ = [
    # SQLAlchemy entities
    "Author",
    "AuthorFileMapping",
    "Base",
    "CommonMixin",
    "Embedding",
    "EmbeddingType",
    "File",
    "Index",
    "Snippet",
    "Source",
    "SourceType",
    # Enums
    "SnippetExtractionStrategy",
    # Value objects and DTOs
    "BM25DeleteRequest",
    "BM25Document",
    "BM25IndexRequest",
    "BM25SearchRequest",
    "BM25SearchResult",
    "EmbeddingRequest",
    "EmbeddingResponse",
    "EnrichmentIndexRequest",
    "EnrichmentRequest",
    "EnrichmentResponse",
    "EnrichmentSearchRequest",
    "FusionRequest",
    "FusionResult",
    "IndexCreateRequest",
    "IndexResult",
    "IndexRunRequest",
    "IndexView",
    "ProgressEvent",
    "SearchRequest",
    "SearchResult",
    "SnippetExtractionRequest",
    "SnippetExtractionResult",
    "VectorIndexRequest",
    "VectorSearchQueryRequest",
    "VectorSearchRequest",
    "VectorSearchResult",
    # Interfaces
    "NullProgressCallback",
    "ProgressCallback",
]
