"""SQLAlchemy implementation of IndexRepository using Index aggregate root."""

from collections.abc import Sequence
from typing import cast

from pydantic import AnyUrl
from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain import entities as domain_entities
from kodit.domain.entities import SnippetWithContext
from kodit.domain.protocols import IndexRepository
from kodit.domain.value_objects import (
    MultiSearchRequest,
)
from kodit.infrastructure.mappers.index_mapper import IndexMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities


class SqlAlchemyIndexRepository(IndexRepository):
    """SQLAlchemy implementation of IndexRepository.

    This repository manages the complete Index aggregate, including:
    - Index entity
    - Source entity and WorkingCopy value object
    - File entities and Author relationships
    - Snippet entities with their contents
    """

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the repository."""
        self._session = session
        self._mapper = IndexMapper(session)

    async def create(
        self, uri: AnyUrl, working_copy: domain_entities.WorkingCopy
    ) -> domain_entities.Index:
        """Create an index with all the files and authors in the working copy."""
        # 1. Verify that a source with this URI does not exist
        existing_source = await self._get_source_by_uri(uri)
        if existing_source:
            # Check if index already exists for this source
            existing_index = await self._get_index_by_source_id(existing_source.id)
            if existing_index:
                return await self._mapper.to_domain_index(existing_index)

        # 2. Create the source
        db_source = db_entities.Source(
            uri=str(uri),
            cloned_path=str(working_copy.cloned_path),
            source_type=db_entities.SourceType(working_copy.source_type.value),
        )
        self._session.add(db_source)
        await self._session.flush()  # Get source ID

        # 3. Create a set of unique authors
        unique_authors = {}
        for domain_file in working_copy.files:
            for author in domain_file.authors:
                key = (author.name, author.email)
                if key not in unique_authors:
                    unique_authors[key] = author

        # 4. Create authors if they don't exist and store their IDs
        author_id_map = {}
        for domain_author in unique_authors.values():
            db_author = await self._find_or_create_author(domain_author)
            author_id_map[(domain_author.name, domain_author.email)] = db_author.id

        # 5. Create files
        for domain_file in working_copy.files:
            db_file = db_entities.File(
                created_at=domain_file.created_at or db_source.created_at,
                updated_at=domain_file.updated_at or db_source.updated_at,
                source_id=db_source.id,
                mime_type=domain_file.mime_type,
                uri=str(domain_file.uri),
                cloned_path=str(domain_file.uri),  # Use URI as cloned path
                sha256=domain_file.sha256,
                size_bytes=0,  # Deprecated
                extension="",  # Deprecated
                file_processing_status=domain_file.file_processing_status.value,
            )
            self._session.add(db_file)
            await self._session.flush()  # Get file ID

            # 6. Create author_file_mappings
            for author in domain_file.authors:
                author_id = author_id_map[(author.name, author.email)]
                mapping = db_entities.AuthorFileMapping(
                    author_id=author_id, file_id=db_file.id
                )
                await self._upsert_author_file_mapping(mapping)

        # 7. Create the index
        db_index = db_entities.Index(source_id=db_source.id)
        self._session.add(db_index)
        await self._session.flush()  # Get index ID

        # 8. Return the new index
        return await self._mapper.to_domain_index(db_index)

    async def get(self, index_id: int) -> domain_entities.Index | None:
        """Get an index by ID."""
        db_index = await self._session.get(db_entities.Index, index_id)
        if not db_index:
            return None

        return await self._mapper.to_domain_index(db_index)

    async def get_by_uri(self, uri: AnyUrl) -> domain_entities.Index | None:
        """Get an index by source URI."""
        db_source = await self._get_source_by_uri(uri)
        if not db_source:
            return None

        db_index = await self._get_index_by_source_id(db_source.id)
        if not db_index:
            return None

        return await self._mapper.to_domain_index(db_index)

    async def all(self) -> list[domain_entities.Index]:
        """List all indexes."""
        stmt = select(db_entities.Index)
        result = await self._session.scalars(stmt)
        db_indexes = result.all()

        domain_indexes = []
        for db_index in db_indexes:
            domain_index = await self._mapper.to_domain_index(db_index)
            domain_indexes.append(domain_index)

        return domain_indexes

    async def update_index_timestamp(self, index_id: int) -> None:
        """Update the timestamp of an index."""
        from datetime import UTC, datetime

        db_index = await self._session.get(db_entities.Index, index_id)
        if db_index:
            db_index.updated_at = datetime.now(UTC)
            # SQLAlchemy will automatically track this change

    async def add_snippets(
        self, index_id: int, snippets: list[domain_entities.Snippet]
    ) -> None:
        """Add snippets to an index.

        The snippets should already contain their file relationships via derives_from.
        """
        if not snippets:
            return

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

    async def update_snippets(
        self, index_id: int, snippets: list[domain_entities.Snippet]
    ) -> None:
        """Update snippets for an index.

        This replaces existing snippets with the provided ones. Snippets should
        already contain their file relationships via derives_from and have IDs.
        """
        if not snippets:
            return

        # Validate the index exists
        db_index = await self._session.get(db_entities.Index, index_id)
        if not db_index:
            raise ValueError(f"Index {index_id} not found")

        # Update each snippet
        for domain_snippet in snippets:
            if not domain_snippet.id:
                raise ValueError("Snippet must have an ID for update")

            # Get the existing snippet
            db_snippet = await self._session.get(db_entities.Snippet, domain_snippet.id)
            if not db_snippet:
                raise ValueError(f"Snippet {domain_snippet.id} not found")

            db_snippet.content = domain_snippet.original_text()
            db_snippet.summary = domain_snippet.summary_text()

            # Update timestamps if provided
            if domain_snippet.updated_at:
                db_snippet.updated_at = domain_snippet.updated_at

    async def search(  # noqa: C901
        self, request: MultiSearchRequest
    ) -> Sequence[SnippetWithContext]:
        """Search snippets with filters.

        This is a basic implementation that performs text search on snippet content.
        In a production environment, this would integrate with specialized search
        services (BM25, vector search, etc.).
        """
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
        result = await self._session.scalars(query)
        db_snippets = result.all()

        # Convert to SnippetWithContext
        snippet_contexts = []
        for db_snippet in db_snippets:
            # Get the file for this snippet
            db_file = await self._session.get(db_entities.File, db_snippet.file_id)
            if not db_file:
                continue

            # Get the source for this file
            db_source = await self._session.get(db_entities.Source, db_file.source_id)
            if not db_source:
                continue

            domain_file = await self._mapper.to_domain_file(db_file)
            snippet_context = SnippetWithContext(
                source=await self._mapper.to_domain_source(db_source),
                file=domain_file,
                authors=domain_file.authors,
                snippet=await self._mapper.to_domain_snippet(
                    db_snippet=db_snippet, domain_files=[domain_file]
                ),
            )
            snippet_contexts.append(snippet_context)

        return snippet_contexts

    async def get_snippets_by_ids(self, ids: list[int]) -> list[SnippetWithContext]:
        """Get snippets by their IDs."""
        if not ids:
            return []

        # Query snippets by IDs
        query = select(db_entities.Snippet).where(db_entities.Snippet.id.in_(ids))

        result = await self._session.scalars(query)
        db_snippets = result.all()

        # Convert to SnippetWithContext using similar logic as search
        snippet_contexts = []
        for db_snippet in db_snippets:
            # Get the file for this snippet
            db_file = await self._session.get(db_entities.File, db_snippet.file_id)
            if not db_file:
                continue

            # Get the source for this file
            db_source = await self._session.get(db_entities.Source, db_file.source_id)
            if not db_source:
                continue

            domain_file = await self._mapper.to_domain_file(db_file)
            snippet_context = SnippetWithContext(
                source=await self._mapper.to_domain_source(db_source),
                file=domain_file,
                authors=domain_file.authors,
                snippet=await self._mapper.to_domain_snippet(
                    db_snippet=db_snippet, domain_files=[domain_file]
                ),
            )
            snippet_contexts.append(snippet_context)

        return snippet_contexts

    async def _get_source_by_uri(self, uri: AnyUrl) -> db_entities.Source | None:
        """Get source by URI."""
        stmt = select(db_entities.Source).where(db_entities.Source.uri == str(uri))
        return cast("db_entities.Source | None", await self._session.scalar(stmt))

    async def _get_index_by_source_id(self, source_id: int) -> db_entities.Index | None:
        """Get index by source ID."""
        stmt = select(db_entities.Index).where(db_entities.Index.source_id == source_id)
        return cast("db_entities.Index | None", await self._session.scalar(stmt))

    async def _find_or_create_author(
        self, domain_author: domain_entities.Author
    ) -> db_entities.Author:
        """Find existing author or create new one."""
        # Try to find existing author
        stmt = select(db_entities.Author).where(
            db_entities.Author.name == domain_author.name,
            db_entities.Author.email == domain_author.email,
        )
        db_author = await self._session.scalar(stmt)

        if db_author:
            return db_author

        # Create new author
        db_author = db_entities.Author(
            name=domain_author.name, email=domain_author.email
        )
        self._session.add(db_author)
        await self._session.flush()  # Get ID

        return db_author

    async def _upsert_author_file_mapping(
        self, mapping: db_entities.AuthorFileMapping
    ) -> db_entities.AuthorFileMapping:
        """Create a new author file mapping or return existing one if already exists."""
        # First check if mapping already exists with same author_id and file_id
        stmt = select(db_entities.AuthorFileMapping).where(
            db_entities.AuthorFileMapping.author_id == mapping.author_id,
            db_entities.AuthorFileMapping.file_id == mapping.file_id,
        )
        existing_mapping = cast(
            "db_entities.AuthorFileMapping | None", await self._session.scalar(stmt)
        )

        if existing_mapping:
            return existing_mapping

        # Mapping doesn't exist, create new one
        self._session.add(mapping)
        return mapping

    async def delete_snippets(self, index_id: int) -> None:
        """Delete all snippets from an index."""
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

    async def delete_snippets_by_file_ids(self, file_ids: list[int]) -> None:
        """Delete snippets by file IDs.

        This is used when files are removed from the working copy to clean up
        orphaned snippets and their associated embeddings.
        """
        if not file_ids:
            return

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

    async def update(self, index: domain_entities.Index) -> None:
        """Update an index by ensuring all domain objects are saved to database."""
        if not index.id:
            raise ValueError("Index must have an ID to be updated")

        # 1. Verify the index exists in the database
        db_index = await self._session.get(db_entities.Index, index.id)
        if not db_index:
            raise ValueError(f"Index {index.id} not found")

        # 2. Update index timestamps
        if index.updated_at:
            db_index.updated_at = index.updated_at

        # 3. Update source if it exists
        await self._update_source(index, db_index)

        # 4. Handle files and authors from working copy
        if index.source and index.source.working_copy:
            await self._update_files_and_authors(index, db_index)

        # 5. Handle snippets
        if index.snippets:
            await self._update_snippets(index)

    async def _update_source(
        self, index: domain_entities.Index, db_index: db_entities.Index
    ) -> None:
        """Update source information."""
        if not index.source:
            return

        db_source = await self._session.get(db_entities.Source, db_index.source_id)
        if db_source and index.source.working_copy:
            db_source.uri = str(index.source.working_copy.remote_uri)
            db_source.cloned_path = str(index.source.working_copy.cloned_path)
            db_source.type = db_entities.SourceType(
                index.source.working_copy.source_type.value
            )
            if index.source.updated_at:
                db_source.updated_at = index.source.updated_at

    async def _update_files_and_authors(
        self, index: domain_entities.Index, db_index: db_entities.Index
    ) -> None:
        """Update files and authors."""
        if not index.source or not index.source.working_copy:
            return

        # Create a set of unique authors
        unique_authors = {}
        for domain_file in index.source.working_copy.files:
            for author in domain_file.authors:
                key = (author.name, author.email)
                if key not in unique_authors:
                    unique_authors[key] = author

        # Find or create authors and store their IDs
        author_id_map = {}
        for domain_author in unique_authors.values():
            db_author = await self._find_or_create_author(domain_author)
            author_id_map[(domain_author.name, domain_author.email)] = db_author.id

        # Update or create files and synchronize domain objects with database IDs
        for domain_file in index.source.working_copy.files:
            file_id = await self._update_or_create_file(domain_file, db_index)
            # CRITICAL: Update domain file with database ID for snippet creation
            if not domain_file.id:
                domain_file.id = file_id
            await self._update_author_file_mappings(domain_file, file_id, author_id_map)

    async def _update_or_create_file(
        self,
        domain_file: domain_entities.File,
        db_index: db_entities.Index,
    ) -> int:
        """Update or create a file and return its ID."""
        # Try to find existing file by URI and source_id
        file_stmt = select(db_entities.File).where(
            db_entities.File.uri == str(domain_file.uri),
            db_entities.File.source_id == db_index.source_id,
        )
        existing_file = await self._session.scalar(file_stmt)

        if existing_file:
            # Update existing file
            if domain_file.created_at:
                existing_file.created_at = domain_file.created_at
            if domain_file.updated_at:
                existing_file.updated_at = domain_file.updated_at
            existing_file.mime_type = domain_file.mime_type
            existing_file.sha256 = domain_file.sha256
            existing_file.file_processing_status = (
                domain_file.file_processing_status.value
            )
            return existing_file.id
        # Create new file
        db_file = db_entities.File(
            created_at=domain_file.created_at or db_index.created_at,
            updated_at=domain_file.updated_at or db_index.updated_at,
            source_id=db_index.source_id,
            mime_type=domain_file.mime_type,
            uri=str(domain_file.uri),
            cloned_path=str(domain_file.uri),
            sha256=domain_file.sha256,
            size_bytes=0,  # Deprecated
            extension="",  # Deprecated
            file_processing_status=domain_file.file_processing_status.value,
        )
        self._session.add(db_file)
        await self._session.flush()
        return db_file.id

    async def _update_author_file_mappings(
        self,
        domain_file: domain_entities.File,
        file_id: int,
        author_id_map: dict[tuple[str, str], int],
    ) -> None:
        """Update author-file mappings for a file."""
        for author in domain_file.authors:
            author_id = author_id_map[(author.name, author.email)]
            mapping = db_entities.AuthorFileMapping(
                author_id=author_id, file_id=file_id
            )
            await self._upsert_author_file_mapping(mapping)

    async def _update_snippets(self, index: domain_entities.Index) -> None:
        """Update snippets for the index."""
        if not index.snippets:
            return

        for domain_snippet in index.snippets:
            if domain_snippet.id:
                # Update existing snippet
                db_snippet = await self._session.get(
                    db_entities.Snippet, domain_snippet.id
                )
                if db_snippet:
                    db_snippet.content = domain_snippet.original_text()
                    db_snippet.summary = domain_snippet.summary_text()
                    if domain_snippet.updated_at:
                        db_snippet.updated_at = domain_snippet.updated_at
            else:
                # Create new snippet
                db_snippet = await self._mapper.from_domain_snippet(
                    domain_snippet, index.id
                )
                self._session.add(db_snippet)

    async def delete(self, index: domain_entities.Index) -> None:
        """Delete everything related to an index."""
        # Delete all snippets and embeddings
        await self.delete_snippets(index.id)

        # Delete all author file mappings
        stmt = delete(db_entities.AuthorFileMapping).where(
            db_entities.AuthorFileMapping.file_id.in_(
                [file.id for file in index.source.working_copy.files]
            )
        )
        await self._session.execute(stmt)

        # Delete all files
        stmt = delete(db_entities.File).where(
            db_entities.File.source_id == index.source.id
        )
        await self._session.execute(stmt)

        # Delete the index
        stmt = delete(db_entities.Index).where(db_entities.Index.id == index.id)
        await self._session.execute(stmt)

        # Delete the source
        stmt = delete(db_entities.Source).where(
            db_entities.Source.id == index.source.id
        )
        await self._session.execute(stmt)
