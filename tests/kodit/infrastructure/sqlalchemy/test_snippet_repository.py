"""Tests for SQLAlchemy snippet repository."""

from datetime import UTC, datetime

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import File, Index, Snippet, Source, SourceType
from kodit.infrastructure.sqlalchemy.snippet_repository import (
    SqlAlchemySnippetRepository,
)


@pytest.mark.asyncio
async def test_list_snippets_by_file_path(session: AsyncSession) -> None:
    """Test listing snippets by specific file path."""
    # Create test data
    source = Source(
        uri="https://github.com/test/repo.git",
        cloned_path="/tmp/test_repo",  # noqa: S108
        source_type=SourceType.GIT,
    )
    session.add(source)
    await session.commit()

    # Create an index
    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()

    file = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source.id,
        mime_type="text/plain",
        uri="test.py",
        cloned_path="/tmp/test_repo/test.py",  # noqa: S108
        sha256="abc123",
        size_bytes=100,
        extension="py",
    )
    session.add(file)
    await session.commit()

    # Create snippets
    snippet1 = Snippet(file_id=file.id, index_id=index.id, content="snippet 1")
    snippet2 = Snippet(file_id=file.id, index_id=index.id, content="snippet 2")
    session.add(snippet1)
    session.add(snippet2)
    await session.commit()

    # Create repository and test
    repository = SqlAlchemySnippetRepository(session)
    result = await repository.list_snippets("/tmp/test_repo/test.py")  # noqa: S108

    # Verify results
    assert len(result) == 2
    assert result[0].content == "snippet 1"
    assert result[0].file_path == "test.py"
    assert result[0].source_uri == "https://github.com/test/repo.git"
    assert result[1].content == "snippet 2"
    assert result[1].file_path == "test.py"
    assert result[1].source_uri == "https://github.com/test/repo.git"


@pytest.mark.asyncio
async def test_list_snippets_by_source_uri(session: AsyncSession) -> None:
    """Test listing snippets by source URI."""
    # Create test data
    source1 = Source(
        uri="https://github.com/test/repo1.git",
        cloned_path="/tmp/test_repo1",  # noqa: S108
        source_type=SourceType.GIT,
    )
    source2 = Source(
        uri="https://github.com/test/repo2.git",
        cloned_path="/tmp/test_repo2",  # noqa: S108
        source_type=SourceType.GIT,
    )
    session.add(source1)
    session.add(source2)
    await session.commit()

    # Create indexes
    index1 = Index(source_id=source1.id)
    index2 = Index(source_id=source2.id)
    session.add(index1)
    session.add(index2)
    await session.commit()

    # Create files
    file1 = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source1.id,
        mime_type="text/plain",
        uri="file1.py",
        cloned_path="/tmp/test_repo1/file1.py",  # noqa: S108
        sha256="abc123",
        size_bytes=100,
        extension="py",
    )
    file2 = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source2.id,
        mime_type="text/plain",
        uri="file2.py",
        cloned_path="/tmp/test_repo2/file2.py",  # noqa: S108
        sha256="abc123",
        size_bytes=100,
        extension="py",
    )
    session.add(file1)
    session.add(file2)
    await session.commit()

    # Create snippets
    snippet1 = Snippet(
        file_id=file1.id, index_id=index1.id, content="snippet from repo1"
    )
    snippet2 = Snippet(
        file_id=file2.id, index_id=index2.id, content="snippet from repo2"
    )
    session.add(snippet1)
    session.add(snippet2)
    await session.commit()

    # Create repository and test
    repository = SqlAlchemySnippetRepository(session)
    result = await repository.list_snippets(
        source_uri="https://github.com/test/repo1.git"
    )

    # Verify results - should only get snippets from repo1
    assert len(result) == 1
    assert result[0].content == "snippet from repo1"
    assert result[0].file_path == "file1.py"
    assert result[0].source_uri == "https://github.com/test/repo1.git"


@pytest.mark.asyncio
async def test_list_snippets_by_directory_path(session: AsyncSession) -> None:
    """Test listing snippets by directory path."""
    # Create test data
    source = Source(
        uri="https://github.com/test/repo.git",
        cloned_path="/tmp/test_repo",  # noqa: S108
        source_type=SourceType.GIT,
    )
    session.add(source)
    await session.commit()

    # Create an index
    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()

    # Create multiple files in the same directory
    file1 = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source.id,
        mime_type="text/plain",
        uri="file1.py",
        cloned_path="/tmp/test_repo/file1.py",  # noqa: S108
        sha256="abc123",
        size_bytes=100,
        extension="py",
    )
    file2 = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source.id,
        mime_type="text/plain",
        uri="file2.py",
        cloned_path="/tmp/test_repo/file2.py",  # noqa: S108
        sha256="abc123",
        size_bytes=100,
        extension="py",
    )
    session.add(file1)
    session.add(file2)
    await session.commit()

    # Create snippets for both files
    snippet1 = Snippet(
        file_id=file1.id, index_id=index.id, content="snippet from file1"
    )
    snippet2 = Snippet(
        file_id=file2.id, index_id=index.id, content="snippet from file2"
    )
    session.add(snippet1)
    session.add(snippet2)
    await session.commit()

    # Create repository and test
    repository = SqlAlchemySnippetRepository(session)
    result = await repository.list_snippets("/tmp/test_repo/")  # noqa: S108

    # Verify results - should get snippets from both files
    assert len(result) == 2
    assert any(s.content == "snippet from file1" for s in result)
    assert any(s.content == "snippet from file2" for s in result)
    assert any(s.file_path == "file1.py" for s in result)
    assert any(s.file_path == "file2.py" for s in result)
    assert all(s.source_uri == "https://github.com/test/repo.git" for s in result)


