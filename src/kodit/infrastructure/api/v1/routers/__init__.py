"""API v1 routers."""

from .indexes import router as indexes_router
from .search import router as search_router

__all__ = ["indexes_router", "search_router"]
