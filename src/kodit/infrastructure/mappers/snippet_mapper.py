"""Mapping between domain Snippet entities and SQLAlchemy entities."""

from pathlib import Path

from pydantic import AnyUrl

import kodit.domain.entities as domain_entities
from kodit.domain.value_objects import FileProcessingStatus, SourceType, TaskOperation
from kodit.infrastructure.sqlalchemy import entities as db_entities


class SnippetMapper:
    """Mapper for converting between domain Snippet entities and database entities."""

    def to_domain_snippet(
        self,
        db_snippet: db_entities.Snippet,
        domain_files: list[domain_entities.File],
        processing_states: list[TaskOperation],
    ) -> domain_entities.Snippet:
        """Convert SQLAlchemy Snippet to domain Snippet."""
        # Find the file this snippet derives from
        derives_from = []
        for domain_file in domain_files:
            if domain_file.id == db_snippet.file_id:
                derives_from.append(domain_file)
                break

        # Create domain snippet with content using factory method
        if not db_snippet.content:
            raise ValueError("Database snippet must have content")

        domain_snippet = domain_entities.Snippet.create_with_content(
            derives_from=derives_from,
            content=db_snippet.content,
            language="unknown",
        )

        # Set database-loaded fields
        domain_snippet.id = db_snippet.id
        domain_snippet.created_at = db_snippet.created_at
        domain_snippet.updated_at = db_snippet.updated_at
        # Use the hash from database if it exists, otherwise keep the calculated one
        if db_snippet.content_hash:
            domain_snippet.content_hash = db_snippet.content_hash

        # Add summary content if it exists
        if db_snippet.summary:
            domain_snippet.add_summary(db_snippet.summary)

        for step in processing_states:
            domain_snippet.mark_processing_completed(step)

        return domain_snippet

    def from_domain_snippet(
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
            content_hash=domain_snippet.content_hash,
        )

        if domain_snippet.id:
            db_snippet.id = domain_snippet.id
        if domain_snippet.created_at:
            db_snippet.created_at = domain_snippet.created_at
        if domain_snippet.updated_at:
            db_snippet.updated_at = domain_snippet.updated_at

        return db_snippet

    def to_domain_file(
        self, db_file: db_entities.File, db_authors: list[db_entities.Author]
    ) -> domain_entities.File:
        """Convert SQLAlchemy File to domain File."""
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

    def to_domain_source(
        self, db_source: db_entities.Source, domain_files: list[domain_entities.File]
    ) -> domain_entities.Source:
        """Convert SQLAlchemy Source to domain Source."""
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