@pytest.mark.asyncio
async def test_list_snippets_no_filter(session: AsyncSession) -> None:
    """Test listing all snippets when no filter is provided."""
    # Create test data
    source = Source(
        uri="https://github.com/test/repo.git",
        cloned_path="/tmp/test_repo",  # noqa: S108
        source_type=SourceType.GIT,
    )
    session.add(source)
    await session.commit()

    # Create an index
    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()

    file = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source.id,
        mime_type="text/plain",
        uri="test.py",
        cloned_path="/tmp/test_repo/test.py",  # noqa: S108
        sha256="abc123",
        size_bytes=100,
        extension="py",
    )
    session.add(file)
    await session.commit()

    # Create snippets
    snippet1 = Snippet(file_id=file.id, index_id=index.id, content="snippet 1")
    snippet2 = Snippet(file_id=file.id, index_id=index.id, content="snippet 2")
    session.add(snippet1)
    session.add(snippet2)
    await session.commit()

    # Create repository and test
    repository = SqlAlchemySnippetRepository(session)
    result = await repository.list_snippets()

    # Verify results - should get all snippets
    assert len(result) == 2
    assert result[0].content == "snippet 1"
    assert result[0].file_path == "test.py"
    assert result[0].source_uri == "https://github.com/test/repo.git"
    assert result[1].content == "snippet 2"
    assert result[1].file_path == "test.py"
    assert result[1].source_uri == "https://github.com/test/repo.git"


@pytest.mark.asyncio
async def test_list_snippets_no_results(session: AsyncSession) -> None:
    """Test listing snippets when no results match the filter."""
    # Create test data
    source = Source(
        uri="https://github.com/test/repo.git",
        cloned_path="/tmp/test_repo",  # noqa: S108
        source_type=SourceType.GIT,
    )
    session.add(source)
    await session.commit()

    # Create an index
    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()

    file = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source.id,
        mime_type="text/plain",
        uri="test.py",
        cloned_path="/tmp/test_repo/test.py",  # noqa: S108
        sha256="abc123",
        size_bytes=100,
        extension="py",
    )
    session.add(file)
    await session.commit()

    # Create snippets
    snippet = Snippet(file_id=file.id, index_id=index.id, content="snippet")
    session.add(snippet)
    await session.commit()

    # Create repository and test
    repository = SqlAlchemySnippetRepository(session)
    result = await repository.list_snippets("/nonexistent/path")

    # Verify results - should get no snippets
    assert len(result) == 0


@pytest.mark.asyncio
async def test_list_snippets_by_relative_path(session: AsyncSession) -> None:
    """Test listing by relative path (should match relative, not full, path)."""
    # Create test data
    source = Source(
        uri="https://dev.azure.com/winderai/public-test/_git/simple-ddd-brewing-demo",
        cloned_path="/tmp/test_repo",  # noqa: S108
        source_type=SourceType.GIT,
    )
    session.add(source)
    await session.commit()

    # Create an index
    index = Index(source_id=source.id)
    session.add(index)
    await session.commit()

    file = File(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source_id=source.id,
        mime_type="text/plain",
        uri="domain/Beer.js",
        cloned_path="/tmp/test_repo/domain/Beer.js",  # noqa: S108
        sha256="abc123",
        size_bytes=100,
        extension="js",
    )
    session.add(file)
    await session.commit()

    # Create a snippet
    snippet = Snippet(file_id=file.id, index_id=index.id, content="class Beer {}")
    session.add(snippet)
    await session.commit()

    # Create repository and test
    repository = SqlAlchemySnippetRepository(session)
    # Should match by relative path
    result = await repository.list_snippets(file_path="domain/Beer.js")

    # This should return the snippet
    assert len(result) == 1
    assert result[0].file_path == "domain/Beer.js"
    assert result[0].content == "class Beer {}"
    assert (
        result[0].source_uri
        == "https://dev.azure.com/winderai/public-test/_git/simple-ddd-brewing-demo"
    )
