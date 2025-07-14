"""JSON:API schemas for the REST API."""

from .index import (
    IndexCreateRequest,
    IndexDetailResponse,
    IndexListResponse,
    IndexResponse,
)
from .search import (
    SearchRequest,
    SearchResponse,
    SearchResponseWithIncluded,
    SnippetDetailResponse,
)

__all__ = [
    "IndexCreateRequest",
    "IndexDetailResponse",
    "IndexListResponse",
    "IndexResponse",
    "SearchRequest",
    "SearchResponse",
    "SearchResponseWithIncluded",
    "SnippetDetailResponse",
]
