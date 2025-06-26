"""Tests for the source service module."""

import shutil
from collections.abc import Callable
from pathlib import Path

import git
import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.services.source_service import SourceService
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemySourceRepository


@pytest.fixture
def repository(session: AsyncSession) -> SqlAlchemySourceRepository:
    """Create a repository instance with a real database session."""
    return SqlAlchemySourceRepository(session)


@pytest.fixture
def session_factory(session: AsyncSession) -> Callable[[], AsyncSession]:
    """Create a session factory that returns the test session."""

    def factory() -> AsyncSession:
        return session

    return factory


@pytest.fixture
def service(
    tmp_path: Path, session_factory: Callable[[], AsyncSession]
) -> SourceService:
    """Create a service instance with a real repository."""
    return SourceService(tmp_path, session_factory)


@pytest.mark.asyncio
async def test_create_source_nonexistent_path(service: SourceService) -> None:
    """Test creating a source with a valid file URI but nonexistent path."""
    # Create a file URI for a path that doesn't exist
    nonexistent_path = Path("/nonexistent/path")
    uri = nonexistent_path.as_uri()

    # Try to create a source with the nonexistent path
    with pytest.raises(ValueError):  # noqa: PT011
        await service.create(uri)


@pytest.mark.asyncio
async def test_create_source_invalid_path_and_uri(service: SourceService) -> None:
    """Test creating a source with an invalid path that is also not a valid URI."""
    # Try to create a source with an invalid path that is also not a valid URI
    invalid_path = "not/a/valid/path/or/uri"
    with pytest.raises(ValueError):  # noqa: PT011
        await service.create(invalid_path)


@pytest.mark.asyncio
async def test_create_source_already_added(
    service: SourceService, tmp_path: Path
) -> None:
    """Test creating a source with a path that has already been added."""
    # Create a temporary directory for testing
    test_dir = tmp_path / "test_folder"
    test_dir.mkdir()

    # Create a folder source
    await service.create(str(test_dir))

    # Try to create the same source again, should be fine
    await service.create(str(test_dir))


@pytest.mark.asyncio
async def test_create_source_unsupported_uri(service: SourceService) -> None:
    """Test creating a source with an unsupported URI."""
    # Try to create a source with an unsupported URI (e.g., http)
    with pytest.raises(ValueError):  # noqa: PT011
        await service.create("http://example.com")


@pytest.mark.asyncio
async def test_create_source_list_source(
    service: SourceService, tmp_path: Path
) -> None:
    """Test listing all sources through the service."""
    # Create a temporary directory for testing
    test_dir = tmp_path / "test_folder"
    test_dir.mkdir()

    # Add some files to the test directory
    (test_dir / ".hidden-file").write_text("Super secret")
    (test_dir / "file1.txt").write_text("Hello, world!")
    (test_dir / "subdir").mkdir()
    (test_dir / "subdir" / "file2.txt").write_text("Hello, world!")

    # Create a folder source
    source = await service.create(str(test_dir))
    assert source.id is not None
    assert source.uri.endswith(str(test_dir)) or source.uri == str(test_dir)
    assert Path(source.cloned_path).is_dir()
    assert source.created_at is not None

    # Get the source by ID
    retrieved_source = await service.get(source.id)
    assert retrieved_source.id == source.id
    assert retrieved_source.uri == source.uri


@pytest.mark.asyncio
async def test_create_git_source(service: SourceService, tmp_path: Path) -> None:
    """Test creating a git source."""
    # Create a temporary git repository
    repo_path = tmp_path / "test_repo"
    repo_path.mkdir()
    repo = git.Repo.init(repo_path)

    # Add some files to the repository
    (repo_path / "file1.txt").write_text("Hello, world!")
    (repo_path / "subdir").mkdir()
    (repo_path / "subdir" / "file2.txt").write_text("Hello, world!")

    # Commit the files
    repo.index.add(["file1.txt", "subdir/file2.txt"])
    repo.index.commit("Initial commit")

    # Create a git source
    source = await service.create(repo_path.as_uri())
    assert source.id is not None
    assert source.uri == repo_path.as_uri()
    assert Path(source.cloned_path).is_dir()
    assert source.created_at is not None

    # Check that the files are present in the cloned directory
    cloned_path = Path(source.cloned_path)
    assert cloned_path.exists()
    assert cloned_path.is_dir()
    assert (cloned_path / "file1.txt").exists()
    assert (cloned_path / "subdir" / "file2.txt").exists()

    # Clean up
    shutil.rmtree(repo_path)


@pytest.mark.asyncio
async def test_create_source_relative_path(service: SourceService) -> None:
    """Test creating a source with a relative path, i.e. the current directory."""
    # Create a test directory in the current working directory
    test_dir = Path.cwd() / "test_relative_dir"
    test_dir.mkdir(exist_ok=True)

    try:
        # Should not raise an error for a valid relative path
        await service.create(str(test_dir))
    finally:
        # Clean up
        if test_dir.exists():
            shutil.rmtree(test_dir)


@pytest.mark.asyncio
async def test_create_git_source_with_authors(
    service: SourceService, tmp_path: Path
) -> None:
    """Test creating a git source with authors."""
    # Create a temporary git repository
    repo_path = tmp_path / "test_repo"
    repo = git.Repo.init(repo_path, mkdir=True)

    # Commit a dummy file with a dummy author
    (repo_path / "file1.txt").write_text("Hello, world!")
    repo.index.add(["file1.txt"])
    author = git.Actor("Test Author", "test@example.com")
    repo.index.commit("Initial commit", author=author)

    # Create a git source
    source = await service.create(repo_path.as_uri())
    assert source.id is not None

    # Get the source to verify it was created
    retrieved_source = await service.get(source.id)
    assert retrieved_source.id == source.id
    assert retrieved_source.uri == repo_path.as_uri()


@pytest.mark.asyncio
async def test_get_source_not_found(service: SourceService) -> None:
    """Test getting a source that doesn't exist."""
    with pytest.raises(ValueError, match="Source not found: 999"):
        await service.get(999)
