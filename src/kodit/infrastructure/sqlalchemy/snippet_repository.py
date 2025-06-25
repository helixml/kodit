"""SQLAlchemy implementation of snippet repository."""

from collections.abc import Sequence
from pathlib import Path

from sqlalchemy import delete, or_, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import (
    Author,
    AuthorFileMapping,
    Embedding,
    File,
    Snippet,
    Source,
)
from kodit.domain.repositories import SnippetRepository
from kodit.domain.value_objects import (
    LanguageMapping,
    MultiSearchRequest,
    SnippetListItem,
)


class SqlAlchemySnippetRepository(SnippetRepository):
    """SQLAlchemy implementation of snippet repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the SQLAlchemy snippet repository.

        Args:
            session: The SQLAlchemy async session to use for database operations

        """
        self.session = session

    async def get(self, id: int) -> Snippet | None:  # noqa: A002
        """Get a snippet by ID."""
        return await self.session.get(Snippet, id)

    async def save(self, entity: Snippet) -> Snippet:
        """Save entity."""
        self.session.add(entity)
        return entity

    async def delete(self, id: int) -> None:  # noqa: A002
        """Delete entity by ID."""
        snippet = await self.get(id)
        if snippet:
            await self.session.delete(snippet)

    async def list(self) -> Sequence[Snippet]:
        """List all entities."""
        return (await self.session.scalars(select(Snippet))).all()

    async def get_by_id(self, snippet_id: int) -> Snippet | None:
        """Get a snippet by ID.

        Args:
            snippet_id: The ID of the snippet to retrieve

        Returns:
            The Snippet instance if found, None otherwise

        """
        query = select(Snippet).where(Snippet.id == snippet_id)
        result = await self.session.execute(query)
        return result.scalar_one_or_none()

    async def get_by_index(self, index_id: int) -> Sequence[Snippet]:
        """Get all snippets for an index.

        Args:
            index_id: The ID of the index to get snippets for

        Returns:
            A list of Snippet instances

        """
        query = select(Snippet).where(Snippet.index_id == index_id)
        result = await self.session.execute(query)
        return list(result.scalars())

    async def delete_by_index(self, index_id: int) -> None:
        """Delete all snippets for an index.

        Args:
            index_id: The ID of the index to delete snippets for

        """
        # First get all snippets for this index
        snippets = await self.get_by_index(index_id)

        # Delete all embeddings for these snippets, if there are any
        for snippet in snippets:
            query = delete(Embedding).where(Embedding.snippet_id == snippet.id)
            await self.session.execute(query)

        # Now delete the snippets
        query = delete(Snippet).where(Snippet.index_id == index_id)
        await self.session.execute(query)

    async def list_snippets(
        self, file_path: str | None = None, source_uri: str | None = None
    ) -> Sequence[SnippetListItem]:
        """List snippets with optional filtering by file path and source URI.

        Args:
            file_path: Optional file or directory path to filter by. Can be relative
            (uri) or absolute (cloned_path).
            source_uri: Optional source URI to filter by. If None, returns snippets from
            all sources.

        Returns:
            A sequence of SnippetListItem instances matching the criteria

        """
        # Build the base query
        query = (
            select(
                Snippet,
                File.cloned_path,
                Source.cloned_path.label("source_cloned_path"),
                Source.uri.label("source_uri"),
            )
            .join(File, Snippet.file_id == File.id)
            .join(Source, File.source_id == Source.id)
        )

        # Apply filters
        if file_path is not None:
            query = query.where(
                or_(
                    File.cloned_path.like(f"%{file_path}%"),
                    File.uri.like(f"%{file_path}%"),
                )
            )

        if source_uri is not None:
            query = query.where(Source.uri == source_uri)

        result = await self.session.execute(query)
        return [
            SnippetListItem(
                id=snippet.id,
                file_path=self._get_relative_path(file_cloned_path, source_cloned_path),
                content=snippet.content,
                source_uri=source_uri_val,
            )
            for (
                snippet,
                file_cloned_path,
                source_cloned_path,
                source_uri_val,
            ) in result.all()
        ]

    def _get_relative_path(self, file_path: str, source_path: str) -> str:
        """Calculate the relative path of a file from the source root.

        Args:
            file_path: The full path to the file
            source_path: The full path to the source root

        Returns:
            The relative path from the source root

        """
        try:
            file_path_obj = Path(file_path)
            source_path_obj = Path(source_path)
            return str(file_path_obj.relative_to(source_path_obj))
        except ValueError:
            # If the file is not relative to the source, return the filename
            return Path(file_path).name

    async def search(self, request: MultiSearchRequest) -> Sequence[SnippetListItem]:
        """Search snippets with filters.

        Args:
            request: The search request containing queries and optional filters.

        Returns:
            A sequence of SnippetListItem instances matching the search criteria.

        """
        # Build the base query with joins
        query = (
            select(
                Snippet,
                File.cloned_path,
                Source.cloned_path.label("source_cloned_path"),
                Source.uri.label("source_uri"),
            )
            .join(File, Snippet.file_id == File.id)
            .join(Source, File.source_id == Source.id)
        )

        # Apply filters if provided
        if request.filters:
            filters = request.filters

            # Language filter (using file extension)
            if filters.language:
                extensions = LanguageMapping.get_extensions_with_fallback(
                    filters.language
                )
                query = query.where(File.extension.in_(extensions))

            # Author filter
            if filters.author:
                query = (
                    query.join(AuthorFileMapping, File.id == AuthorFileMapping.file_id)
                    .join(Author, AuthorFileMapping.author_id == Author.id)
                    .where(Author.name.ilike(f"%{filters.author}%"))
                )

            # Date filters
            if filters.created_after:
                query = query.where(Snippet.created_at >= filters.created_after)

            if filters.created_before:
                query = query.where(Snippet.created_at <= filters.created_before)

            # Source repository filter
            if filters.source_repo:
                query = query.where(Source.uri.like(f"%{filters.source_repo}%"))

        # Only apply top_k limit if there are no search queries
        # This ensures that when used for pre-filtering (with search queries),
        # all matching snippets are returned for the search services to consider
        if request.top_k and not any(
            [request.keywords, request.code_query, request.text_query]
        ):
            query = query.limit(request.top_k)

        result = await self.session.execute(query)
        return [
            SnippetListItem(
                id=snippet.id,
                file_path=self._get_relative_path(file_cloned_path, source_cloned_path),
                content=snippet.content,
                source_uri=source_uri_val,
            )
            for (
                snippet,
                file_cloned_path,
                source_cloned_path,
                source_uri_val,
            ) in result.all()
        ]
