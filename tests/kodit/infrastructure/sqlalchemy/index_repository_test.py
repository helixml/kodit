"""Tests for SqlAlchemyIndexRepository."""

from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import AsyncMock, Mock

import pytest
from pydantic import AnyUrl

from kodit.domain import entities as domain_entities
from kodit.domain.value_objects import (
    FileProcessingStatus,
    SourceType,
)
from kodit.infrastructure.sqlalchemy.index_repository import SqlAlchemyIndexRepository
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


@pytest.fixture
def repository(unit_of_work: SqlAlchemyUnitOfWork) -> SqlAlchemyIndexRepository:
    """Create a repository with a unit of work."""
    return SqlAlchemyIndexRepository(unit_of_work)


@pytest.fixture
def sample_author() -> domain_entities.Author:
    """Create a sample author."""
    return domain_entities.Author(id=1, name="John Doe", email="john@example.com")


@pytest.fixture
def sample_file(sample_author: domain_entities.Author) -> domain_entities.File:
    """Create a sample file."""
    return domain_entities.File(
        id=1,
        uri=AnyUrl("file:///test/sample.py"),
        sha256="abc123",
        authors=[sample_author],
        mime_type="text/x-python",
        file_processing_status=FileProcessingStatus.CLEAN,
    )


@pytest.fixture
def sample_working_copy(
    sample_file: domain_entities.File,
) -> domain_entities.WorkingCopy:
    """Create a sample working copy."""
    return domain_entities.WorkingCopy(
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        cloned_path=Path("/test/repo"),
        source_type=SourceType.GIT,
        files=[sample_file],
    )


