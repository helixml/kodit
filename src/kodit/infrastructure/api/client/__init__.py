"""API client for remote Kodit server communication."""

from .base import BaseAPIClient
from .exceptions import AuthenticationError, KoditAPIError
from .search_client import SearchClient

__all__ = [
    "AuthenticationError",
    "BaseAPIClient",
    "KoditAPIError",
    "SearchClient",
]
