"""Query parameters for the API."""

from typing import Annotated

from fastapi import Depends, Query


class PaginationParams:
    """Pagination parameters for the API."""

    def __init__(
        self,
        page: Annotated[
            int, Query(ge=1, description="Page number, starting from 1")
        ] = 1,
        page_size: Annotated[
            int, Query(ge=1, le=100, description="Items per page")
        ] = 20,
    ) -> None:
        """Initialize pagination parameters."""
        self.page = page
        self.page_size = page_size
        self.offset = (page - 1) * page_size
        self.limit = page_size


PaginationParamsDep = Annotated[PaginationParams, Depends()]
