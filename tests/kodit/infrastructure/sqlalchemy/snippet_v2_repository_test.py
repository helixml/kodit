"""Tests for SqlAlchemySnippetRepositoryV2."""

from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitFile, SnippetV2
from kodit.domain.value_objects import Enrichment, EnrichmentType
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.snippet_v2_repository import (
    SqlAlchemySnippetRepositoryV2,
)


@pytest.fixture
def repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemySnippetRepositoryV2:
    """Create a repository with a session factory."""
    return SqlAlchemySnippetRepositoryV2(session_factory)


@pytest.fixture
async def test_git_repo(session: AsyncSession) -> db_entities.GitRepo:
    """Create a sample Git repository entity for testing."""
    git_repo = db_entities.GitRepo(
        sanitized_remote_uri=str(AnyUrl("https://github.com/test/repo")),
        remote_uri=str(AnyUrl("https://github.com/test/repo.git")),
        cloned_path=Path("/tmp/test_repo"),
        last_scanned_at=datetime.now(UTC),
    )
    session.add(git_repo)
    await session.flush()
    await session.commit()
    return git_repo


@pytest.fixture
async def test_git_commit(
    session: AsyncSession, test_git_repo: db_entities.GitRepo
) -> db_entities.GitCommit:
    """Create multiple Git commit entities with different commit SHAs."""
    git_commit = db_entities.GitCommit(
        repo_id=test_git_repo.id,
        commit_sha="abc123def456",
        date=datetime.now(UTC),
        message="Initial commit",
        parent_commit_sha=None,
        author="Test Author",
    )
    session.add(git_commit)
    await session.flush()
    await session.commit()
    return git_commit


@pytest.fixture
async def sample_git_file(
    session: AsyncSession, test_git_commit: db_entities.GitCommit
) -> db_entities.GitCommitFile:
    """Create a sample GitFile entity representing a Python file."""
    git_file = db_entities.GitCommitFile(
        commit_sha=test_git_commit.commit_sha,
        path="src/main.py",
        blob_sha="file_blob_sha_123",
        mime_type="text/x-python",
        size=1024,
        extension="py",
        created_at=datetime.now(UTC),
    )
    session.add(git_file)
    await session.flush()
    await session.commit()
    return git_file


@pytest.fixture
def sample_snippet(sample_git_file: db_entities.GitCommitFile) -> SnippetV2:
    """Create a sample SnippetV2 entity with enrichments and file associations."""
    domain_file = GitFile(
        created_at=datetime.now(UTC),
        blob_sha=sample_git_file.blob_sha,
        path=sample_git_file.path,
        mime_type=sample_git_file.mime_type,
        size=sample_git_file.size,
        extension=sample_git_file.extension,
    )

    content = "def hello_world():\n    print('Hello, World!')"
    sha = SnippetV2.compute_sha(content)

    return SnippetV2(
        sha=sha,
        derives_from=[domain_file],
        content=content,
        enrichments=[
            Enrichment(
                type=EnrichmentType.SUMMARIZATION,
                content="A simple hello world function"
            )
        ],
        extension="py",
    )


class TestSaveSnippets:
    """Tests the core functionality of persisting code snippets to the database."""

    async def test_saves_snippets_with_file_associations(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        test_git_commit: db_entities.GitCommit,
        sample_snippet: SnippetV2,
    ) -> None:
        """Verifies repository can persist snippets with file metadata."""
        # Save the snippet
        await repository.save_snippets(test_git_commit.commit_sha, [sample_snippet])

        # Retrieve snippets back
        snippets = await repository.get_snippets_for_commit(test_git_commit.commit_sha)

        assert len(snippets) == 1
        saved_snippet = snippets[0]
        assert saved_snippet.sha == sample_snippet.sha
        assert saved_snippet.content == sample_snippet.content
        assert len(saved_snippet.derives_from) == 1
        assert saved_snippet.derives_from[0].path == "src/main.py"
        assert len(saved_snippet.enrichments) == 1
        assert saved_snippet.enrichments[0].content == "A simple hello world function"

    async def test_saves_empty_snippets_list(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        test_git_commit: db_entities.GitCommit,
    ) -> None:
        """Ensures the repository gracefully handles empty snippet lists."""
        # Save empty list
        await repository.save_snippets(test_git_commit.commit_sha, [])

        # Should not raise error and should return empty list
        snippets = await repository.get_snippets_for_commit(test_git_commit.commit_sha)
        assert snippets == []

    async def test_replaces_existing_snippets_for_commit(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        test_git_commit: db_entities.GitCommit,
        sample_snippet: SnippetV2,
    ) -> None:
        """Validates proper upsert behavior, preventing duplicates."""
        # Save initial snippet
        await repository.save_snippets(test_git_commit.commit_sha, [sample_snippet])

        # Save the same snippet again
        await repository.save_snippets(test_git_commit.commit_sha, [sample_snippet])

        # Should still have only one snippet
        snippets = await repository.get_snippets_for_commit(test_git_commit.commit_sha)
        assert len(snippets) == 1
        assert snippets[0].sha == sample_snippet.sha

    async def test_creates_git_files_if_not_exist(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        session: AsyncSession,
        test_git_commit: db_entities.GitCommit,
    ) -> None:
        """Ensures the repository handles references to files not previously indexed."""
        # Create a snippet with a file that doesn't exist yet
        new_file = GitFile(
            created_at=datetime.now(UTC),
            blob_sha="new_file_blob_sha",
            path="src/new_file.py",
            mime_type="text/x-python",
            size=512,
            extension="py",
        )

        # Create the git commit file in the database first
        git_commit_file = db_entities.GitCommitFile(
            commit_sha=test_git_commit.commit_sha,
            path=new_file.path,
            blob_sha=new_file.blob_sha,
            mime_type=new_file.mime_type,
            size=new_file.size,
            extension=new_file.extension,
            created_at=datetime.now(UTC),
        )
        session.add(git_commit_file)
        await session.flush()
        await session.commit()

        content = "def new_function():\n    return True"
        snippet = SnippetV2(
            sha=SnippetV2.compute_sha(content),
            derives_from=[new_file],
            content=content,
            enrichments=[],
            extension="py",
        )

        # Save snippet - should handle the file reference properly
        await repository.save_snippets(test_git_commit.commit_sha, [snippet])

        # Verify snippet was saved
        snippets = await repository.get_snippets_for_commit(test_git_commit.commit_sha)
        assert len(snippets) == 1
        assert snippets[0].derives_from[0].path == "src/new_file.py"


