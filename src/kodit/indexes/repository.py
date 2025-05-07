"""Source repository."""

from datetime import datetime

import pydantic
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.indexes.models import Index as IndexModel
from kodit.sources.models import FolderSource, GitSource, Source


class Index(pydantic.BaseModel):
    """Index model."""

    id: int
    created_at: datetime
    source_id: int
    source_uri: str | None = None
    updated_at: datetime | None = None


class IndexRepository:
    """Index repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the index repository."""
        self.session = session

    async def create(self, source_id: int) -> Index:
        """Create an index."""
        # First, check if the source exists
        source = await self.session.execute(
            select(Source).where(Source.id == source_id)
        )
        if not source.scalar_one_or_none():
            msg = f"Source not found, please create it first: {source_id}"
            raise ValueError(msg)

        # Now check if there is already an index on this source
        index = await self.session.execute(
            select(IndexModel).where(IndexModel.source_id == source_id)
        )
        if index.scalar_one_or_none():
            msg = f"Index already exists on this source: {source_id}"
            raise ValueError(msg)

        index = IndexModel(source_id=source_id)
        self.session.add(index)
        await self.session.commit()
        return Index(
            id=index.id, created_at=index.created_at, source_id=index.source_id
        )

    async def list(self) -> list[Index]:
        """List indexes."""
        query = (
            select(IndexModel, Source, GitSource, FolderSource)
            .join(Source, IndexModel.source_id == Source.id)
            .outerjoin(GitSource, Source.id == GitSource.source_id)
            .outerjoin(FolderSource, Source.id == FolderSource.source_id)
        )
        result = await self.session.execute(query)
        rows = result.all()
        # Map to Pydantic model
        return [
            Index(
                id=index.id,
                created_at=index.created_at,
                source_id=index.source_id,
                source_uri=git_source.uri
                if git_source
                else folder_source.path
                if folder_source
                else None,
            )
            for index, source, git_source, folder_source in rows
        ]
