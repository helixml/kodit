"""Tests for the source service module."""

from datetime import UTC, datetime, timedelta
from pathlib import Path
import shutil

import git
import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.source.source_repository import SourceRepository
from kodit.source.source_service import SourceService


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
    with pytest.raises(ValueError):
        await service.create(uri)


@pytest.mark.asyncio
async def test_create_source_invalid_path_and_uri(service: SourceService) -> None:
    """Test creating a source with an invalid path that is also not a valid URI."""
    # Try to create a source with an invalid path that is also not a valid URI
    invalid_path = "not/a/valid/path/or/uri"
    with pytest.raises(ValueError):
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
    with pytest.raises(ValueError):
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
    await service.create("./tests/kodit/snippets")


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

    # Assert that the author exists in the database
    author = await service.repository.get_author_by_email("test@example.com")
    assert author is not None
    assert author.id is not None

    # Assert there is a file in the database
    files = await service.repository.list_files_for_source(source.id)
    assert len(files) == 1
    file = files[0]
    assert file.id is not None

    # Assert there is a mapping of the author to the file
    files = await service.repository.list_files_for_author(author.id)
    assert len(files) == 1
    assert files[0].id == file.id


@pytest.mark.asyncio
async def test_create_git_source_with_multiple_commits(
    service: SourceService, tmp_path: Path
) -> None:
    """Test creating a git source with multiple commits."""

    # Create a temporary git repository
    repo_path = tmp_path / "test_repo"
    repo = git.Repo.init(repo_path, mkdir=True)

    # Commit a dummy file with a dummy author
    (repo_path / "file1.txt").write_text("Hello, world!")
    repo.index.add(["file1.txt"])
    repo.index.commit(
        "Initial commit",
        commit_date=datetime.now(UTC) - timedelta(days=1),
    )

    # Add a second commit
    (repo_path / "file1.txt").write_text("Hello, world 2!")
    repo.index.add(["file1.txt"])
    repo.index.commit("Second commit")

    # Create a git source
    source = await service.create(repo_path.as_uri())
    assert source.id is not None

    # Assert there is a file in the database
    files = await service.repository.list_files_for_source(source.id)
    assert len(files) == 1
    file = files[0]
    assert file.id is not None

    # Assert that the file has the correct created_at and updated_at
    assert file.created_at is not None
    assert file.updated_at is not None
    assert file.created_at < file.updated_at


@pytest.mark.asyncio
async def test_create_git_source_with_gitignore(
    service: SourceService, tmp_path: Path
) -> None:
    """Test creating a git source with .gitignore file excludes ignored files."""
    # Create a temporary git repository
    repo_path = tmp_path / "test_repo"
    repo = git.Repo.init(repo_path, mkdir=True)

    # Create a .gitignore file with some patterns
    gitignore_content = """# Ignore log files
*.log
*.tmp

# Ignore build directory
build/

# Ignore specific files
secret.txt
config/private.conf
"""
    (repo_path / ".gitignore").write_text(gitignore_content)

    # Create files that should be tracked
    (repo_path / "main.py").write_text("print('main')")
    (repo_path / "utils.py").write_text("def helper(): pass")
    (repo_path / "config").mkdir()
    (repo_path / "config" / "public.conf").write_text("public=true")
    (repo_path / "README.md").write_text("# Test Project")

    # Create files that should be ignored according to .gitignore
    (repo_path / "debug.log").write_text("debug info")
    (repo_path / "temp.tmp").write_text("temporary data")
    (repo_path / "secret.txt").write_text("secret data")
    (repo_path / "config" / "private.conf").write_text("private=secret")
    (repo_path / "build").mkdir()
    (repo_path / "build" / "output.js").write_text("compiled code")

    # Add tracked files to git (excluding ignored files)
    repo.index.add(
        [".gitignore", "main.py", "utils.py", "config/public.conf", "README.md"]
    )
    repo.index.commit("Initial commit with .gitignore")

    # Create a git source
    source = await service.create(repo_path.as_uri())
    assert source.id is not None
    assert source.uri == repo_path.as_uri()
    assert source.cloned_path.is_dir()
    assert source.created_at is not None

    # Get all files ingested by the service
    files = await service.repository.list_files_for_source(source.id)
    ingested_paths = [Path(f.cloned_path).name for f in files]

    # Assert that tracked files are ingested
    assert "main.py" in ingested_paths
    assert "utils.py" in ingested_paths
    assert "public.conf" in ingested_paths
    assert "README.md" in ingested_paths

    # Assert that ignored files are NOT ingested (this is the core test)
    assert "debug.log" not in ingested_paths
    assert "temp.tmp" not in ingested_paths
    assert "secret.txt" not in ingested_paths
    assert "private.conf" not in ingested_paths
    assert "output.js" not in ingested_paths

    # Verify that ignored files don't exist in the cloned directory
    # (Git clone respects .gitignore for untracked files)
    cloned_path = Path(source.cloned_path)
    assert not (cloned_path / "debug.log").exists()
    assert not (cloned_path / "temp.tmp").exists()
    assert not (cloned_path / "secret.txt").exists()

    # These tracked files should exist on disk and be ingested
    assert (cloned_path / "main.py").exists()
    assert (cloned_path / "utils.py").exists()
    assert (cloned_path / "README.md").exists()

    # Verify we only ingested the tracked files (not the ignored ones)
    assert len(files) == 4  # main.py, utils.py, public.conf, README.md

    # Clean up
    shutil.rmtree(repo_path)
