"""Tests for the GitMapper."""

from datetime import UTC, datetime
from pathlib import Path

import kodit.domain.entities.git as domain_git_entities
from kodit.domain.entities.git import IndexStatus
from kodit.domain.value_objects import Enrichment, EnrichmentType
from kodit.infrastructure.mappers.git_mapper import GitMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities


class TestGitMapper:
    """Test the GitMapper."""

    def test_to_domain_git_repo(self) -> None:
        """Test converting database GitRepo to domain GitRepo."""
        # Create test data
        now = datetime.now(UTC)

        db_repo = db_entities.GitRepo(
            sanitized_remote_uri="https://github.com/test/repo",
            remote_uri="https://github.com/test/repo.git",
            cloned_path=Path("/tmp/test_repo"),
            last_scanned_at=now,
        )
        db_repo.id = 1
        db_repo.created_at = now
        db_repo.updated_at = now

        db_file = db_entities.GitFile(
            blob_sha="file_sha_123",
            path="src/main.py",
            mime_type="text/x-python",
            size=1024,
            extension="py",
        )
        db_file.created_at = now
        db_file.updated_at = now

        db_commit = db_entities.GitCommit(
            commit_sha="commit_sha_456",
            repo_id=1,
            date=now,
            message="Initial commit",
            parent_commit_sha="parent_sha_789",
            author="Test Author",
        )
        db_commit.created_at = now
        db_commit.updated_at = now

        db_branch = db_entities.GitBranch(
            repo_id=1,
            name="main",
            head_commit_sha="commit_sha_456",
        )
        db_branch.id = 1
        db_branch.created_at = now
        db_branch.updated_at = now

        db_tag = db_entities.GitTag(
            repo_id=1,
            name="v1.0.0",
            target_commit_sha="commit_sha_456",
        )
        db_tag.created_at = now
        db_tag.updated_at = now

        commit_files_map = {"commit_sha_456": ["file_sha_123"]}

        # Test mapping
        mapper = GitMapper()
        domain_repo = mapper.to_domain_git_repo(
            db_repo=db_repo,
            db_branches=[db_branch],
            db_commits=[db_commit],
            db_tags=[db_tag],
            db_files=[db_file],
            commit_files_map=commit_files_map,
            tracking_branch_name="main",
        )

        # Verify mapping
        assert domain_repo.id == db_repo.id
        assert str(domain_repo.sanitized_remote_uri) == db_repo.sanitized_remote_uri
        assert str(domain_repo.remote_uri) == db_repo.remote_uri
        assert domain_repo.cloned_path == db_repo.cloned_path
        assert domain_repo.last_scanned_at == db_repo.last_scanned_at

        # Verify branch
        assert len(domain_repo.branches) == 1
        assert domain_repo.branches[0].name == "main"
        assert domain_repo.tracking_branch is not None
        assert domain_repo.tracking_branch.name == "main"

        # Verify commit
        assert len(domain_repo.commits) == 1
        assert domain_repo.commits[0].commit_sha == "commit_sha_456"
        assert domain_repo.commits[0].message == "Initial commit"
        assert len(domain_repo.commits[0].files) == 1
        assert domain_repo.commits[0].files[0].blob_sha == "file_sha_123"

        # Verify tag
        assert len(domain_repo.tags) == 1
        assert domain_repo.tags[0].name == "v1.0.0"

    def test_to_domain_snippet_v2(self) -> None:
        """Test converting database SnippetV2 to domain SnippetV2."""
        now = datetime.now(UTC)

        db_snippet = db_entities.SnippetV2(
            sha="snippet_sha_123",
            content="def hello():\n    print('Hello')",
            extension="py",
        )
        db_snippet.created_at = now
        db_snippet.updated_at = now

        domain_file = domain_git_entities.GitFile(
            created_at=now,
            updated_at=now,
            blob_sha="file_sha_456",
            path="hello.py",
            mime_type="text/x-python",
            size=50,
            extension="py",
        )

        db_enrichment = db_entities.Enrichment(
            snippet_sha="snippet_sha_123",
            type=db_entities.EnrichmentType.SUMMARIZATION,
            content="A simple hello function",
        )

        # Test mapping
        mapper = GitMapper()
        domain_snippet = mapper.to_domain_snippet_v2(
            db_snippet=db_snippet,
            derives_from=[domain_file],
            db_enrichments=[db_enrichment],
        )

        # Verify mapping
        assert domain_snippet.sha == db_snippet.sha
        assert domain_snippet.content == db_snippet.content
        assert domain_snippet.extension == db_snippet.extension
        assert len(domain_snippet.derives_from) == 1
        assert domain_snippet.derives_from[0].blob_sha == "file_sha_456"
        assert len(domain_snippet.enrichments) == 1
        assert domain_snippet.enrichments[0].type == EnrichmentType.SUMMARIZATION
        assert domain_snippet.enrichments[0].content == "A simple hello function"

    def test_from_domain_snippet_v2(self) -> None:
        """Test converting domain SnippetV2 to database SnippetV2."""
        now = datetime.now(UTC)

        domain_file = domain_git_entities.GitFile(
            created_at=now,
            updated_at=now,
            blob_sha="file_sha_789",
            path="test.js",
            mime_type="text/javascript",
            size=100,
            extension="js",
        )

        domain_snippet = domain_git_entities.SnippetV2(
            sha="snippet_sha_456",
            created_at=now,
            updated_at=now,
            derives_from=[domain_file],
            content="console.log('test');",
            enrichments=[],
            extension="js",
        )

        # Test mapping
        mapper = GitMapper()
        db_snippet = mapper.from_domain_snippet_v2(domain_snippet)

        # Verify mapping
        assert db_snippet.sha == domain_snippet.sha
        assert db_snippet.content == domain_snippet.content
        assert db_snippet.extension == domain_snippet.extension

    def test_from_domain_enrichments(self) -> None:
        """Test converting domain enrichments to database enrichments."""
        enrichments = [
            Enrichment(
                type=EnrichmentType.SUMMARIZATION,
                content="Function that processes data",
            ),
            Enrichment(
                type=EnrichmentType.UNKNOWN,
                content="Detailed documentation here",
            ),
        ]

        # Test mapping
        mapper = GitMapper()
        db_enrichments = mapper.from_domain_enrichments(
            snippet_sha="test_sha",
            enrichments=enrichments,
        )

        # Verify mapping
        assert len(db_enrichments) == 2
        assert db_enrichments[0].snippet_sha == "test_sha"
        assert db_enrichments[0].type == db_entities.EnrichmentType.SUMMARIZATION
        assert db_enrichments[0].content == "Function that processes data"
        assert db_enrichments[1].type == db_entities.EnrichmentType.UNKNOWN
        assert db_enrichments[1].content == "Detailed documentation here"

    def test_to_domain_commit_index(self) -> None:
        """Test converting database CommitIndex to domain CommitIndex."""
        now = datetime.now(UTC)

        db_commit_index = db_entities.CommitIndex(
            commit_sha="commit_sha_123",
            status=db_entities.IndexStatusType.COMPLETED,
            indexed_at=now,
            error_message=None,
            files_processed=5,
            processing_time_seconds=2.5,
        )
        db_commit_index.created_at = now
        db_commit_index.updated_at = now

        domain_snippet = domain_git_entities.SnippetV2(
            sha="snippet_sha_789",
            created_at=now,
            updated_at=now,
            derives_from=[],
            content="test content",
            enrichments=[],
            extension="py",
        )

        # Test mapping
        mapper = GitMapper()
        domain_commit_index = mapper.to_domain_commit_index(
            db_commit_index=db_commit_index,
            snippets=[domain_snippet],
        )

        # Verify mapping
        assert domain_commit_index.commit_sha == db_commit_index.commit_sha
        assert domain_commit_index.status == IndexStatus.COMPLETED
        assert domain_commit_index.indexed_at == db_commit_index.indexed_at
        assert domain_commit_index.files_processed == db_commit_index.files_processed
        assert domain_commit_index.processing_time_seconds == 2.5
        assert len(domain_commit_index.snippets) == 1
        assert domain_commit_index.snippets[0].sha == "snippet_sha_789"

    def test_from_domain_commit_index(self) -> None:
        """Test converting domain CommitIndex to database CommitIndex."""
        now = datetime.now(UTC)

        domain_commit_index = domain_git_entities.CommitIndex(
            commit_sha="commit_sha_456",
            created_at=now,
            updated_at=now,
            snippets=[],
            status=IndexStatus.IN_PROGRESS,
            indexed_at=None,
            error_message="Processing error",
            files_processed=3,
            processing_time_seconds=1.5,
        )

        # Test mapping
        mapper = GitMapper()
        db_commit_index = mapper.from_domain_commit_index(domain_commit_index)

        # Verify mapping
        assert db_commit_index.commit_sha == domain_commit_index.commit_sha
        assert db_commit_index.status == db_entities.IndexStatusType.IN_PROGRESS
        assert db_commit_index.indexed_at == domain_commit_index.indexed_at
        assert db_commit_index.error_message == domain_commit_index.error_message
        assert db_commit_index.files_processed == domain_commit_index.files_processed
        assert db_commit_index.processing_time_seconds == "1.5"

    def test_to_domain_git_repo_no_tracking_branch(self) -> None:
        """Test converting GitRepo when tracking branch is not found."""
        now = datetime.now(UTC)

        db_repo = db_entities.GitRepo(
            sanitized_remote_uri="https://github.com/test/repo",
            remote_uri="https://github.com/test/repo.git",
            cloned_path=Path("/tmp/test_repo2"),
            last_scanned_at=now,
        )
        db_repo.id = 1
        db_repo.created_at = now
        db_repo.updated_at = now

        db_commit = db_entities.GitCommit(
            commit_sha="commit_sha_123",
            repo_id=1,
            date=now,
            message="Test commit",
            parent_commit_sha=None,
            author="Author",
        )
        db_commit.created_at = now
        db_commit.updated_at = now

        db_branch = db_entities.GitBranch(
            repo_id=1,
            name="develop",
            head_commit_sha="commit_sha_123",
        )
        db_branch.id = 1
        db_branch.created_at = now
        db_branch.updated_at = now

        # Test mapping with missing tracking branch (should use first branch)
        mapper = GitMapper()
        domain_repo = mapper.to_domain_git_repo(
            db_repo=db_repo,
            db_branches=[db_branch],
            db_commits=[db_commit],
            db_tags=[],
            db_files=[],
            commit_files_map={},
            tracking_branch_name="non_existent_branch",
        )

        # Should fallback to first branch
        assert domain_repo.tracking_branch is not None
        assert domain_repo.tracking_branch.name == "develop"
