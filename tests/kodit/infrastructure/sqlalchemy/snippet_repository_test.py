"""Tests for SnippetRepository implementation."""

from collections.abc import Callable
from datetime import UTC, datetime

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

import kodit.domain.entities as domain_entities
from kodit.domain.value_objects import (
    FileProcessingStatus,
    MultiSearchRequest,
    SnippetSearchFilters,
    SourceType,
)
from kodit.infrastructure.sqlalchemy.index_repository import (
    SqlAlchemyIndexRepository,
    create_index_repository,
)
from kodit.infrastructure.sqlalchemy.snippet_repository import (
    SqlAlchemySnippetRepository,
    create_snippet_repository,
)
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


@pytest.fixture
def snippet_repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemySnippetRepository:
    """Create a snippet repository for testing."""
    return create_snippet_repository(session_factory=session_factory)  # type: ignore[return-value]


@pytest.fixture
def index_repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemyIndexRepository:
    """Create an index repository for testing (needed for test setup)."""
    return create_index_repository(session_factory=session_factory)  # type: ignore[return-value]


@pytest.fixture
def sample_author() -> domain_entities.Author:
    """Create a sample author."""
    return domain_entities.Author(name="John Doe", email="john@example.com")


@pytest.fixture
def sample_file(sample_author: domain_entities.Author) -> domain_entities.File:
    """Create a sample file."""
    return domain_entities.File(
        id=1,
        uri=AnyUrl("file:///path/to/sample.py"),
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
    from pathlib import Path

    return domain_entities.WorkingCopy(
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        cloned_path=Path("/tmp/test-repo"),
        source_type=SourceType.GIT,
        files=[sample_file],
    )


@pytest.fixture
def sample_snippet(sample_file: domain_entities.File) -> domain_entities.Snippet:
    """Create a sample snippet."""
    snippet = domain_entities.Snippet(id=1, derives_from=[sample_file])
    snippet.add_original_content("def hello():\n    pass", "python")
    snippet.add_summary("A simple hello function")
    return snippet


@pytest.fixture
async def test_index(
    index_repository: SqlAlchemyIndexRepository,
    sample_working_copy: domain_entities.WorkingCopy,
) -> domain_entities.Index:
    """Create a test index for snippet tests."""
    uri = AnyUrl("https://github.com/test/repo.git")
    return await index_repository.create(uri, sample_working_copy)


class TestSnippetRepositoryAdd:
    """Test add() method."""

    async def test_adds_snippets_when_index_exists(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that add() adds snippets when index exists."""
        # Add snippets
        await snippet_repository.add([sample_snippet], test_index.id)

        # Verify snippets were added by getting them back
        snippets = await snippet_repository.get_by_index_id(test_index.id)
        assert len(snippets) == 1
        assert snippets[0].snippet.original_text() == "def hello():\n    pass"
        assert snippets[0].snippet.summary_text() == "A simple hello function"

    async def test_raises_error_when_index_not_exists(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that add() raises error when index doesn't exist."""
        with pytest.raises(ValueError, match="Index 99999 not found"):
            await snippet_repository.add([sample_snippet], 99999)

    async def test_does_nothing_when_no_snippets(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
    ) -> None:
        """Test that add() does nothing when no snippets provided."""
        # Add empty snippets list - should not raise error
        await snippet_repository.add([], test_index.id)

        # Verify no snippets were added
        snippets = await snippet_repository.get_by_index_id(test_index.id)
        assert len(snippets) == 0

    async def test_commits_transaction(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that add() commits transaction."""
        # Add snippets
        await snippet_repository.add([sample_snippet], test_index.id)

        # Verify snippets persisted after commit
        snippets = await snippet_repository.get_by_index_id(test_index.id)
        assert len(snippets) == 1


class TestSnippetRepositoryUpdate:
    """Test update() method."""

    async def test_updates_snippets_when_they_exist(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that update() updates snippets when they exist."""
        # Add snippet first
        await snippet_repository.add([sample_snippet], test_index.id)

        # Get the snippet with its assigned ID
        added_snippets = await snippet_repository.get_by_index_id(test_index.id)
        snippet_with_id = added_snippets[0].snippet

        # Update snippet content
        snippet_with_id.add_original_content(
            "def updated():\n    return True", "python"
        )
        snippet_with_id.add_summary("An updated function")

        # Update snippets
        await snippet_repository.update([snippet_with_id])

        # Verify snippets were updated
        updated_snippets = await snippet_repository.get_by_index_id(test_index.id)
        assert len(updated_snippets) == 1
        assert "def updated():" in updated_snippets[0].snippet.original_text()
        assert "An updated function" in updated_snippets[0].snippet.summary_text()

    async def test_raises_error_when_snippet_no_id(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
    ) -> None:
        """Test that update() raises error when snippet has no ID."""
        # Create snippet without ID
        snippet_without_id = domain_entities.Snippet(derives_from=[])

        with pytest.raises(ValueError, match="Snippet must have an ID for update"):
            await snippet_repository.update([snippet_without_id])

    async def test_raises_error_when_snippet_not_exists(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
    ) -> None:
        """Test that update() raises error when snippet doesn't exist."""
        # Create snippet with non-existent ID
        snippet_with_fake_id = domain_entities.Snippet(id=99999, derives_from=[])

        with pytest.raises(ValueError, match="Snippet 99999 not found"):
            await snippet_repository.update([snippet_with_fake_id])

    async def test_does_nothing_when_no_snippets(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
    ) -> None:
        """Test that update() does nothing when no snippets provided."""
        # Should not raise error
        await snippet_repository.update([])


class TestSnippetRepositoryGetByIds:
    """Test get_by_ids() method."""

    async def test_returns_empty_when_no_ids(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
    ) -> None:
        """Test that get_by_ids() returns empty list when no IDs provided."""
        result = await snippet_repository.get_by_ids([])
        assert result == []

    async def test_returns_snippets_by_ids(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that get_by_ids() returns snippets by their IDs."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Get snippet ID
        added_snippets = await snippet_repository.get_by_index_id(test_index.id)
        snippet_id = added_snippets[0].snippet.id
        assert snippet_id is not None

        # Get snippets by IDs
        result = await snippet_repository.get_by_ids([snippet_id])

        assert len(result) == 1
        assert result[0].snippet.id == snippet_id

    async def test_ignores_non_existent_ids(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that get_by_ids() ignores non-existent IDs."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Get snippets by mix of existing and non-existing IDs
        added_snippets = await snippet_repository.get_by_index_id(test_index.id)
        snippet_id = added_snippets[0].snippet.id
        assert snippet_id is not None

        result = await snippet_repository.get_by_ids([snippet_id, 99999])

        assert len(result) == 1
        assert result[0].snippet.id == snippet_id


class TestSnippetRepositorySearch:
    """Test search() method."""

    async def test_returns_empty_when_no_snippets(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
    ) -> None:
        """Test that search() returns empty list when no snippets exist."""
        request = MultiSearchRequest(text_query="test")

        result = await snippet_repository.search(request)
        assert result == []

    async def test_searches_by_text_query(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that search() filters by text query."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Search for content that exists
        request = MultiSearchRequest(text_query="hello")
        result = await snippet_repository.search(request)

        assert len(result) == 1
        assert "hello" in result[0].snippet.original_text()

    async def test_searches_by_code_query(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that search() filters by code query."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Search for code that exists
        request = MultiSearchRequest(code_query="def")
        result = await snippet_repository.search(request)

        assert len(result) == 1
        assert "def" in result[0].snippet.original_text()

    async def test_searches_by_keywords(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that search() filters by keywords."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Search for keywords that exist
        request = MultiSearchRequest(keywords=["hello", "pass"])
        result = await snippet_repository.search(request)

        assert len(result) == 1

    async def test_searches_by_source_repo_filter(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that search() filters by source repo."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Search with matching source filter
        filters = SnippetSearchFilters(source_repo="test/repo")
        request = MultiSearchRequest(filters=filters)
        result = await snippet_repository.search(request)

        assert len(result) == 1

    async def test_searches_by_file_path_filter(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that search() filters by file path."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Search with matching file path filter
        filters = SnippetSearchFilters(file_path="sample.py")
        request = MultiSearchRequest(filters=filters)
        result = await snippet_repository.search(request)

        assert len(result) == 1

    async def test_searches_by_created_after_filter(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that search() filters by created_after date."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Search with created_after filter (yesterday)
        yesterday = datetime.now(UTC).replace(hour=0, minute=0, second=0, microsecond=0)
        filters = SnippetSearchFilters(created_after=yesterday)
        request = MultiSearchRequest(filters=filters)
        result = await snippet_repository.search(request)

        assert len(result) == 1

    async def test_searches_by_created_before_filter(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that search() filters by created_before date."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Search with created_before filter (tomorrow)
        tomorrow = datetime.now(UTC).replace(
            hour=23, minute=59, second=59, microsecond=0
        )
        filters = SnippetSearchFilters(created_before=tomorrow)
        request = MultiSearchRequest(filters=filters)
        result = await snippet_repository.search(request)

        assert len(result) == 1

    async def test_applies_top_k_limit(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that search() applies top_k limit."""
        # Create multiple snippets
        snippets = []
        for i in range(5):
            snippet = domain_entities.Snippet(derives_from=sample_working_copy.files)
            snippet.add_original_content(f"def function_{i}():\n    pass", "python")
            snippets.append(snippet)

        await snippet_repository.add(snippets, test_index.id)

        # Search with limit of 3
        request = MultiSearchRequest(text_query="def", top_k=3)
        result = await snippet_repository.search(request)

        assert len(result) == 3


class TestSnippetRepositoryDelete:
    """Test delete methods."""

    async def test_delete_by_index_id(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that delete_by_index_id() deletes all snippets for an index."""
        # Add snippets
        await snippet_repository.add([sample_snippet], test_index.id)

        # Verify snippet exists
        snippets_before = await snippet_repository.get_by_index_id(test_index.id)
        assert len(snippets_before) == 1

        # Delete snippets
        await snippet_repository.delete_by_index_id(test_index.id)

        # Verify snippets are deleted
        snippets_after = await snippet_repository.get_by_index_id(test_index.id)
        assert len(snippets_after) == 0

    async def test_delete_by_file_ids(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that delete_by_file_ids() deletes snippets for given files."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Get file ID
        snippets_before = await snippet_repository.get_by_index_id(test_index.id)
        file_id = snippets_before[0].file.id
        assert file_id is not None

        # Delete snippets by file ID
        await snippet_repository.delete_by_file_ids([file_id])

        # Verify snippets are deleted
        snippets_after = await snippet_repository.get_by_index_id(test_index.id)
        assert len(snippets_after) == 0

    async def test_delete_by_file_ids_does_nothing_when_no_files(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
    ) -> None:
        """Test that delete_by_file_ids() does nothing when no file IDs provided."""
        # Should not raise error
        await snippet_repository.delete_by_file_ids([])


class TestSnippetRepositoryGetByIndexId:
    """Test get_by_index_id() method."""

    async def test_returns_all_snippets_for_index(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
        sample_working_copy: domain_entities.WorkingCopy,
    ) -> None:
        """Test that get_by_index_id() returns all snippets for an index."""
        # Add multiple snippets
        snippet1 = sample_snippet
        snippet2 = domain_entities.Snippet(derives_from=sample_working_copy.files)
        snippet2.add_original_content("def another():\n    return 42", "python")

        await snippet_repository.add([snippet1, snippet2], test_index.id)

        # Get all snippets for index
        result = await snippet_repository.get_by_index_id(test_index.id)

        assert len(result) == 2

    async def test_returns_empty_for_nonexistent_index(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
    ) -> None:
        """Test that get_by_index_id() returns empty list for non-existent index."""
        result = await snippet_repository.get_by_index_id(99999)
        assert len(result) == 0


class TestSnippetRepositoryContextManagement:
    """Test snippet contexts and UnitOfWork usage."""

    async def test_snippet_with_context_contains_all_fields(
        self,
        snippet_repository: SqlAlchemySnippetRepository,
        test_index: domain_entities.Index,
        sample_snippet: domain_entities.Snippet,
    ) -> None:
        """Test that SnippetWithContext contains all required fields."""
        # Add snippet
        await snippet_repository.add([sample_snippet], test_index.id)

        # Get snippet with context
        results = await snippet_repository.get_by_index_id(test_index.id)
        snippet_context = results[0]

        # Verify all context fields are present
        assert snippet_context.snippet is not None
        assert snippet_context.file is not None
        assert snippet_context.source is not None
        assert snippet_context.authors is not None
        assert len(snippet_context.authors) > 0

        # Verify snippet content
        assert snippet_context.snippet.original_text() == "def hello():\n    pass"
        assert snippet_context.snippet.summary_text() == "A simple hello function"

        # Verify file information
        assert "sample.py" in str(snippet_context.file.uri)
        assert snippet_context.file.authors[0].name == "John Doe"

        # Verify source information
        assert "test/repo" in str(snippet_context.source.working_copy.remote_uri)


class TestSnippetRepositoryFactory:
    """Test repository factory function."""

    async def test_creates_repository(
        self, session_factory: Callable[[], AsyncSession]
    ) -> None:
        """Test that factory creates repository."""
        repository = create_snippet_repository(session_factory=session_factory)

        assert isinstance(repository, SqlAlchemySnippetRepository)

    async def test_uses_unit_of_work(
        self, session_factory: Callable[[], AsyncSession]
    ) -> None:
        """Test that repository uses unit of work as context manager."""
        # This is implicitly tested by all the other tests, but we can verify
        # the structure
        repository = create_snippet_repository(session_factory=session_factory)

        assert hasattr(repository, "uow")
        assert isinstance(repository.uow, SqlAlchemyUnitOfWork)

