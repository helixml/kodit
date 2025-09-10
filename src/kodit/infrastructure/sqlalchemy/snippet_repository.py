"""SQLAlchemy implementation of SnippetRepository."""

from collections.abc import Callable

from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain import entities as domain_entities
from kodit.domain.entities import SnippetWithContext
from kodit.domain.protocols import SnippetRepository
from kodit.domain.value_objects import MultiSearchRequest, TaskOperation
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
        return SnippetMapper()

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
                db_snippet = self._mapper.from_domain_snippet(domain_snippet, index_id)
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

        # Load authors for this file
        from sqlalchemy import select

        authors_stmt = (
            select(db_entities.Author)
            .join(db_entities.AuthorFileMapping)
            .where(db_entities.AuthorFileMapping.file_id == db_file.id)
        )
        db_authors = list((await self._session.scalars(authors_stmt)).all())

        # Load all files for the source (needed for working copy)
        files_stmt = select(db_entities.File).where(
            db_entities.File.source_id == db_source.id
        )
        db_files = (await self._session.scalars(files_stmt)).all()

        # Convert file to domain (with its authors)
        domain_file = self._mapper.to_domain_file(db_file, db_authors)

        # Convert all files to domain for the source
        domain_files = []
        for db_f in db_files:
            # Load authors for each file
            f_authors_stmt = (
                select(db_entities.Author)
                .join(db_entities.AuthorFileMapping)
                .where(db_entities.AuthorFileMapping.file_id == db_f.id)
            )
            db_f_authors = list((await self._session.scalars(f_authors_stmt)).all())
            domain_f = self._mapper.to_domain_file(db_f, db_f_authors)
            domain_files.append(domain_f)

        # Convert source to domain (with all its files)
        domain_source = self._mapper.to_domain_source(db_source, domain_files)

        # Load processing states for this snippet
        processing_states = []
        if db_snippet.id:
            query = select(db_entities.SnippetProcessingState.processing_step).where(
                db_entities.SnippetProcessingState.snippet_id == db_snippet.id
            )
            result = await self._session.scalars(query)
            state_strings = result.all()

            # Convert to TaskOperation enum values
            for state_str in state_strings:
                try:
                    processing_states.append(TaskOperation(state_str))
                except ValueError:
                    # Skip unknown processing steps for forward compatibility
                    continue

        return SnippetWithContext(
            source=domain_source,
            file=domain_file,
            authors=domain_file.authors,
            snippet=self._mapper.to_domain_snippet(
                db_snippet=db_snippet,
                domain_files=[domain_file],
                processing_states=processing_states
            ),
        )

    async def get_by_file_ids(self, file_ids: list[int]) -> list[SnippetWithContext]:
        """Get snippets by file IDs."""
        if not file_ids:
            return []

        async with self.uow:
            # Query snippets by file IDs
            query = select(db_entities.Snippet).where(
                db_entities.Snippet.file_id.in_(file_ids)
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

    async def get_snippets_needing_processing(
        self, index_id: int, step: TaskOperation
    ) -> list[SnippetWithContext]:
        """Get snippets that need processing for a specific step."""
        async with self.uow:
            # LEFT JOIN to find snippets without processing state for this step
            query = (
                select(db_entities.Snippet)
                .outerjoin(
                    db_entities.SnippetProcessingState,
                    (
                        db_entities.Snippet.id
                        == db_entities.SnippetProcessingState.snippet_id
                    )
                    & (db_entities.SnippetProcessingState.processing_step == str(step)),
                )
                .where(
                    (db_entities.Snippet.index_id == index_id)
                    & (db_entities.SnippetProcessingState.id.is_(None))
                )
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

    async def mark_processing_completed(
        self, snippet_ids: list[int], step: TaskOperation
    ) -> None:
        """Mark processing step as completed for given snippet IDs."""
        if not snippet_ids:
            return

        async with self.uow:
            for snippet_id in snippet_ids:
                # Create or update processing state record
                processing_state = db_entities.SnippetProcessingState(
                    snippet_id=snippet_id, processing_step=str(step)
                )
                self._session.add(processing_state)

    async def reset_processing_states(self, snippet_ids: list[int]) -> None:
        """Reset all processing states for given snippet IDs."""
        if not snippet_ids:
            return

        async with self.uow:
            # Delete all processing states for these snippets
            stmt = delete(db_entities.SnippetProcessingState).where(
                db_entities.SnippetProcessingState.snippet_id.in_(snippet_ids)
            )
            await self._session.execute(stmt)