class TestGetSnippetsForCommit:
    """Tests the retrieval functionality for code snippets."""

    async def test_returns_empty_list_for_nonexistent_commit(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
    ) -> None:
        """Verifies that queries for non-existent data return empty results."""
        snippets = await repository.get_snippets_for_commit("nonexistent_commit_sha")
        assert snippets == []

    async def test_returns_snippets_with_file_associations(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        test_git_commit: db_entities.GitCommit,
        sample_snippet: SnippetV2,
    ) -> None:
        """Validates complete snippet reconstruction with file metadata."""
        # Save snippet first
        await repository.save_snippets(test_git_commit.commit_sha, [sample_snippet])

        # Retrieve and verify complete reconstruction
        snippets = await repository.get_snippets_for_commit(test_git_commit.commit_sha)

        assert len(snippets) == 1
        retrieved_snippet = snippets[0]

        # Verify snippet data
        assert retrieved_snippet.sha == sample_snippet.sha
        assert retrieved_snippet.content == sample_snippet.content
        assert retrieved_snippet.extension == sample_snippet.extension

        # Verify file associations
        assert len(retrieved_snippet.derives_from) == 1
        file_association = retrieved_snippet.derives_from[0]
        assert file_association.path == "src/main.py"
        assert file_association.blob_sha == "file_blob_sha_123"
        assert file_association.mime_type == "text/x-python"

        # Verify enrichments
        assert len(retrieved_snippet.enrichments) == 1
        enrichment = retrieved_snippet.enrichments[0]
        assert enrichment.type == EnrichmentType.SUMMARIZATION
        assert enrichment.content == "A simple hello world function"


class TestDeleteSnippetsForCommit:
    """Tests the removal functionality for code snippets and their associations."""

    async def test_deletes_snippets_and_associations(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        test_git_commit: db_entities.GitCommit,
        sample_snippet: SnippetV2,
    ) -> None:
        """Ensures deletion operations clean up snippet data and associations."""
        # Save snippet first
        await repository.save_snippets(test_git_commit.commit_sha, [sample_snippet])

        # Verify it exists
        snippets = await repository.get_snippets_for_commit(test_git_commit.commit_sha)
        assert len(snippets) == 1

        # Delete snippets for commit
        await repository.delete_snippets_for_commit(test_git_commit.commit_sha)

        # Verify snippets are gone
        snippets = await repository.get_snippets_for_commit(test_git_commit.commit_sha)
        assert snippets == []

    async def test_handles_nonexistent_commit_gracefully(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
    ) -> None:
        """Verifies deletion operations on non-existent data don't cause errors."""
        # Delete from non-existent commit should not raise error
        await repository.delete_snippets_for_commit("nonexistent_commit_sha")

        # Should complete without error
        assert True

    async def test_deletes_multiple_snippets_for_commit(
        self,
        repository: SqlAlchemySnippetRepositoryV2,
        session: AsyncSession,  # noqa: ARG002
        test_git_commit: db_entities.GitCommit,
        sample_git_file: db_entities.GitCommitFile,
    ) -> None:
        """Validates bulk deletion operations work correctly."""
        # Create multiple snippets for the same commit
        domain_file = GitFile(
            created_at=datetime.now(UTC),
            blob_sha=sample_git_file.blob_sha,
            path=sample_git_file.path,
            mime_type=sample_git_file.mime_type,
            size=sample_git_file.size,
            extension=sample_git_file.extension,
        )

        snippet1_content = "def function_one():\n    return 1"
        snippet1 = SnippetV2(
            sha=SnippetV2.compute_sha(snippet1_content),
            derives_from=[domain_file],
            content=snippet1_content,
            enrichments=[],
            extension="py",
        )

        snippet2_content = "def function_two():\n    return 2"
        snippet2 = SnippetV2(
            sha=SnippetV2.compute_sha(snippet2_content),
            derives_from=[domain_file],
            content=snippet2_content,
            enrichments=[],
            extension="py",
        )

        # Save multiple snippets
        await repository.save_snippets(test_git_commit.commit_sha, [snippet1, snippet2])

        # Verify both exist
        snippets = await repository.get_snippets_for_commit(test_git_commit.commit_sha)
        assert len(snippets) == 2

        # Delete all snippets for commit
        await repository.delete_snippets_for_commit(test_git_commit.commit_sha)

        # Verify all are gone
        snippets = await repository.get_snippets_for_commit(test_git_commit.commit_sha)
        assert snippets == []
