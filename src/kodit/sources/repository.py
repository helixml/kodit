"""Source repository for database operations.

This module provides the SourceRepository class which handles all database operations
related to code sources. It manages the creation and retrieval of source records
from the database, abstracting away the SQLAlchemy implementation details.
"""

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.sources.models import FolderSource, GitSource, Source


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

    async def create_git_source(self, uri: str) -> Source:
        """Create a new git source record in the database.

        This method creates both a Source record and a linked GitSource record
        in a single transaction.

        Args:
            uri: The URI of the git repository to create a source for.

        Returns:
            The created Source model instance.

        Note:
            This method commits the transaction to ensure the source.id is available
            for creating the linked GitSource record.

        """
        source = Source()
        self.session.add(source)
        await self.session.commit()  # Commit to get the source.id
        git_source = GitSource(source_id=source.id, uri=uri)
        self.session.add(git_source)
        await self.session.commit()
        return source

    async def create_folder_source(self, path: str) -> Source:
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
        source = Source()
        self.session.add(source)
        await self.session.commit()  # Commit to get the source.id
        folder_source = FolderSource(source_id=source.id, path=path)
        self.session.add(folder_source)
        await self.session.commit()
        return source

    async def list_sources(
        self,
    ) -> list[tuple[Source, GitSource | None, FolderSource | None]]:
        """Retrieve all sources from the database with their associated details.

        This method performs a left outer join to get all sources and their
        associated git or folder source details, if any exist.

        Returns:
            A list of tuples containing (Source, GitSource, FolderSource) where
            GitSource and FolderSource may be None if the source is not of that type.

        """
        query = (
            select(Source, GitSource, FolderSource)
            .outerjoin(GitSource, Source.id == GitSource.source_id)
            .outerjoin(FolderSource, Source.id == FolderSource.source_id)
        )
        result = await self.session.execute(query)
        return result.all()
