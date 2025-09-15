"""Tests for SqlAlchemySnippetRepositoryV2."""

from datetime import UTC, datetime
from pathlib import Path

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitFile, SnippetV2
from kodit.domain.value_objects import Enrichment, EnrichmentType
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.snippet_v2_repository import (
    SqlAlchemySnippetRepositoryV2,
)
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


@pytest.fixture
async def test_git_repo(session: AsyncSession) -> db_entities.GitRepo:
    """Create a test git repository."""
    repo = db_entities.GitRepo(
        sanitized_remote_uri="https://github.com/test/repo.git",
        remote_uri="https://github.com/test/repo.git",
        cloned_path=Path("/tmp/test/repo"),
    )
    session.add(repo)
    await session.flush()
    return repo


@pytest.fixture
async def test_git_commit(
    session: AsyncSession, test_git_repo: db_entities.GitRepo
) -> db_entities.GitCommit:
    """Create test git commits that will be used in tests."""
    # Create multiple commits for different tests
    commits = [
        db_entities.GitCommit(
            commit_sha="commit_sha_789",
            repo_id=test_git_repo.id,
            date=datetime.now(UTC),
            message="Test commit 1",
            parent_commit_sha=None,
            author="Test Author",
        ),
        db_entities.GitCommit(
            commit_sha="commit_sha_empty",
            repo_id=test_git_repo.id,
            date=datetime.now(UTC),
            message="Test commit 2",
            parent_commit_sha=None,
            author="Test Author",
        ),
        db_entities.GitCommit(
            commit_sha="commit_sha_replace",
            repo_id=test_git_repo.id,
            date=datetime.now(UTC),
            message="Test commit 3",
            parent_commit_sha=None,
            author="Test Author",
        ),
        db_entities.GitCommit(
            commit_sha="commit_with_new_file",
            repo_id=test_git_repo.id,
            date=datetime.now(UTC),
            message="Test commit 4",
            parent_commit_sha=None,
            author="Test Author",
        ),
        db_entities.GitCommit(
            commit_sha="test_commit_sha",
            repo_id=test_git_repo.id,
            date=datetime.now(UTC),
            message="Test commit 5",
            parent_commit_sha=None,
            author="Test Author",
        ),
        db_entities.GitCommit(
            commit_sha="commit_to_delete",
            repo_id=test_git_repo.id,
            date=datetime.now(UTC),
            message="Test commit 6",
            parent_commit_sha=None,
            author="Test Author",
        ),
        db_entities.GitCommit(
            commit_sha="commit_with_multiple_snippets",
            repo_id=test_git_repo.id,
            date=datetime.now(UTC),
            message="Test commit 7",
            parent_commit_sha=None,
            author="Test Author",
        ),
    ]

    for commit in commits:
        session.add(commit)

    await session.flush()
    await session.commit()

    # Return the first one as the main fixture
    return commits[0]


@pytest.fixture
def repository(unit_of_work: SqlAlchemyUnitOfWork) -> SqlAlchemySnippetRepositoryV2:
    """Create a repository with a unit of work."""
    return SqlAlchemySnippetRepositoryV2(unit_of_work)


@pytest.fixture
def sample_git_file() -> GitFile:
    """Create a sample git file."""
    return GitFile(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        blob_sha="file_sha_123",
        path="src/main.py",
        mime_type="text/x-python",
        size=1024,
        extension="py",
    )


@pytest.fixture
def sample_snippet(sample_git_file: GitFile) -> SnippetV2:
    """Create a sample snippet."""
    return SnippetV2(
        sha="snippet_sha_456",
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        derives_from=[sample_git_file],
        content="def hello():\n    print('Hello, World!')",
        enrichments=[
            Enrichment(
                type=EnrichmentType.SUMMARIZATION,
                content="A simple hello function",
            )
        ],
        extension="py",
    )


