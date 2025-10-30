"""Tests for SqlAlchemyGitFileRepository."""

from collections.abc import Callable
from datetime import UTC, datetime

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit, GitFile, GitRepo
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_file_repository import (
    create_git_file_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


async def test_save_and_get_file(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
) -> None:
    """Test saving a file and retrieving it by composite ID."""
    # Setup: Create repo and commit first (foreign key requirement)
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)
    file_repository = create_git_file_repository(session_factory)

    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None

    # Create a commit without files
    commit = GitCommit(
        created_at=datetime.now(UTC),
        commit_sha="test_commit_1",
        repo_id=saved_repo.id,
        date=datetime.now(UTC),
        message="Test commit",
        parent_commit_sha=None,
        author="test@example.com",
    )
    await commit_repository.save_bulk([commit])

    # Create a test file
    test_file = GitFile(
        created_at=datetime.now(UTC),
        blob_sha="test_blob_sha",
        commit_sha="test_commit_1",
        path="src/test.py",
        mime_type="text/x-python",
        size=1024,
        extension="py",
    )

    # Test: Save and retrieve file
    saved_file = await file_repository.save(test_file)

    # Verify all fields are mapped correctly
    assert saved_file.blob_sha == test_file.blob_sha
    assert saved_file.commit_sha == test_file.commit_sha
    assert saved_file.path == test_file.path
    assert saved_file.mime_type == test_file.mime_type
    assert saved_file.size == test_file.size
    assert saved_file.extension == test_file.extension
    assert saved_file.created_at is not None

    # Get by composite ID (commit_sha, path)
    composite_id = (test_file.commit_sha, test_file.path)
    retrieved_file = await file_repository.get(composite_id)

    assert retrieved_file.blob_sha == test_file.blob_sha
    assert retrieved_file.commit_sha == test_file.commit_sha
    assert retrieved_file.path == test_file.path


async def test_save_bulk_and_find(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
) -> None:
    """Test bulk saving files and finding them with queries."""
    # Setup repositories
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)
    file_repository = create_git_file_repository(session_factory)

    # Create repo and commit
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None

    commit = GitCommit(
        created_at=datetime.now(UTC),
        commit_sha="test_commit_sha",
        repo_id=saved_repo.id,
        date=datetime.now(UTC),
        message="Test commit",
        parent_commit_sha=None,
        author="test@example.com",
    )
    await commit_repository.save_bulk([commit])

    # Create test files
    files = [
        GitFile(
            created_at=datetime.now(UTC),
            blob_sha="blob_1",
            commit_sha="test_commit_sha",
            path="src/main.py",
            mime_type="text/x-python",
            size=1024,
            extension="py",
        ),
        GitFile(
            created_at=datetime.now(UTC),
            blob_sha="blob_2",
            commit_sha="test_commit_sha",
            path="src/utils.py",
            mime_type="text/x-python",
            size=512,
            extension="py",
        ),
        GitFile(
            created_at=datetime.now(UTC),
            blob_sha="blob_3",
            commit_sha="test_commit_sha",
            path="README.md",
            mime_type="text/markdown",
            size=2048,
            extension="md",
        ),
    ]

    # Save all files
    saved_files = await file_repository.save_bulk(files)
    assert len(saved_files) == 3

    # Find all files (no filter)
    query = QueryBuilder()
    all_files = await file_repository.find(query)
    assert len(all_files) == 3

    # Find by extension
    query = QueryBuilder().filter("extension", FilterOperator.EQ, "py")
    py_files = await file_repository.find(query)
    assert len(py_files) == 2
    for file in py_files:
        assert file.extension == "py"

    # Find by mime_type
    query = QueryBuilder().filter("mime_type", FilterOperator.EQ, "text/markdown")
    md_files = await file_repository.find(query)
    assert len(md_files) == 1
    assert md_files[0].path == "README.md"
