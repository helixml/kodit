"""SQLAlchemy implementation of file repository."""

from collections.abc import Sequence

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import File, Index
from kodit.domain.repositories import FileRepository


class SqlAlchemyFileRepository(FileRepository):
    """SQLAlchemy implementation of file repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the SQLAlchemy file repository.

        Args:
            session: The SQLAlchemy async session to use for database operations

        """
        self.session = session

    async def get_files_for_index(self, index_id: int) -> Sequence[File]:
        """Get all files for an index.

        Args:
            index_id: The ID of the index to get files for

        Returns:
            A list of File instances

        """
        # Get the index first to find its source_id
        index_query = select(Index).where(Index.id == index_id)
        index_result = await self.session.execute(index_query)
        index = index_result.scalar_one_or_none()

        if not index:
            return []

        # Get all files for the source
        query = select(File).where(File.source_id == index.source_id)
        result = await self.session.execute(query)
        return list(result.scalars())

    async def get_by_id(self, file_id: int) -> File | None:
        """Get a file by ID.

        Args:
            file_id: The ID of the file to retrieve

        Returns:
            The File instance if found, None otherwise

        """
        query = select(File).where(File.id == file_id)
        result = await self.session.execute(query)
        return result.scalar_one_or_none()

    async def save(self, file: File) -> File:
        """Save file using SQLAlchemy.

        Args:
            file: The file to save

        Returns:
            The saved file

        """
        self.session.add(file)
        await self.session.commit()
        return file
