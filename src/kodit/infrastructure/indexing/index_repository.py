"""Infrastructure implementation of the index repository."""

from datetime import UTC, datetime
from typing import TypeVar

from sqlalchemy import delete, func, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Embedding, File, Index, Snippet, Source
from kodit.domain.services.indexing_service import IndexRepository
from kodit.domain.value_objects import FileInfo, IndexView, SnippetInfo, SnippetWithFile

T = TypeVar("T")


class SQLAlchemyIndexRepository(IndexRepository):
    """SQLAlchemy implementation of the index repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the index repository.

        Args:
            session: The SQLAlchemy async session to use for database operations.

        """
        self.session = session

    async def create_index(self, source_id: int) -> IndexView:
        """Create a new index for a source.

        Args:
            source_id: The ID of the source to create an index for.

        Returns:
            The created index view.

        """
        # Check if index already exists
        existing_index = await self.get_index_by_source_id(source_id)
        if existing_index:
            return existing_index

        index = Index(source_id=source_id)
        self.session.add(index)

        # Get source for the view
        source_query = select(Source).where(Source.id == source_id)
        source_result = await self.session.execute(source_query)
        source = source_result.scalar_one()

        return IndexView(
            id=index.id,
            created_at=index.created_at,
            updated_at=index.updated_at,
            source=source.uri,
            num_snippets=0,
        )

    async def _get_index_view(self, index: Index, source: Source) -> IndexView:
        """Create an IndexView from Index and Source entities.

        Args:
            index: The index entity
            source: The source entity

        Returns:
            The index view

        """
        num_snippets = await self.num_snippets_for_index(index.id)
        return IndexView(
            id=index.id,
            created_at=index.created_at,
            updated_at=index.updated_at,
            source=source.uri,
            num_snippets=num_snippets,
        )

    async def get_index_by_id(self, index_id: int) -> IndexView | None:
        """Get an index by its ID.

        Args:
            index_id: The ID of the index to retrieve.

        Returns:
            The index view if found, None otherwise.

        """
        query = (
            select(Index, Source)
            .join(Source, Index.source_id == Source.id)
            .where(Index.id == index_id)
        )
        result = await self.session.execute(query)
        row = result.first()

        if not row:
            return None

        index, source = row
        return await self._get_index_view(index, source)

    async def get_index_by_source_id(self, source_id: int) -> IndexView | None:
        """Get an index by its source ID.

        Args:
            source_id: The ID of the source to retrieve an index for.

        Returns:
            The index view if found, None otherwise.

        """
        query = (
            select(Index, Source)
            .join(Source, Index.source_id == Source.id)
            .where(Index.source_id == source_id)
        )
        result = await self.session.execute(query)
        row = result.first()

        if not row:
            return None

        index, source = row
        return await self._get_index_view(index, source)

    async def list_indexes(self) -> list[IndexView]:
        """List all indexes.

        Returns:
            A list of index views.

        """
        query = select(Index, Source).join(
            Source, Index.source_id == Source.id, full=True
        )
        result = await self.session.execute(query)
        rows = result.tuples()

        indexes = []
        for index, source in rows:
            index_view = await self._get_index_view(index, source)
            indexes.append(index_view)

        return indexes

    async def update_index_timestamp(self, index_id: int) -> None:
        """Update the timestamp of an index.

        Args:
            index_id: The ID of the index to update.

        """
        query = select(Index).where(Index.id == index_id)
        result = await self.session.execute(query)
        index = result.scalar_one_or_none()

        if index:
            index.updated_at = datetime.now(UTC)

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

    async def get_snippets_for_index(self, index_id: int) -> list[Snippet]:
        """Get all snippets for an index.

        Args:
            index_id: The ID of the index to get snippets for.

        Returns:
            A list of Snippet entities.

        """
        query = select(Snippet).where(Snippet.index_id == index_id)
        result = await self.session.execute(query)
        return list(result.scalars())

    async def add_snippet(self, snippet: dict) -> None:
        """Add a snippet to the database.

        Args:
            snippet: The snippet to add.

        """
        db_snippet = Snippet(
            file_id=snippet["file_id"],
            index_id=snippet["index_id"],
            content=snippet["content"],
        )
        self.session.add(db_snippet)

    async def update_snippet_content(self, snippet_id: int, content: str) -> None:
        """Update the content of an existing snippet.

        Args:
            snippet_id: The ID of the snippet to update.
            content: The new content for the snippet.

        """
        query = select(Snippet).where(Snippet.id == snippet_id)
        result = await self.session.execute(query)
        snippet = result.scalar_one_or_none()

        if snippet:
            snippet.content = content
            # SQLAlchemy will automatically track this change

    async def list_snippets_by_ids(self, ids: list[int]) -> list[SnippetWithFile]:
        """List snippets by IDs.

        Args:
            ids: List of snippet IDs to retrieve.

        Returns:
            List of SnippetWithFile objects containing file and snippet information.

        """
        query = (
            select(Snippet, File)
            .where(Snippet.id.in_(ids))
            .join(File, Snippet.file_id == File.id)
        )
        rows = await self.session.execute(query)

        # Create a dictionary for O(1) lookup of results by ID
        id_to_result = {}
        for snippet, file in rows.all():
            id_to_result[snippet.id] = SnippetWithFile(
                file=FileInfo(uri=file.uri),
                snippet=SnippetInfo(id=snippet.id, content=snippet.content)
            )

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

    async def num_snippets_for_index(self, index_id: int) -> int:
        """Get the number of snippets for an index.

        Args:
            index_id: The ID of the index.

        Returns:
            The number of snippets.

        """
        query = select(func.count()).where(Snippet.index_id == index_id)
        result = await self.session.execute(query)
        return result.scalar_one()
