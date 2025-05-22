"""Tests for the source service module."""

from datetime import UTC, datetime, timedelta
from pathlib import Path
import shutil

import git
import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.sources.repository import SourceRepository
from kodit.sources.service import SourceService


@pytest.fixture
def repository(session: AsyncSession) -> SourceRepository:
    """Create a repository instance with a real database session."""
    return SourceRepository(session)


@pytest.fixture
def service(tmp_path: Path, repository: SourceRepository) -> SourceService:
    """Create a service instance with a real repository."""
    return SourceService(tmp_path, repository)


@pytest.mark.asyncio
async def test_create_source_nonexistent_path(service: SourceService) -> None:
    """Test creating a source with a valid file URI but nonexistent path."""
    # Create a file URI for a path that doesn't exist
    nonexistent_path = Path("/nonexistent/path")
    uri = nonexistent_path.as_uri()

    # Try to create a source with the nonexistent path
    with pytest.raises(ValueError, match=f"Folder does not exist: {nonexistent_path}"):
        await service.create(uri)


@pytest.mark.asyncio
async def test_create_source_invalid_path_and_uri(service: SourceService) -> None:
    """Test creating a source with an invalid path that is also not a valid URI."""
    # Try to create a source with an invalid path that is also not a valid URI
    invalid_path = "not/a/valid/path/or/uri"
    with pytest.raises(ValueError, match=f"Unsupported source type: {invalid_path}"):
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
    with pytest.raises(ValueError, match="Unsupported source type: http://example.com"):
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
    assert source.uri == test_dir.as_uri()
    assert source.cloned_path.is_dir()
    assert source.created_at is not None
    assert source.num_files == 2

    # List sources
    sources = await service.list_sources()

    assert len(sources) == 1
    assert sources[0].id == 1
    assert sources[0].created_at.astimezone(UTC) - datetime.now(UTC) < timedelta(
        seconds=1
    )
    assert sources[0].uri.endswith("test_folder")

    # Check that the files are present in the cloned directory
    cloned_path = Path(sources[0].cloned_path)
    assert cloned_path.exists()
    assert cloned_path.is_dir()
    assert not (cloned_path / ".hidden-file").exists()
    assert (cloned_path / "file1.txt").exists()
    assert (cloned_path / "subdir" / "file2.txt").exists()


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
    assert source.cloned_path.is_dir()
    assert source.created_at is not None
    assert source.num_files == 2

    # Check that the files are present in the cloned directory
    cloned_path = Path(source.cloned_path)
    assert cloned_path.exists()
    assert cloned_path.is_dir()
    assert (cloned_path / "file1.txt").exists()
    assert (cloned_path / "subdir" / "file2.txt").exists()

    # Clean up
    shutil.rmtree(repo_path)


@pytest.mark.asyncio
async def test_create_source_relative_path(
    service: SourceService, tmp_path: Path
) -> None:
    """Test creating a source with a relative path, i.e. the current directory."""

    # Should not raise an error
    await service.create(".")
