"""API client for remote Kodit server communication."""

from .base import BaseAPIClient
from .exceptions import AuthenticationError, KoditAPIError
from .index_client import IndexClient
from .search_client import SearchClient

__all__ = [
    "AuthenticationError",
    "BaseAPIClient",
    "IndexClient",
    "KoditAPIError",
    "SearchClient",
]
