"""Mapping between domain Index aggregate and SQLAlchemy entities."""

from datetime import UTC, datetime
from pathlib import Path

from pydantic import AnyUrl
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

import kodit.domain.entities as domain_entities
from kodit.domain.value_objects import (
    FileProcessingStatus,
    SourceType,
)
from kodit.infrastructure.sqlalchemy import entities as db_entities


class IndexMapper:
    """Mapper for converting between domain Index aggregate and database entities."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize mapper with database session."""
        self._session = session

    async def to_domain_index(
        self, db_index: db_entities.Index
    ) -> domain_entities.Index:
        """Convert SQLAlchemy Index to domain Index aggregate.

        Loads the full aggregate including Source, WorkingCopy, Files, and Snippets.
        """
        # Load the source
        db_source = await self._session.get(db_entities.Source, db_index.source_id)
        if not db_source:
            raise ValueError(f"Source not found for index {db_index.id}")

        # Load files for the source
        files_stmt = select(db_entities.File).where(
            db_entities.File.source_id == db_source.id
        )
        db_files = (await self._session.scalars(files_stmt)).all()

        # Convert files to domain
        domain_files = []
        for db_file in db_files:
            # Load authors for this file
            authors_stmt = (
                select(db_entities.Author)
                .join(db_entities.AuthorFileMapping)
                .where(db_entities.AuthorFileMapping.file_id == db_file.id)
            )
            db_authors = (await self._session.scalars(authors_stmt)).all()

            domain_authors = [
                domain_entities.Author(
                    id=author.id, name=author.name, email=author.email
                )
                for author in db_authors
            ]

            domain_file = domain_entities.File(
                id=db_file.id,
                created_at=db_file.created_at,
                updated_at=db_file.updated_at,
                uri=AnyUrl(db_file.uri),
                sha256=db_file.sha256,
                authors=domain_authors,
                mime_type=db_file.mime_type,
                file_processing_status=FileProcessingStatus(
                    db_file.file_processing_status
                ),
            )
            domain_files.append(domain_file)

        # Create working copy
        working_copy = domain_entities.WorkingCopy(
            created_at=db_source.created_at,
            updated_at=db_source.updated_at,
            remote_uri=AnyUrl(db_source.uri),
            cloned_path=Path(db_source.cloned_path),
            source_type=SourceType(db_source.type.value),
            files=domain_files,
        )

        # Create source
        domain_source = domain_entities.Source(
            id=db_source.id,
            created_at=db_source.created_at,
            updated_at=db_source.updated_at,
            working_copy=working_copy,
        )

        # Load snippets for this index
        snippets_stmt = select(db_entities.Snippet).where(
            db_entities.Snippet.index_id == db_index.id
        )
        db_snippets = (await self._session.scalars(snippets_stmt)).all()

        domain_snippets = []
        for db_snippet in db_snippets:
            domain_snippet = await self.to_domain_snippet(db_snippet, domain_files)
            domain_snippets.append(domain_snippet)

        # Create index aggregate
        return domain_entities.Index(
            id=db_index.id,
            created_at=db_index.created_at,
            updated_at=db_index.updated_at,
            source=domain_source,
            snippets=domain_snippets,
        )

    async def to_domain_source(
        self, db_source: db_entities.Source
    ) -> domain_entities.Source:
        """Convert SQLAlchemy Source to domain Source."""
        # Load files for the source
        files_stmt = select(db_entities.File).where(
            db_entities.File.source_id == db_source.id
        )
        db_files = (await self._session.scalars(files_stmt)).all()

        # Convert files to domain
        domain_files = []
        for db_file in db_files:
            # Load authors for this file
            authors_stmt = (
                select(db_entities.Author)
                .join(db_entities.AuthorFileMapping)
                .where(db_entities.AuthorFileMapping.file_id == db_file.id)
            )
            db_authors = (await self._session.scalars(authors_stmt)).all()

            domain_authors = [
                domain_entities.Author(
                    id=author.id, name=author.name, email=author.email
                )
                for author in db_authors
            ]

            domain_file = domain_entities.File(
                id=db_file.id,
                created_at=db_file.created_at,
                updated_at=db_file.updated_at,
                uri=AnyUrl(db_file.uri),
                sha256=db_file.sha256,
                authors=domain_authors,
                mime_type=db_file.mime_type,
                file_processing_status=FileProcessingStatus(
                    db_file.file_processing_status
                ),
            )
            domain_files.append(domain_file)

        # Create working copy
        working_copy = domain_entities.WorkingCopy(
            created_at=db_source.created_at,
            updated_at=db_source.updated_at,
            remote_uri=AnyUrl(db_source.uri),
            cloned_path=Path(db_source.cloned_path),
            source_type=SourceType(db_source.type.value),
            files=domain_files,
        )

        # Create source
        return domain_entities.Source(
            id=db_source.id,
            created_at=db_source.created_at,
            updated_at=db_source.updated_at,
            working_copy=working_copy,
        )

    async def to_domain_file(self, db_file: db_entities.File) -> domain_entities.File:
        """Convert SQLAlchemy File to domain File."""
        # Load authors for this file
        authors_stmt = (
            select(db_entities.Author)
            .join(db_entities.AuthorFileMapping)
            .where(db_entities.AuthorFileMapping.file_id == db_file.id)
        )
        db_authors = (await self._session.scalars(authors_stmt)).all()

        domain_authors = [
            domain_entities.Author(id=author.id, name=author.name, email=author.email)
            for author in db_authors
        ]

        return domain_entities.File(
            id=db_file.id,
            created_at=db_file.created_at,
            updated_at=db_file.updated_at,
            uri=AnyUrl(db_file.uri),
            sha256=db_file.sha256,
            authors=domain_authors,
            mime_type=db_file.mime_type,
            file_processing_status=FileProcessingStatus(db_file.file_processing_status),
        )

    async def to_domain_snippet(
        self, db_snippet: db_entities.Snippet, domain_files: list[domain_entities.File]
    ) -> domain_entities.Snippet:
        """Convert SQLAlchemy Snippet to domain Snippet."""
        # Find the file this snippet derives from
        derives_from = []
        for domain_file in domain_files:
            if domain_file.id == db_snippet.file_id:
                derives_from.append(domain_file)
                break

        # Create domain snippet with original content
        domain_snippet = domain_entities.Snippet(
            id=db_snippet.id,
            created_at=db_snippet.created_at,
            updated_at=db_snippet.updated_at,
            derives_from=derives_from,
        )

        # Add original content
        if db_snippet.content:
            domain_snippet.add_original_content(db_snippet.content, "unknown")

        # Add summary content if it exists
        if db_snippet.summary:
            domain_snippet.add_summary(db_snippet.summary)

        return domain_snippet

    async def from_domain_index(  # noqa: C901
        self, domain_index: domain_entities.Index
    ) -> tuple[
        db_entities.Index,
        db_entities.Source,
        list[db_entities.File],
        list[db_entities.Author],
    ]:
        """Convert domain Index aggregate to SQLAlchemy entities.

        Returns all the entities that need to be persisted.
        """
        # Create source entity
        db_source = db_entities.Source(
            uri=str(domain_index.source.working_copy.remote_uri),
            cloned_path=str(domain_index.source.working_copy.cloned_path),
            source_type=db_entities.SourceType(
                domain_index.source.working_copy.source_type.value
            ),
        )
        if domain_index.source.id:
            db_source.id = domain_index.source.id
        if domain_index.source.created_at:
            db_source.created_at = domain_index.source.created_at
        if domain_index.source.updated_at:
            db_source.updated_at = domain_index.source.updated_at

        # Create index entity
        # Will be set after source is saved
        db_index = db_entities.Index(source_id=db_source.id or 0)
        if domain_index.id:
            db_index.id = domain_index.id
        if domain_index.created_at:
            db_index.created_at = domain_index.created_at
        if domain_index.updated_at:
            db_index.updated_at = domain_index.updated_at

        # Create file entities
        db_files = []
        all_authors = []

        for domain_file in domain_index.source.working_copy.files:
            now = datetime.now(UTC)
            db_file = db_entities.File(
                created_at=domain_file.created_at or now,
                updated_at=domain_file.updated_at or now,
                source_id=db_source.id or 0,  # Will be set after source is saved
                mime_type="",  # Would need to be determined
                uri=str(domain_file.uri),
                # Would need to be determined from working copy + relative path
                cloned_path="",
                sha256=domain_file.sha256,
                size_bytes=0,  # Would need to be determined
                extension="",  # Would need to be determined
                file_processing_status=domain_file.file_processing_status.value,
            )
            if domain_file.id:
                db_file.id = domain_file.id

            db_files.append(db_file)
            all_authors.extend(domain_file.authors)

        # Create unique author entities
        unique_authors = {}
        for author in all_authors:
            key = (author.name, author.email)
            if key not in unique_authors:
                db_author = db_entities.Author(name=author.name, email=author.email)
                if author.id:
                    db_author.id = author.id
                unique_authors[key] = db_author

        return db_index, db_source, db_files, list(unique_authors.values())

    async def from_domain_snippet(
        self, domain_snippet: domain_entities.Snippet, index_id: int
    ) -> db_entities.Snippet:
        """Convert domain Snippet to SQLAlchemy Snippet."""
        # Get file ID from derives_from (use first file if multiple)
        if not domain_snippet.derives_from:
            raise ValueError("Snippet must derive from at least one file")

        file_id = domain_snippet.derives_from[0].id
        if file_id is None:
            raise ValueError("File must have an ID")

        db_snippet = db_entities.Snippet(
            file_id=file_id,
            index_id=index_id,
            content=domain_snippet.original_text(),
            summary=domain_snippet.summary_text(),
        )

        if domain_snippet.id:
            db_snippet.id = domain_snippet.id
        if domain_snippet.created_at:
            db_snippet.created_at = domain_snippet.created_at
        if domain_snippet.updated_at:
            db_snippet.updated_at = domain_snippet.updated_at

        return db_snippet

    async def load_snippets_for_index(
        self, index_id: int, domain_files: list[domain_entities.File]
    ) -> list[domain_entities.Snippet]:
        """Load all snippets for an index and convert to domain entities."""
        stmt = select(db_entities.Snippet).where(
            db_entities.Snippet.index_id == index_id
        )
        db_snippets = (await self._session.scalars(stmt)).all()

        domain_snippets = []
        for db_snippet in db_snippets:
            domain_snippet = await self.to_domain_snippet(db_snippet, domain_files)
            domain_snippets.append(domain_snippet)

        return domain_snippets
