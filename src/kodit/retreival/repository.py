"""Retrieval repository."""

import pydantic
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.indexes.models import File, Snippet


class RetrievalResult(pydantic.BaseModel):
    """Retrieval result."""

    file_path: str
    content: str


class RetrievalRepository:
    """Retrieval repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the retrieval repository."""
        self.session = session

    async def string_search(self, query: str) -> list[RetrievalResult]:
        """Search for snippets containing the given query string."""
        results = await self.session.execute(
            select(Snippet, File)
            .where(Snippet.content.ilike(f"%{query}%"))
            .outerjoin(File, Snippet.file_id == File.id)
            .order_by(Snippet.created_at)
            .limit(10)
        )
        rows = results.all()

        return [
            RetrievalResult(
                file_path=file.path,
                content=snippet.content,
            )
            for (snippet, file) in rows
        ]
