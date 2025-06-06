"""Repository for managing code indexes and their associated files and snippets.

This module provides the IndexRepository class which handles all database operations
related to code indexes, including creating indexes, managing files and snippets,
and retrieving index information with their associated metadata.
"""

from datetime import UTC, datetime
from typing import TypeVar

from sqlalchemy import delete, func, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.embedding.embedding_models import Embedding
from kodit.indexing.indexing_models import Index, Snippet
from kodit.source.source_models import File, Source

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

    async def create(self, source_id: int) -> Index:
        """Create a new index for a source.

        Args:
            source_id: The ID of the source to create an index for.

        Returns:
            The newly created Index instance.

        """
        index = Index(source_id=source_id)
        self.session.add(index)
        await self.session.commit()
        return index

    async def get_by_id(self, index_id: int) -> Index | None:
        """Get an index by its ID.

        Args:
            index_id: The ID of the index to retrieve.

        Returns:
            The Index instance if found, None otherwise.

        """
        query = select(Index).where(Index.id == index_id)
        result = await self.session.execute(query)
        return result.scalar_one_or_none()

    async def get_by_source_id(self, source_id: int) -> Index | None:
        """Get an index by its source ID.

        Args:
            source_id: The ID of the source to retrieve an index for.

        """
        query = select(Index).where(Index.source_id == source_id)
        result = await self.session.execute(query)
        return result.scalar_one_or_none()

    async def files_for_index(self, index_id: int) -> list[File]:
        """Get all files for an index.

        Args:
            index_id: The ID of the index to get files for.

        Returns:
            A list of File instances.

        """
        query = (
            select(File)
            .join(Source, File.source_id == Source.id)
            .join(Index, Index.source_id == Source.id)
            .where(Index.id == index_id)
        )
        result = await self.session.execute(query)
        return list(result.scalars())

    async def list_indexes(self) -> list[tuple[Index, Source]]:
        """List all indexes.

        Returns:
            A list of tuples containing index information, source details,
            and counts of files and snippets.

        """
        query = select(Index, Source).join(
            Source, Index.source_id == Source.id, full=True
        )
        result = await self.session.execute(query)
        return list(result.tuples())

    async def num_snippets_for_index(self, index_id: int) -> int:
        """Get the number of snippets for an index."""
        query = select(func.count()).where(Snippet.index_id == index_id)
        result = await self.session.execute(query)
        return result.scalar_one()

    async def update_index_timestamp(self, index: Index) -> None:
        """Update the updated_at timestamp of an index.

        Args:
            index: The Index instance to update.

        """
        index.updated_at = datetime.now(UTC)
        await self.session.commit()

    async def add_snippet(self, snippet: Snippet) -> None:
        """Add a new snippet to the database if it doesn't exist, otherwise update it.

        Args:
            snippet: The Snippet instance to add.

        """
        self.session.add(snippet)
        await self.session.commit()

    async def delete_all_snippets(self, index_id: int) -> None:
        """Delete all snippets for an index.

        Args:
            index_id: The ID of the index to delete snippets for.

        """
        # First get all snippets for this index
        snippets = await self.get_snippets_for_index(index_id)

        # Delete all embeddings for these snippets, if there are any
        for snippet in snippets:
            query = delete(Embedding).where(Embedding.snippet_id == snippet.id)
            await self.session.execute(query)

        # Now delete the snippets
        query = delete(Snippet).where(Snippet.index_id == index_id)
        await self.session.execute(query)
        await self.session.commit()

    async def get_snippets_for_index(self, index_id: int) -> list[Snippet]:
        """Get all snippets for an index.

        Args:
            index_id: The ID of the index to get snippets for.

        """
        query = select(Snippet).where(Snippet.index_id == index_id)
        result = await self.session.execute(query)
        return list(result.scalars())

    async def get_all_snippets(self, index_id: int) -> list[Snippet]:
        """Get all snippets.

        Returns:
            A list of all snippets.

        """
        query = select(Snippet).where(Snippet.index_id == index_id).order_by(Snippet.id)
        result = await self.session.execute(query)
        return list(result.scalars())

    async def add_embedding(self, embedding: Embedding) -> None:
        """Add a new embedding to the database.

        Args:
            embedding: The Embedding instance to add.

        """
        self.session.add(embedding)
        await self.session.commit()

    async def list_snippets_by_ids(self, ids: list[int]) -> list[tuple[File, Snippet]]:
        """List snippets by IDs.

        Returns:
            A list of snippets in the same order as the input IDs.

        """
        query = (
            select(Snippet, File)
            .where(Snippet.id.in_(ids))
            .join(File, Snippet.file_id == File.id)
        )
        rows = await self.session.execute(query)

        # Create a dictionary for O(1) lookup of results by ID
        id_to_result = {snippet.id: (file, snippet) for snippet, file in rows.all()}

        # Check that all IDs are present
        if len(id_to_result) != len(ids):
            # Create a list of missing IDs
            missing_ids = [
                snippet_id for snippet_id in ids if snippet_id not in id_to_result
            ]
            msg = f"Some IDs are not present: {missing_ids}"
            raise ValueError(msg)

        # Rebuild the list in the same order that it was passed in
        return [id_to_result[i] for i in ids]