class TestSaveSnippets:
    """Test save_snippets() method."""

    @pytest.mark.usefixtures("test_git_commit")
    async def test_saves_snippets_with_file_associations(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        sample_snippet: SnippetV2,
    ) -> None:
        """Test that save_snippets() saves snippets with file associations."""
        commit_sha = "commit_sha_789"
        snippets = [sample_snippet]

        await repository.save_snippets(commit_sha, snippets)

        # Verify the snippets were saved
        result = await repository.get_snippets_for_commit(commit_sha)
        assert len(result) == 1
        assert result[0].sha == sample_snippet.sha
        assert result[0].content == sample_snippet.content
        assert len(result[0].derives_from) == 1
        assert result[0].derives_from[0].blob_sha == "file_sha_123"

    async def test_saves_empty_snippets_list(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
    ) -> None:
        """Test that save_snippets() handles empty snippets list."""
        commit_sha = "commit_sha_empty"
        snippets: list[SnippetV2] = []

        # Should not raise an error
        await repository.save_snippets(commit_sha, snippets)

        # Should return empty list
        result = await repository.get_snippets_for_commit(commit_sha)
        assert result == []

    @pytest.mark.usefixtures("test_git_commit")
    async def test_replaces_existing_snippets_for_commit(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        sample_snippet: SnippetV2,
        sample_git_file: GitFile,
    ) -> None:
        """Test that save_snippets() replaces existing snippets for a commit."""
        commit_sha = "commit_sha_replace"

        # First save
        await repository.save_snippets(commit_sha, [sample_snippet])

        # Verify first save
        result = await repository.get_snippets_for_commit(commit_sha)
        assert len(result) == 1
        assert result[0].sha == sample_snippet.sha

        # Create a different snippet
        new_snippet = SnippetV2(
            sha="new_snippet_sha",
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            derives_from=[sample_git_file],
            content="def goodbye():\n    print('Goodbye!')",
            enrichments=[],
            extension="py",
        )

        # Save new snippets (should replace old ones)
        await repository.save_snippets(commit_sha, [new_snippet])

        # Verify replacement
        result = await repository.get_snippets_for_commit(commit_sha)
        assert len(result) == 1
        assert result[0].sha == "new_snippet_sha"
        assert result[0].content == "def goodbye():\n    print('Goodbye!')"

    @pytest.mark.usefixtures("test_git_commit")
    async def test_creates_git_files_if_not_exist(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
    ) -> None:
        """Test that save_snippets() creates GitFile entities if they don't exist."""
        # Create a snippet with a new file that doesn't exist in DB
        new_file = GitFile(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            blob_sha="new_file_sha_999",
            path="src/new_file.py",
            mime_type="text/x-python",
            size=512,
            extension="py",
        )

        snippet = SnippetV2(
            sha="snippet_with_new_file",
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            derives_from=[new_file],
            content="print('new file')",
            enrichments=[],
            extension="py",
        )

        commit_sha = "commit_with_new_file"

        # Save the snippet (should create the GitFile)
        await repository.save_snippets(commit_sha, [snippet])

        # Verify the snippet and file were created
        result = await repository.get_snippets_for_commit(commit_sha)
        assert len(result) == 1
        assert len(result[0].derives_from) == 1
        assert result[0].derives_from[0].blob_sha == "new_file_sha_999"
        assert result[0].derives_from[0].path == "src/new_file.py"


class TestGetSnippetsForCommit:
    """Test get_snippets_for_commit() method."""

    @pytest.mark.usefixtures("test_git_commit")
    async def test_returns_empty_list_for_nonexistent_commit(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
    ) -> None:
        """Test returns empty list for non-existent commit."""
        result = await repository.get_snippets_for_commit("nonexistent_commit")
        assert result == []

    @pytest.mark.usefixtures("test_git_commit")
    async def test_returns_snippets_with_file_associations(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        sample_snippet: SnippetV2,
    ) -> None:
        """Test returns snippets with file associations."""
        commit_sha = "test_commit_sha"

        # Save snippets first
        await repository.save_snippets(commit_sha, [sample_snippet])

        # Get the snippets
        result = await repository.get_snippets_for_commit(commit_sha)

        assert len(result) == 1
        snippet = result[0]
        assert snippet.sha == sample_snippet.sha
        assert snippet.content == sample_snippet.content
        assert len(snippet.derives_from) == 1
        assert snippet.derives_from[0].blob_sha == "file_sha_123"
        assert snippet.derives_from[0].path == "src/main.py"


class TestDeleteSnippetsForCommit:
    """Test delete_snippets_for_commit() method."""

    @pytest.mark.usefixtures("test_git_commit")
    async def test_deletes_snippets_and_associations(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        sample_snippet: SnippetV2,
    ) -> None:
        """Test that delete_snippets_for_commit() removes snippets and associations."""
        commit_sha = "commit_to_delete"

        # Save snippets first
        await repository.save_snippets(commit_sha, [sample_snippet])

        # Verify they exist
        result = await repository.get_snippets_for_commit(commit_sha)
        assert len(result) == 1

        # Delete them
        await repository.delete_snippets_for_commit(commit_sha)

        # Verify they're gone
        result = await repository.get_snippets_for_commit(commit_sha)
        assert result == []

    @pytest.mark.usefixtures("test_git_commit")
    async def test_handles_nonexistent_commit_gracefully(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
    ) -> None:
        """Test handles non-existent commit gracefully."""
        # Should not raise an error
        await repository.delete_snippets_for_commit("nonexistent_commit")

    @pytest.mark.usefixtures("test_git_commit")
    async def test_deletes_multiple_snippets_for_commit(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        sample_git_file: GitFile,
    ) -> None:
        """Test that delete_snippets_for_commit() handles multiple snippets."""
        commit_sha = "commit_with_multiple_snippets"

        # Create multiple snippets
        snippet1 = SnippetV2(
            sha="snippet_1",
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            derives_from=[sample_git_file],
            content="snippet 1 content",
            enrichments=[],
            extension="py",
        )

        snippet2 = SnippetV2(
            sha="snippet_2",
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            derives_from=[sample_git_file],
            content="snippet 2 content",
            enrichments=[],
            extension="py",
        )

        # Save multiple snippets
        await repository.save_snippets(commit_sha, [snippet1, snippet2])

        # Verify they exist
        result = await repository.get_snippets_for_commit(commit_sha)
        assert len(result) == 2

        # Delete all snippets for the commit
        await repository.delete_snippets_for_commit(commit_sha)

        # Verify they're all gone
        result = await repository.get_snippets_for_commit(commit_sha)
        assert result == []
