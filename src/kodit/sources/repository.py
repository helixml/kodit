"""Source repository."""

from datetime import datetime
from pathlib import Path

import pydantic
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.sources.models import FolderSource, GitSource
from kodit.sources.models import Source as SourceModel


class Source(pydantic.BaseModel):
    """Source model."""

    id: int
    uri: str
    created_at: datetime


class SourceRepository:
    """Source repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the source repository."""
        self.session = session

    async def create(self, uri: str) -> Source:
        """Create a source.

        This will try it's best to infer the type of source from the URI.
        """
        if uri.startswith("https://"):
            return await self._create_git_source(uri)
        if Path(uri).is_dir():
            return await self._create_folder_source(uri)
        msg = f"Unsupported source type: {uri}. Please pass a git-like URI or a local directory."
        raise ValueError(msg)

    async def _create_git_source(self, uri: str) -> None:
        """Create a git source."""
        source = SourceModel(name=uri)
        self.session.add(source)
        await self.session.commit()  # Commit to get the source.id
        git_source = GitSource(source_id=source.id, uri=uri)
        self.session.add(git_source)
        await self.session.commit()

    async def _create_folder_source(self, uri: str) -> None:
        """Create a folder source."""
        # Expand uri into a full path
        uri = Path(uri).expanduser().resolve()

        # Check if the folder exists
        if not uri.exists():
            msg = f"Folder does not exist: {uri}"
            raise ValueError(msg)

        # Check if that folder is already added
        query = select(FolderSource).where(FolderSource.path == str(uri))
        result = await self.session.execute(query)
        if result.scalar_one_or_none() is not None:
            msg = f"Folder already added: {uri}"
            raise ValueError(msg)

        source = SourceModel()
        self.session.add(source)
        await self.session.commit()  # Commit to get the source.id
        folder_source = FolderSource(source_id=source.id, path=str(uri))
        self.session.add(folder_source)
        await self.session.commit()

    async def list(self) -> list[Source]:
        """List sources."""
        query = (
            select(SourceModel, GitSource, FolderSource)
            .outerjoin(GitSource, SourceModel.id == GitSource.source_id)
            .outerjoin(FolderSource, SourceModel.id == FolderSource.source_id)
        )
        result = await self.session.execute(query)
        rows = result.all()

        # Map to Pydantic model
        return [
            Source(
                id=source.id,
                uri=git_source.uri
                if git_source
                else folder_source.path
                if folder_source
                else None,
                created_at=source.created_at,
            )
            for source, git_source, folder_source in rows
        ]
