"""Query parameters for the API."""

from typing import Annotated

from fastapi import Depends, Query


class PaginationParams:
    """Pagination parameters for the API."""

    def __init__(
        self,
        page: int = Query(1, ge=1, description="Page number, starting from 1"),
        page_size: int = Query(20, ge=1, le=100, description="Items per page"),
    ) -> None:
        """Initialize pagination parameters."""
        self.page = page
        self.page_size = page_size
        self.offset = (page - 1) * page_size
        self.limit = page_size


PaginationParamsDep = Annotated[PaginationParams, Depends()]
