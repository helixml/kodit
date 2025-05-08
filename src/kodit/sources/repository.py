"""Source repository for database operations.

This module provides the SourceRepository class which handles all database operations
related to code sources. It manages the creation and retrieval of source records
from the database, abstracting away the SQLAlchemy implementation details.
"""

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.sources.models import File, Source


class SourceRepository:
    """Repository for managing source database operations.

    This class provides methods for creating and retrieving source records from the
    database. It handles the low-level database operations and transaction management.

    Args:
        session: The SQLAlchemy async session to use for database operations.

    """

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the source repository."""
        self.session = session

    async def create_source(self, source: Source) -> Source:
        """Create a new folder source record in the database.

        This method creates both a Source record and a linked FolderSource record
        in a single transaction.

        Args:
            path: The absolute path of the folder to create a source for.

        Returns:
            The created Source model instance.

        Note:
            This method commits the transaction to ensure the source.id is available
            for creating the linked FolderSource record.

        """
        self.session.add(source)
        await self.session.commit()
        return source

    async def create_file(self, file: File) -> File:
        """Create a new file record in the database.

        This method creates a new File record and adds it to the session.

        """
        self.session.add(file)
        await self.session.commit()
        return file

    async def num_files_for_source(self, source_id: int) -> int:
        """Get the number of files for a source.

        Args:
            source_id: The ID of the source to get the number of files for.

        Returns:
            The number of files for the source.

        """
        query = (
            select(func.count()).select_from(File).where(File.source_id == source_id)
        )
        result = await self.session.execute(query)
        return result.scalar_one()

    async def list_sources(self) -> list[Source]:
        """Retrieve all sources from the database.

        Returns:
            A list of Source instances.

        """
        query = select(Source).limit(10)
        result = await self.session.execute(query)
        return list(result.scalars())

    async def get_source_by_uri(self, uri: str) -> Source | None:
        """Get a source by its URI.

        Args:
            uri: The URI of the source to get.

        Returns:
            The source with the given URI, or None if it does not exist.

        """
        query = select(Source).where(Source.uri == uri)
        result = await self.session.execute(query)
        return result.scalar_one_or_none()

    async def get_source_by_id(self, source_id: int) -> Source | None:
        """Get a source by its ID.

        Args:
            source_id: The ID of the source to get.

        """
        query = select(Source).where(Source.id == source_id)
        result = await self.session.execute(query)
        return result.scalar_one_or_none()
