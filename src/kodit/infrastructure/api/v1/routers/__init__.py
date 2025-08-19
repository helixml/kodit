"""API v1 routers."""

from .indexes import router as indexes_router
from .queue import router as queue_router
from .search import router as search_router

__all__ = ["indexes_router", "queue_router", "search_router"]
