"""Domain service for resolving trackables to commits."""

import structlog

from kodit.domain.protocols import (
    GitBranchRepository,
    GitCommitRepository,
    GitTagRepository,
)
from kodit.domain.tracking.trackable import Trackable, TrackableReferenceType


class TrackableResolutionService:
    """Resolves trackables to ordered lists of commits.

    This is a domain service because it orchestrates multiple aggregates
    (branches, tags, commits) without belonging to any single entity.
    """

    def __init__(
        self,
        commit_repo: GitCommitRepository,
        branch_repo: GitBranchRepository,
        tag_repo: GitTagRepository,
    ) -> None:
        """Initialize the trackable resolution service."""
        self.commit_repo = commit_repo
        self.branch_repo = branch_repo
        self.tag_repo = tag_repo
        self.log = structlog.get_logger(__name__)

    async def resolve_to_commits(
        self, trackable: Trackable, limit: int = 100
    ) -> list[str]:
        """Resolve a trackable to an ordered list of commit SHAs.

        Returns commits from newest to oldest based on git history.
        """
        if trackable.type == TrackableReferenceType.BRANCH:
            return await self._resolve_branch(trackable, limit)
        if trackable.type == TrackableReferenceType.TAG:
            return await self._resolve_tag(trackable, limit)
        # COMMIT_SHA
        return [trackable.identifier]

    async def _resolve_branch(self, trackable: Trackable, limit: int) -> list[str]:
        """Get commits from branch HEAD backwards through history."""
        branch = await self.branch_repo.get_by_name(
            trackable.identifier, trackable.repo_id
        )
        # Walk commit history from head_commit backwards
        return await self._walk_commit_history(branch.head_commit_sha, limit)

    async def _resolve_tag(self, trackable: Trackable, limit: int) -> list[str]:
        """Get commits from tag target backwards through history."""
        tag = await self.tag_repo.get_by_name(trackable.identifier, trackable.repo_id)
        return await self._walk_commit_history(tag.target_commit_sha, limit)

    async def _walk_commit_history(self, start_sha: str, limit: int) -> list[str]:
        """Walk commit history backwards from start_sha."""
        result = []
        current_sha: str | None = start_sha

        for _ in range(limit):
            if not current_sha:
                break
            result.append(current_sha)
            if await self.commit_repo.exists(current_sha):
                commit = await self.commit_repo.get(current_sha)
                current_sha = commit.parent_commit_sha or None
            else:
                current_sha = None

        return result
