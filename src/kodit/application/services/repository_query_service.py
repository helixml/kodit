"""Application service for querying repository information."""

from collections.abc import Callable

import structlog

from kodit.domain.entities.git import GitRepo, TrackingConfig, TrackingType
from kodit.domain.protocols import GitAdapter, GitRepoRepository
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.domain.tracking.trackable import Trackable, TrackableReferenceType
from kodit.infrastructure.api.v1.query_params import PaginationParams


class RepositoryQueryService:
    """Service for querying repository information."""

    def __init__(
        self,
        git_repo_repository: GitRepoRepository,
        trackable_resolution: TrackableResolutionService,
        git_adapter: GitAdapter | None = None,
    ) -> None:
        """Initialize the repository query service."""
        self.git_repo_repository = git_repo_repository
        self.trackable_resolution = trackable_resolution
        self.git_adapter = git_adapter
        self.log = structlog.get_logger(__name__)

    async def find_repo_by_url(self, repo_url: str) -> int | None:
        """Find a repository ID by its URL.

        Matches against both remote_uri and sanitized_remote_uri using fuzzy matching.
        """
        from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder

        # Normalize the input URL to match how it's stored in the database
        normalized_url = self._normalize_repo_url(repo_url)

        # Try exact match first
        repos = await self.git_repo_repository.find(
            QueryBuilder().filter(
                "sanitized_remote_uri", FilterOperator.ILIKE, str(normalized_url)
            )
        )

        if repos:
            return repos[0].id

        # Try fuzzy match with repo identifier (e.g., %quant-helper%)
        fuzzy_pattern = self._extract_repo_identifier(repo_url)
        repos = await self.git_repo_repository.find(
            QueryBuilder().filter(
                "sanitized_remote_uri", FilterOperator.ILIKE, f"%{fuzzy_pattern}%"
            )
        )

        if repos:
            return repos[0].id

        self.log.warning("Repository not found by URL", repo_url=repo_url)
        return None

    def _normalize_repo_url(self, url: str) -> str:
        """Normalize a repository URL for consistent matching.

        Handles URLs with or without protocol (https://).
        """
        from kodit.domain.entities import WorkingCopy

        # If URL doesn't start with a protocol, assume https://
        if not url.startswith(("http://", "https://", "ssh://", "git@", "file://")):
            url = f"https://{url}"

        # Use the same sanitization logic as when storing repos
        try:
            sanitized = WorkingCopy.sanitize_git_url(url)
            return str(sanitized)
        except ValueError:
            # If sanitization fails, return the original URL
            return url

    def _extract_repo_identifier(self, url: str) -> str:
        """Extract the repository identifier for fuzzy matching.

        Examples:
            - "quant-helper" -> "quant-helper"
            - "github.com/philwinder/quant-helper" -> "quant-helper"
            - "https://github.com/philwinder/quant-helper" -> "quant-helper"

        """
        # Remove protocol if present
        url = url.replace("https://", "").replace("http://", "")
        url = url.replace("ssh://", "").replace("git@", "")

        # Split by '/' and get the last non-empty part (repo name)
        parts = [p for p in url.split("/") if p]
        if parts:
            # Remove .git suffix if present
            return parts[-1].replace(".git", "")

        # If no parts, return the original (cleaned) url
        return url

    async def find_latest_commit(
        self,
        repo_id: int,
        max_commits_to_check: int = 100,
    ) -> str | None:
        """Find the most recent commit for a repository.

        Uses the repository's tracking_config to determine which branch/tag to check.
        """
        # Get the repository
        repo = await self.git_repo_repository.get(repo_id)
        if not repo:
            self.log.warning("Repository not found", repo_id=repo_id)
            return None

        if not repo.tracking_config:
            raise ValueError(f"Repository {repo.id} has no tracking config")

        # Create trackable from repository's tracking config
        trackable = Trackable(
            type=TrackableReferenceType(repo.tracking_config.type),
            identifier=repo.tracking_config.name,
            repo_id=repo_id,
        )

        # Get candidate commits from the trackable
        candidate_commits = await self.trackable_resolution.resolve_to_commits(
            trackable, max_commits_to_check
        )

        if not candidate_commits:
            return None

        # Return the most recent commit
        return candidate_commits[0]

    async def find_latest_enriched_commit(
        self,
        repo_id: int,
        enrichment_type: str | None = None,
        max_commits_to_check: int = 100,
        check_enrichments_fn: Callable | None = None,
    ) -> str | None:
        """Find the most recent commit with enrichments for a repository.

        Uses the repository's tracking_config to determine which branch/tag to check.
        """
        # Get the repository
        repo = await self.git_repo_repository.get(repo_id)
        if not repo:
            self.log.warning("Repository not found", repo_id=repo_id)
            return None

        if not repo.tracking_config:
            raise ValueError(f"Repository {repo.id} has no tracking config")

        # Create trackable from repository's tracking config
        trackable = Trackable(
            type=TrackableReferenceType(repo.tracking_config.type),
            identifier=repo.tracking_config.name,
            repo_id=repo_id,
        )

        # Get candidate commits from the trackable
        candidate_commits = await self.trackable_resolution.resolve_to_commits(
            trackable, max_commits_to_check
        )

        if not candidate_commits:
            return None

        # Check which commits have enrichments using the provided function
        if check_enrichments_fn:
            for commit_sha in candidate_commits:
                has_enrichments = await check_enrichments_fn(
                    commit_sha=commit_sha,
                    pagination=PaginationParams(page_size=1),
                    enrichment_type=enrichment_type,
                )
                if has_enrichments:
                    return commit_sha

        return None

    async def get_tracking_config(self, repo: GitRepo) -> TrackingConfig:
        """Get the tracking info for a repository."""
        if repo.tracking_config:
            return repo.tracking_config

        # If it doesn't exist, use the git adapter to get the default branch
        if not self.git_adapter:
            raise ValueError("GitAdapter is required for get_tracking_config")
        if not repo.cloned_path:
            raise ValueError(f"Repository {repo.id} has never been cloned")
        default_branch = await self.git_adapter.get_default_branch(repo.cloned_path)
        return TrackingConfig(type=TrackingType.BRANCH, name=default_branch)

    async def resolve_tracked_commit_from_git(self, repo: GitRepo) -> str:
        """Resolve commit SHA from tracking config by querying git directly.

        This is used during initial scanning before branches/tags are in the database.
        Similar to find_latest_commit but works with git directly instead of database.
        """
        if not self.git_adapter:
            raise ValueError(
                "GitAdapter is required for resolve_tracked_commit_from_git"
            )

        if not repo.cloned_path:
            raise ValueError(f"Repository {repo.id} has never been cloned")

        if not repo.tracking_config:
            raise ValueError(f"Repository {repo.id} has no tracking config")

        if repo.tracking_config.type == TrackingType.BRANCH.value:
            return await self.git_adapter.get_latest_commit_sha(
                repo.cloned_path, repo.tracking_config.name
            )
        if repo.tracking_config.type == TrackingType.TAG.value:
            # Get commit SHA from tag
            tags = await self.git_adapter.get_all_tags(repo.cloned_path)
            tag = next(
                (t for t in tags if t["name"] == repo.tracking_config.name),
                None,
            )
            if not tag:
                raise ValueError(
                    f"Tag {repo.tracking_config.name} not found in repository"
                )
            return tag["target_commit_sha"]
        if repo.tracking_config.type == TrackingType.COMMIT_SHA.value:
            return repo.tracking_config.name

        raise ValueError(f"Unknown tracking type: {repo.tracking_config.type}")
