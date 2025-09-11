from typing import Any

import structlog
from pydantic import AnyUrl

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities import CommitIndex, IndexStatus
from kodit.domain.protocols import (
    CommitIndexRepository,
    GitCommitRepository,
    GitRepoRepository,
    SnippetRepository,
    SnippetRepositoryV2,
)
from kodit.domain.services.index_service import IndexDomainService
from kodit.domain.value_objects import TaskOperation


class CommitIndexingApplicationService:
    """Application service for commit indexing operations."""

    def __init__(
        self,
        commit_index_repository: CommitIndexRepository,
        snippet_v2_repository: SnippetRepositoryV2,
        repo_repository: GitRepoRepository,
        domain_indexer: IndexDomainService,
        commit_repository: GitCommitRepository,
        operation: ProgressTracker,
    ):
        self.commit_index_repository = commit_index_repository
        self.snippet_repository = snippet_v2_repository
        self.repo_repository = repo_repository
        self.domain_indexer = domain_indexer
        self.commit_repository = commit_repository
        self.operation = operation
        self._log = structlog.get_logger(__name__)

    async def index_commit(self, repo_uri: str, commit_sha: str) -> CommitIndex:
        """Index a specific commit."""
        # Check if already indexed
        existing = await self.commit_index_repository.get_by_commit(
            repo_uri, commit_sha
        )
        if existing and existing.status == IndexStatus.COMPLETED:
            self._log.info(f"Commit {commit_sha} already indexed")
            return existing

        async with self.operation.create_child(TaskOperation.INDEX_COMMIT):
            commit = await self.commit_repository.get_by_commit_sha(commit_sha)
            if not commit:
                raise ValueError(f"Commit {commit_sha} not found")

            # Get repository to find local path
            repo = await self.repo_repository.get_by_uri(AnyUrl(repo_uri))
            if not repo:
                raise ValueError(f"Repository {repo_uri} not found")

            # Create pending index entry
            pending_index = CommitIndex(
                commit_sha=commit_sha,
                repo_uri=repo_uri,
                snippets=[],
                status=IndexStatus.IN_PROGRESS,
            )
            await self.commit_index_repository.save(pending_index)

            # Perform indexing
            result = await self.domain_indexer.extract_snippets_from_git_commit(
                commit,
            )

            # Save everything
            commit_index = CommitIndex(
                commit_sha=commit_sha,
                repo_uri=repo_uri,
                snippets=result,
                status=IndexStatus.COMPLETED,
            )
            await self.commit_index_repository.save(commit_index)
            await self.snippet_repository.save_snippets(repo_uri, commit_sha, result)

            return commit_index


class CommitIndexQueryService:
    """Query service for indexed commit data."""

    def __init__(
        self,
        commit_index_repository: CommitIndexRepository,
        snippet_repository: SnippetRepository,
    ):
        self.commit_index_repository = commit_index_repository
        self.snippet_repository = snippet_repository

    async def get_indexed_commits(self, repo_uri: str) -> list[CommitIndex]:
        """Get all indexed commits for a repository."""
        return await self.commit_index_repository.get_indexed_commits_for_repo(repo_uri)

    async def get_commit_index_stats(self, repo_uri: str) -> dict[str, Any]:
        """Get statistics about indexed commits."""
        indexed_commits = await self.get_indexed_commits(repo_uri)

        total_snippets = sum(c.get_snippet_count() for c in indexed_commits)
        completed_count = len(
            [c for c in indexed_commits if c.status == IndexStatus.COMPLETED]
        )
        failed_count = len(
            [c for c in indexed_commits if c.status == IndexStatus.FAILED]
        )

        return {
            "total_indexed_commits": len(indexed_commits),
            "completed_commits": completed_count,
            "failed_commits": failed_count,
            "total_snippets": total_snippets,
            "average_snippets_per_commit": total_snippets / max(completed_count, 1),
        }
