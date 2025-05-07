"""Repository for managing code indexes and their associated files and snippets.

This module provides the IndexRepository class which handles all database operations
related to code indexes, including creating indexes, managing files and snippets,
and retrieving index information with their associated metadata.
"""

from datetime import UTC, datetime
from typing import Any, TypeVar, cast

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.sql import Select

from kodit.indexes.models import File, Snippet
from kodit.indexes.models import Index as IndexModel
from kodit.sources.models import FolderSource, GitSource, Source

T = TypeVar("T")


class IndexRepository:
    """Repository for managing code indexes and their associated data.

    This class provides methods for creating and managing code indexes, including
    their associated files and snippets. It handles all database operations related
    to indexing code sources.
    """

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the index repository.

        Args:
            session: The SQLAlchemy async session to use for database operations.

        """
        self.session = session

    async def _execute_query(self, query: Select[Any], single: bool = False) -> Any:
        """Execute a SQLAlchemy query and return the results.

        Args:
            query: The SQLAlchemy select query to execute.
            single: Whether to return a single result or a list of results.

        Returns:
            The query results, either as a single item or a list.

        """
        result = await self.session.execute(query)
        return result.scalar_one_or_none() if single else result.all()

    async def create(self, source_id: int) -> IndexModel:
        """Create a new index for a source.

        Args:
            source_id: The ID of the source to create an index for.

        Returns:
            The newly created IndexModel instance.

        """
        index = IndexModel(source_id=source_id)
        self.session.add(index)
        await self.session.commit()
        return index

    async def get_by_id(self, index_id: int) -> IndexModel | None:
        """Get an index by its ID.

        Args:
            index_id: The ID of the index to retrieve.

        Returns:
            The IndexModel instance if found, None otherwise.

        """
        query = select(IndexModel).where(IndexModel.id == index_id)
        return await self._execute_query(query, single=True)

    async def get_source_details(
        self, source_id: int
    ) -> tuple[Source, GitSource | None, FolderSource | None] | None:
        """Get detailed information about a source including its type-specific data.

        Args:
            source_id: The ID of the source to get details for.

        Returns:
            A tuple containing the source and its type-specific data (git or folder),
            or None if the source is not found.

        """
        query = (
            select(Source, GitSource, FolderSource)
            .where(Source.id == source_id)
            .outerjoin(GitSource, Source.id == GitSource.source_id)
            .outerjoin(FolderSource, Source.id == FolderSource.source_id)
        )
        return await self._execute_query(query, single=True)

    async def list_with_details(self) -> list[tuple]:
        """List all indexes with their associated metadata and statistics.

        Returns:
            A list of tuples containing index information, source details,
            and counts of files and snippets.

        """
        query = (
            select(
                IndexModel,
                Source,
                GitSource,
                FolderSource,
                func.count(File.id).label("file_count"),
                func.count(Snippet.id).label("snippet_count"),
            )
            .join(Source, IndexModel.source_id == Source.id)
            .outerjoin(GitSource, Source.id == GitSource.source_id)
            .outerjoin(FolderSource, Source.id == FolderSource.source_id)
            .outerjoin(File, Source.id == File.source_id)
            .outerjoin(Snippet, File.id == Snippet.file_id)
            .group_by(IndexModel.id, Source.id, GitSource.id, FolderSource.id)
        )
        return await self._execute_query(query)

    async def get_existing_files(self, source_id: int) -> set[str]:
        """Get the set of SHA256 hashes for files already indexed in a source.

        Args:
            source_id: The ID of the source to get file hashes for.

        Returns:
            A set of SHA256 hashes for files already indexed in the source.

        """
        query = select(File.sha256).where(File.source_id == source_id)
        result = await self._execute_query(query)
        return set(cast("list[str]", result))

    async def get_existing_snippets(self, index_id: int) -> set[int]:
        """Get the set of file IDs that already have snippets in an index.

        Args:
            index_id: The ID of the index to get snippet file IDs for.

        Returns:
            A set of file IDs that already have snippets in the index.

        """
        query = select(Snippet).where(Snippet.index_id == index_id)
        result = await self._execute_query(query)
        return {snippet.file_id for snippet in result}

    async def update_index_timestamp(self, index: IndexModel) -> None:
        """Update the updated_at timestamp of an index.

        Args:
            index: The IndexModel instance to update.

        """
        index.updated_at = datetime.now(UTC)
        await self.session.commit()

    async def get_files_by_source(self, source_id: int) -> list[File]:
        """Get all files associated with a source.

        Args:
            source_id: The ID of the source to get files for.

        Returns:
            A list of File instances associated with the source.

        """
        query = select(File).where(File.source_id == source_id)
        return await self._execute_query(query)

    async def add_file(self, file: File) -> None:
        """Add a new file to the database.

        Args:
            file: The File instance to add.

        """
        self.session.add(file)
        await self.session.commit()

    async def add_snippet(self, snippet: Snippet) -> None:
        """Add a new snippet to the database.

        Args:
            snippet: The Snippet instance to add.

        """
        self.session.add(snippet)
        await self.session.commit()

    async def commit(self) -> None:
        """Commit any pending changes to the database."""
        await self.session.commit()
