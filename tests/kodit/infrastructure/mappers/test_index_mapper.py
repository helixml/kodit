"""Tests for the IndexMapper."""

from datetime import UTC, datetime
from pathlib import Path

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

import kodit.domain.entities as domain_entities
from kodit.domain.value_objects import FileProcessingStatus, SourceType
from kodit.infrastructure.mappers.index_mapper import IndexMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities


class TestIndexMapper:
    """Test the IndexMapper."""

    @pytest.mark.asyncio
    async def test_to_domain_file(self, session: AsyncSession) -> None:
        """Test converting a database File to domain File."""
        # Create test data
        db_source = db_entities.Source(
            uri="file:///test/repo",
            cloned_path="/test/repo",
            source_type=db_entities.SourceType.FOLDER,
        )
        session.add(db_source)
        await session.flush()

        db_author = db_entities.Author(name="Test Author", email="test@example.com")
        session.add(db_author)
        await session.flush()

        db_file = db_entities.File(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source_id=db_source.id,
            mime_type="text/plain",
            uri="file:///test/file.txt",
            cloned_path="/test/file.txt",
            sha256="abc123",
            size_bytes=100,
            extension="txt",
            file_processing_status=FileProcessingStatus.CLEAN.value,
        )
        session.add(db_file)
        await session.flush()

        # Create author-file mapping
        mapping = db_entities.AuthorFileMapping(
            author_id=db_author.id, file_id=db_file.id
        )
        session.add(mapping)
        await session.flush()

        # Test mapping
        mapper = IndexMapper(session)
        domain_file = await mapper.to_domain_file(db_file)

        # Verify mapping
        assert domain_file.id == db_file.id
        assert domain_file.created_at == db_file.created_at
        assert domain_file.updated_at == db_file.updated_at
        assert str(domain_file.uri) == db_file.uri
        assert domain_file.sha256 == db_file.sha256
        assert domain_file.mime_type == db_file.mime_type
        assert domain_file.file_processing_status == FileProcessingStatus.CLEAN
        assert len(domain_file.authors) == 1
        assert domain_file.authors[0].name == "Test Author"
        assert domain_file.authors[0].email == "test@example.com"

    @pytest.mark.asyncio
    async def test_from_domain_snippet(self, session: AsyncSession) -> None:
        """Test converting a domain Snippet to database Snippet."""
        # Create test data
        db_source = db_entities.Source(
            uri="file:///test/repo",
            cloned_path="/test/repo",
            source_type=db_entities.SourceType.FOLDER,
        )
        session.add(db_source)
        await session.flush()

        db_file = db_entities.File(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source_id=db_source.id,
            mime_type="text/plain",
            uri="file:///test/file.txt",
            cloned_path="/test/file.txt",
            sha256="abc123",
            size_bytes=100,
            extension="txt",
            file_processing_status=FileProcessingStatus.CLEAN.value,
        )
        session.add(db_file)
        await session.flush()

        # Create domain snippet
        domain_file = domain_entities.File(
            id=db_file.id,
            created_at=db_file.created_at,
            updated_at=db_file.updated_at,
            uri=AnyUrl(db_file.uri),
            sha256=db_file.sha256,
            authors=[],
            mime_type=db_file.mime_type,
            file_processing_status=FileProcessingStatus.CLEAN,
        )

        domain_snippet = domain_entities.Snippet(
            id=1,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            derives_from=[domain_file],
        )
        domain_snippet.add_original_content("def test(): pass", "python")
        domain_snippet.add_summary("Test function")

        # Test mapping
        mapper = IndexMapper(session)
        db_snippet = await mapper.from_domain_snippet(domain_snippet, index_id=1)

        # Verify mapping
        assert db_snippet.id == domain_snippet.id
        assert db_snippet.created_at == domain_snippet.created_at
        assert db_snippet.updated_at == domain_snippet.updated_at
        assert db_snippet.file_id == domain_file.id
        assert db_snippet.index_id == 1
        assert db_snippet.content == "def test(): pass"
        assert db_snippet.summary == "Test function"

    @pytest.mark.asyncio
    async def test_from_domain_index(self, session: AsyncSession) -> None:
        """Test converting a domain Index to database entities."""
        # Create domain entities
        author = domain_entities.Author(name="Test Author", email="test@example.com")
        file = domain_entities.File(
            uri=AnyUrl("file:///test/file.txt"),
            sha256="abc123",
            authors=[author],
            mime_type="text/plain",
            file_processing_status=FileProcessingStatus.CLEAN,
        )
        working_copy = domain_entities.WorkingCopy(
            remote_uri=AnyUrl("file:///test/repo"),
            cloned_path=Path("/test/repo"),
            source_type=SourceType.FOLDER,
            files=[file],
        )
        source = domain_entities.Source(working_copy=working_copy)
        index = domain_entities.Index(
            id=1,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source=source,
            snippets=[],
        )

        # Test mapping
        mapper = IndexMapper(session)
        db_index, db_source, db_files, db_authors = await mapper.from_domain_index(
            index
        )

        # Verify mapping
        assert db_index.id == index.id
        assert db_index.created_at == index.created_at
        assert db_index.updated_at == index.updated_at

        assert db_source.uri == str(index.source.working_copy.remote_uri)
        assert db_source.cloned_path == str(index.source.working_copy.cloned_path)
        assert db_source.type.value == index.source.working_copy.source_type.value

        assert len(db_files) == 1
        assert db_files[0].uri == str(file.uri)
        assert db_files[0].sha256 == file.sha256
        # Note: mime_type is not set in from_domain_index (hardcoded to empty string)
        assert db_files[0].mime_type == ""

        assert len(db_authors) == 1
        assert db_authors[0].name == author.name
        assert db_authors[0].email == author.email
