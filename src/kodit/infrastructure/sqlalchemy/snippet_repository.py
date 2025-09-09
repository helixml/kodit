"""SQLAlchemy implementation of SnippetRepository."""

from collections.abc import Callable

from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain import entities as domain_entities
from kodit.domain.entities import SnippetWithContext
from kodit.domain.protocols import SnippetRepository
from kodit.domain.value_objects import MultiSearchRequest
from kodit.infrastructure.mappers.snippet_mapper import SnippetMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_snippet_repository(
    session_factory: Callable[[], AsyncSession],
) -> SnippetRepository:
    """Create a snippet repository."""
    uow = SqlAlchemyUnitOfWork(session_factory=session_factory)
    return SqlAlchemySnippetRepository(uow)


class SqlAlchemySnippetRepository(SnippetRepository):
    """SQLAlchemy implementation of SnippetRepository."""

    def __init__(self, uow: SqlAlchemyUnitOfWork) -> None:
        """Initialize the repository."""
        self.uow = uow

    @property
    def _mapper(self) -> SnippetMapper:
        if self.uow.session is None:
            raise RuntimeError("UnitOfWork must be used within async context")
        return SnippetMapper(self.uow.session)

    @property
    def _session(self) -> AsyncSession:
        if self.uow.session is None:
            raise RuntimeError("UnitOfWork must be used within async context")
        return self.uow.session

    async def add(self, snippets: list[domain_entities.Snippet], index_id: int) -> None:
        """Add snippets to an index."""
        if not snippets:
            return

        async with self.uow:
            # Validate the index exists
            db_index = await self._session.get(db_entities.Index, index_id)
            if not db_index:
                raise ValueError(f"Index {index_id} not found")

            # Convert domain snippets to database entities
            for domain_snippet in snippets:
                db_snippet = await self._mapper.from_domain_snippet(
                    domain_snippet, index_id
                )
                self._session.add(db_snippet)

    async def update(self, snippets: list[domain_entities.Snippet]) -> None:
        """Update existing snippets."""
        if not snippets:
            return

        async with self.uow:
            # Update each snippet
            for domain_snippet in snippets:
                if not domain_snippet.id:
                    raise ValueError("Snippet must have an ID for update")

                # Get the existing snippet
                db_snippet = await self._session.get(
                    db_entities.Snippet, domain_snippet.id
                )
                if not db_snippet:
                    raise ValueError(f"Snippet {domain_snippet.id} not found")

                db_snippet.content = domain_snippet.original_text()
                db_snippet.summary = domain_snippet.summary_text()

                # Update timestamps if provided
                if domain_snippet.updated_at:
                    db_snippet.updated_at = domain_snippet.updated_at

    async def get_by_ids(self, ids: list[int]) -> list[SnippetWithContext]:
        """Get snippets by their IDs."""
        if not ids:
            return []

        async with self.uow:
            # Query snippets by IDs
            query = select(db_entities.Snippet).where(db_entities.Snippet.id.in_(ids))

            result = await self._session.scalars(query)
            db_snippets = result.all()

            # Convert to SnippetWithContext
            snippet_contexts = []
            for db_snippet in db_snippets:
                snippet_context = await self._build_snippet_with_context(db_snippet)
                if snippet_context:
                    snippet_contexts.append(snippet_context)

            return snippet_contexts

    async def search(  # noqa: C901
        self, request: MultiSearchRequest
    ) -> list[SnippetWithContext]:
        """Search snippets with filters."""
        # Build base query joining all necessary tables
        query = (
            select(db_entities.Snippet)
            .join(db_entities.File, db_entities.Snippet.file_id == db_entities.File.id)
            .join(
                db_entities.Source, db_entities.File.source_id == db_entities.Source.id
            )
        )

        # Apply text search if provided
        if request.text_query:
            query = query.where(
                db_entities.Snippet.content.ilike(f"%{request.text_query}%")
            )

        # Apply code search if provided
        if request.code_query:
            query = query.where(
                db_entities.Snippet.content.ilike(f"%{request.code_query}%")
            )

        # Apply keyword search if provided
        if request.keywords:
            for keyword in request.keywords:
                query = query.where(db_entities.Snippet.content.ilike(f"%{keyword}%"))

        # Apply filters if provided
        if request.filters:
            if request.filters.source_repo:
                query = query.where(
                    db_entities.Source.uri.ilike(f"%{request.filters.source_repo}%")
                )

            if request.filters.file_path:
                query = query.where(
                    db_entities.File.uri.ilike(f"%{request.filters.file_path}%")
                )

            if request.filters.created_after:
                query = query.where(
                    db_entities.Snippet.created_at >= request.filters.created_after
                )

            if request.filters.created_before:
                query = query.where(
                    db_entities.Snippet.created_at <= request.filters.created_before
                )

        # Apply limit
        query = query.limit(request.top_k)

        # Execute query
        async with self.uow:
            result = await self._session.scalars(query)
            db_snippets = result.all()

            # Convert to SnippetWithContext
            snippet_contexts = []
            for db_snippet in db_snippets:
                snippet_context = await self._build_snippet_with_context(db_snippet)
                if snippet_context:
                    snippet_contexts.append(snippet_context)

            return snippet_contexts

    async def delete_by_index_id(self, index_id: int) -> None:
        """Delete all snippets from an index."""
        async with self.uow:
            # First get all snippets for this index
            stmt = select(db_entities.Snippet).where(
                db_entities.Snippet.index_id == index_id
            )
            result = await self._session.scalars(stmt)
            snippets = result.all()

            # Delete all embeddings for these snippets
            for snippet in snippets:
                embedding_stmt = delete(db_entities.Embedding).where(
                    db_entities.Embedding.snippet_id == snippet.id
                )
                await self._session.execute(embedding_stmt)

            # Now delete the snippets
            snippet_stmt = delete(db_entities.Snippet).where(
                db_entities.Snippet.index_id == index_id
            )
            await self._session.execute(snippet_stmt)

    async def delete_by_file_ids(self, file_ids: list[int]) -> None:
        """Delete snippets by file IDs."""
        if not file_ids:
            return

        async with self.uow:
            # First get all snippets for these files
            stmt = select(db_entities.Snippet).where(
                db_entities.Snippet.file_id.in_(file_ids)
            )
            result = await self._session.scalars(stmt)
            snippets = result.all()

            # Delete all embeddings for these snippets
            for snippet in snippets:
                embedding_stmt = delete(db_entities.Embedding).where(
                    db_entities.Embedding.snippet_id == snippet.id
                )
                await self._session.execute(embedding_stmt)

            # Now delete the snippets
            snippet_stmt = delete(db_entities.Snippet).where(
                db_entities.Snippet.file_id.in_(file_ids)
            )
            await self._session.execute(snippet_stmt)

    async def get_by_index_id(self, index_id: int) -> list[SnippetWithContext]:
        """Get all snippets for an index."""
        async with self.uow:
            # Query snippets by index ID
            query = select(db_entities.Snippet).where(
                db_entities.Snippet.index_id == index_id
            )

            result = await self._session.scalars(query)
            db_snippets = result.all()

            # Convert to SnippetWithContext
            snippet_contexts = []
            for db_snippet in db_snippets:
                snippet_context = await self._build_snippet_with_context(db_snippet)
                if snippet_context:
                    snippet_contexts.append(snippet_context)

            return snippet_contexts

    async def _build_snippet_with_context(
        self, db_snippet: db_entities.Snippet
    ) -> SnippetWithContext | None:
        """Build a SnippetWithContext from a database snippet."""
        # Get the file for this snippet
        db_file = await self._session.get(db_entities.File, db_snippet.file_id)
        if not db_file:
            return None

        # Get the source for this file
        db_source = await self._session.get(db_entities.Source, db_file.source_id)
        if not db_source:
            return None

        domain_file = await self._mapper.to_domain_file(db_file)
        domain_source = await self._mapper.to_domain_source(db_source)

        return SnippetWithContext(
            source=domain_source,
            file=domain_file,
            authors=domain_file.authors,
            snippet=await self._mapper.to_domain_snippet(
                db_snippet=db_snippet, domain_files=[domain_file]
            ),
        )
