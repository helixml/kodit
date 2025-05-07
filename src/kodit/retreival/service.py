"""Retrieval service."""

import pydantic

from kodit.retreival.repository import RetrievalRepository, RetrievalResult


class RetrievalRequest(pydantic.BaseModel):
    """Request for a retrieval."""

    query: str


class Snippet(pydantic.BaseModel):
    """Snippet model."""

    content: str
    file_path: str


class RetrievalService:
    """Service for retrieving relevant data."""

    def __init__(self, repository: RetrievalRepository) -> None:
        """Initialize the retrieval service."""
        self.repository = repository

    async def retrieve(self, request: RetrievalRequest) -> list[RetrievalResult]:
        """Retrieve relevant data."""
        return await self.repository.string_search(request.query)