class TestCreate:
    """Test create() method."""

    async def test_creates_new_index_with_all_entities(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that create() creates index with source, files, and authors."""
        uri = AnyUrl("https://github.com/test/repo.git")

        result = await repository.create(uri, sample_working_copy)

        assert result.id is not None
        assert result.source is not None
        assert result.source.working_copy is not None
        assert len(result.source.working_copy.files) == 1
        assert result.source.working_copy.files[0].id is not None
        assert len(result.source.working_copy.files[0].authors) == 1
        assert result.source.working_copy.files[0].authors[0].id is not None

    async def test_returns_existing_index_when_source_exists(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that create() returns existing index if source already exists."""
        uri = AnyUrl("https://github.com/test/repo.git")

        # Create first index
        first_result = await repository.create(uri, sample_working_copy)

        # Create second index with same URI
        second_result = await repository.create(uri, sample_working_copy)

        assert first_result.id == second_result.id

    async def test_creates_unique_authors_only(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_author: domain_entities.Author,
    ) -> None:
        """Test that create() creates unique authors only."""
        # Create working copy with duplicate authors
        file1 = domain_entities.File(
            uri=AnyUrl("file:///test/file1.py"),
            sha256="abc123",
            authors=[sample_author, sample_author],  # Same author twice
            mime_type="text/x-python",
            file_processing_status=FileProcessingStatus.CLEAN,
        )
        file2 = domain_entities.File(
            uri=AnyUrl("file:///test/file2.py"),
            sha256="def456",
            authors=[sample_author],  # Same author again
            mime_type="text/x-python",
            file_processing_status=FileProcessingStatus.CLEAN,
        )
        working_copy = domain_entities.WorkingCopy(
            remote_uri=AnyUrl("https://github.com/test/repo.git"),
            cloned_path=Path("/test/repo"),
            source_type=SourceType.GIT,
            files=[file1, file2],
        )

        uri = AnyUrl("https://github.com/test/repo.git")
        result = await repository.create(uri, working_copy)

        # Should only create one unique author
        unique_authors = set()
        for file in result.source.working_copy.files:
            for author in file.authors:
                unique_authors.add((author.name, author.email))

        assert len(unique_authors) == 1

    async def test_creates_author_file_mappings(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that create() creates author-file mappings."""
        uri = AnyUrl("https://github.com/test/repo.git")

        result = await repository.create(uri, sample_working_copy)

        # Verify that files have authors with IDs
        file = result.source.working_copy.files[0]
        assert file.id is not None
        assert len(file.authors) == 1
        assert file.authors[0].id is not None

    async def test_commits_transaction(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that create() commits the transaction."""
        uri = AnyUrl("https://github.com/test/repo.git")

        result = await repository.create(uri, sample_working_copy)

        # Verify we can retrieve the index after creation
        retrieved = await repository.get(result.id)
        assert retrieved is not None
        assert retrieved.id == result.id


class TestGet:
    """Test get() method."""

    async def test_returns_index_when_exists(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that get() returns index when it exists."""
        # Create an index first
        uri = AnyUrl("https://github.com/test/repo.git")
        created_index = await repository.create(uri, sample_working_copy)

        result = await repository.get(created_index.id)

        assert result is not None
        assert result.id == created_index.id

    async def test_returns_none_when_not_exists(
        self, repository: SqlAlchemyIndexRepository
    ) -> None:
        """Test that get() returns None when index doesn't exist."""
        result = await repository.get(99999)

        assert result is None

    async def test_uses_unit_of_work_context_manager(self) -> None:
        """Test that get() uses unit of work as context manager."""
        uow_mock = AsyncMock(spec=SqlAlchemyUnitOfWork)
        uow_mock.session = AsyncMock()
        uow_mock.session.get = AsyncMock(return_value=None)
        repository = SqlAlchemyIndexRepository(uow_mock)

        await repository.get(1)

        uow_mock.__aenter__.assert_called_once()
        uow_mock.__aexit__.assert_called_once()


class TestGetByUri:
    """Test get_by_uri() method."""

    async def test_returns_index_when_source_exists(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that get_by_uri() returns index when source exists."""
        uri = AnyUrl("https://github.com/test/repo.git")
        created_index = await repository.create(uri, sample_working_copy)

        result = await repository.get_by_uri(uri)

        assert result is not None
        assert result.id == created_index.id

    async def test_returns_none_when_source_not_exists(
        self, repository: SqlAlchemyIndexRepository
    ) -> None:
        """Test that get_by_uri() returns None when source doesn't exist."""
        uri = AnyUrl("https://github.com/nonexistent/repo.git")

        result = await repository.get_by_uri(uri)

        assert result is None

    async def test_returns_none_when_source_exists_but_no_index(
        self, repository: SqlAlchemyIndexRepository
    ) -> None:
        """Test that get_by_uri() returns None when source exists but no index."""
        # This is a edge case that shouldn't happen in normal operation
        # but we should handle it gracefully
        uri = AnyUrl("https://github.com/test/repo.git")

        result = await repository.get_by_uri(uri)

        assert result is None

    async def test_uses_unit_of_work_context_manager(self) -> None:
        """Test that get_by_uri() uses unit of work as context manager."""
        uow_mock = AsyncMock(spec=SqlAlchemyUnitOfWork)
        uow_mock.session = AsyncMock()
        uow_mock.session.scalar = AsyncMock(return_value=None)
        repository = SqlAlchemyIndexRepository(uow_mock)

        uri = AnyUrl("https://github.com/test/repo.git")
        await repository.get_by_uri(uri)

        uow_mock.__aenter__.assert_called_once()
        uow_mock.__aexit__.assert_called_once()


class TestAll:
    """Test all() method."""

    async def test_returns_empty_list_when_no_indexes(
        self, repository: SqlAlchemyIndexRepository
    ) -> None:
        """Test that all() returns empty list when no indexes exist."""
        result = await repository.all()

        assert result == []

    async def test_returns_all_indexes(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that all() returns all indexes."""
        # Create multiple indexes
        uri1 = AnyUrl("https://github.com/test/repo1.git")
        uri2 = AnyUrl("https://github.com/test/repo2.git")

        index1 = await repository.create(uri1, sample_working_copy)
        index2 = await repository.create(uri2, sample_working_copy)

        result = await repository.all()

        assert len(result) == 2
        index_ids = {index.id for index in result}
        assert index1.id in index_ids
        assert index2.id in index_ids

    async def test_uses_unit_of_work_context_manager(self) -> None:
        """Test that all() uses unit of work as context manager."""
        uow_mock = AsyncMock(spec=SqlAlchemyUnitOfWork)
        uow_mock.session = AsyncMock()
        mock_scalars_result = Mock()
        mock_scalars_result.all.return_value = []
        uow_mock.session.scalars = AsyncMock(return_value=mock_scalars_result)
        repository = SqlAlchemyIndexRepository(uow_mock)

        await repository.all()

        uow_mock.__aenter__.assert_called_once()
        uow_mock.__aexit__.assert_called_once()


class TestUpdateIndexTimestamp:
    """Test update_index_timestamp() method."""

    async def test_updates_timestamp_when_index_exists(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that update_index_timestamp() updates timestamp when index exists."""
        # Create an index
        uri = AnyUrl("https://github.com/test/repo.git")
        created_index = await repository.create(uri, sample_working_copy)
        original_timestamp = created_index.updated_at

        # Update timestamp
        await repository.update_index_timestamp(created_index.id)

        # Verify timestamp was updated
        updated_index = await repository.get(created_index.id)
        assert updated_index is not None
        assert updated_index.updated_at > original_timestamp

    async def test_raises_error_when_index_not_exists(
        self, repository: SqlAlchemyIndexRepository
    ) -> None:
        """Test that update_index_timestamp() raises error when index doesn't exist."""
        with pytest.raises(ValueError, match="Index 99999 not found"):
            await repository.update_index_timestamp(99999)

    async def test_uses_unit_of_work_context_manager(self) -> None:
        """Test that update_index_timestamp() uses unit of work as context manager."""
        uow_mock = AsyncMock(spec=SqlAlchemyUnitOfWork)
        uow_mock.session = AsyncMock()
        uow_mock.session.get = AsyncMock(return_value=None)
        uow_mock.commit = AsyncMock()
        repository = SqlAlchemyIndexRepository(uow_mock)

        with pytest.raises(ValueError, match="Index 1 not found"):
            await repository.update_index_timestamp(1)

        uow_mock.__aenter__.assert_called_once()
        uow_mock.__aexit__.assert_called_once()


class TestUpdate:
    """Test update() method."""

    async def test_raises_error_when_index_not_exists(
        self, repository: SqlAlchemyIndexRepository
    ) -> None:
        """Test that update() raises error when index doesn't exist in database."""
        # Create index with fake ID
        working_copy = domain_entities.WorkingCopy(
            remote_uri=AnyUrl("https://github.com/test/repo.git"),
            cloned_path=Path("/test"),
            source_type=SourceType.GIT,
            files=[],
        )
        source = domain_entities.Source(working_copy=working_copy)
        fake_index = domain_entities.Index(
            id=99999,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source=source,
            snippets=[],
        )

        with pytest.raises(ValueError, match="Index 99999 not found"):
            await repository.update(fake_index)

    async def test_updates_index_timestamp(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that update() updates index timestamp."""
        # Create index
        uri = AnyUrl("https://github.com/test/repo.git")
        created_index = await repository.create(uri, sample_working_copy)

        # Update timestamp and call update
        new_timestamp = datetime.now(UTC)
        created_index.updated_at = new_timestamp
        await repository.update(created_index)

        # Verify timestamp was updated
        updated_index = await repository.get(created_index.id)
        assert updated_index is not None
        assert updated_index.updated_at == new_timestamp

    async def test_updates_source(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that update() updates source information."""
        # Create index
        uri = AnyUrl("https://github.com/test/repo.git")
        created_index = await repository.create(uri, sample_working_copy)

        # Update source working copy
        created_index.source.working_copy.remote_uri = AnyUrl(
            "https://github.com/updated/repo.git"
        )
        created_index.source.working_copy.cloned_path = Path("/updated/path")
        created_index.source.updated_at = datetime.now(UTC)

        await repository.update(created_index)

        # Verify source was updated
        updated_index = await repository.get(created_index.id)
        assert updated_index is not None
        assert "updated/repo" in str(updated_index.source.working_copy.remote_uri)

    async def test_updates_files_and_authors(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that update() updates files and authors."""
        # Create index
        uri = AnyUrl("https://github.com/test/repo.git")
        created_index = await repository.create(uri, sample_working_copy)

        # Add new file to working copy
        new_author = domain_entities.Author(name="Jane Doe", email="jane@example.com")
        new_file = domain_entities.File(
            uri=AnyUrl("file:///test/new_file.py"),
            sha256="new123",
            authors=[new_author],
            mime_type="text/x-python",
            file_processing_status=FileProcessingStatus.ADDED,
        )
        created_index.source.working_copy.files.append(new_file)

        await repository.update(created_index)

        # Verify files and authors were updated
        updated_index = await repository.get(created_index.id)
        assert updated_index is not None
        assert len(updated_index.source.working_copy.files) == 2

    async def test_uses_unit_of_work_context_manager(self) -> None:
        """Test that update() uses unit of work as context manager."""
        uow_mock = AsyncMock(spec=SqlAlchemyUnitOfWork)
        uow_mock.session = AsyncMock()
        uow_mock.session.get = AsyncMock(return_value=None)
        repository = SqlAlchemyIndexRepository(uow_mock)

        working_copy = domain_entities.WorkingCopy(
            remote_uri=AnyUrl("https://github.com/test/repo.git"),
            cloned_path=Path("/test"),
            source_type=SourceType.GIT,
            files=[],
        )
        source = domain_entities.Source(working_copy=working_copy)
        index = domain_entities.Index(
            id=1,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source=source,
            snippets=[],
        )

        with pytest.raises(ValueError, match="Index 1 not found"):
            await repository.update(index)

        uow_mock.__aenter__.assert_called_once()
        uow_mock.__aexit__.assert_called_once()


class TestDelete:
    """Test delete() method."""

    async def test_deletes_all_related_entities(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that delete() deletes all entities related to an index."""
        # Create index
        uri = AnyUrl("https://github.com/test/repo.git")
        created_index = await repository.create(uri, sample_working_copy)

        # Delete index
        await repository.delete(created_index)

        # Verify index no longer exists
        result = await repository.get(created_index.id)
        assert result is None

        # Verify we can't find it by URI either
        result_by_uri = await repository.get_by_uri(uri)
        assert result_by_uri is None

    async def test_deletes_author_file_mappings(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that delete() deletes author-file mappings."""
        # Create index
        uri = AnyUrl("https://github.com/test/repo.git")
        created_index = await repository.create(uri, sample_working_copy)

        # Delete index (should delete author-file mappings)
        await repository.delete(created_index)

        # Verify index is gone
        result = await repository.get(created_index.id)
        assert result is None

    async def test_deletes_files(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that delete() deletes files."""
        # Create index
        uri = AnyUrl("https://github.com/test/repo.git")
        created_index = await repository.create(uri, sample_working_copy)

        # Delete index (should delete files)
        await repository.delete(created_index)

        # Verify index is gone
        result = await repository.get(created_index.id)
        assert result is None

    async def test_deletes_index_and_source(
        self,
        repository: SqlAlchemyIndexRepository,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that delete() deletes index and source."""
        # Create index
        uri = AnyUrl("https://github.com/test/repo.git")
        created_index = await repository.create(uri, sample_working_copy)

        # Delete index (should delete index and source)
        await repository.delete(created_index)

        # Verify index is gone
        result = await repository.get(created_index.id)
        assert result is None

        # Verify source is gone
        result_by_uri = await repository.get_by_uri(uri)
        assert result_by_uri is None
