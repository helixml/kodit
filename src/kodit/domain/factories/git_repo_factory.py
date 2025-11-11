"""Factory for creating GitRepo domain entities."""

from datetime import datetime
from pathlib import Path

from pydantic import AnyUrl

from kodit.domain.entities import WorkingCopy
from kodit.domain.entities.git import GitRepo, TrackingConfig, TrackingType


class GitRepoFactory:
    """Factory for creating GitRepo domain entities."""

    @staticmethod
    def create_from_remote_uri(remote_uri: AnyUrl) -> GitRepo:
        """Create a new Git repository from a remote URI."""
        return GitRepo(
            remote_uri=remote_uri,
            sanitized_remote_uri=WorkingCopy.sanitize_git_url(str(remote_uri)),
        )

    @staticmethod
    def create_from_components(  # noqa: PLR0913
        *,
        repo_id: int | None = None,
        created_at: datetime | None = None,
        updated_at: datetime | None = None,
        sanitized_remote_uri: AnyUrl,
        remote_uri: AnyUrl,
        cloned_path: Path | None = None,
        tracking_config: TrackingConfig | None = None,
        last_scanned_at: datetime | None = None,
        num_commits: int = 0,
        num_branches: int = 0,
        num_tags: int = 0,
    ) -> GitRepo:
        """Create a GitRepo from individual components."""
        if tracking_config is not None:
            return GitRepo(
                id=repo_id,
                created_at=created_at,
                updated_at=updated_at,
                sanitized_remote_uri=sanitized_remote_uri,
                remote_uri=remote_uri,
                cloned_path=cloned_path,
                tracking_config=tracking_config,
                last_scanned_at=last_scanned_at,
                num_commits=num_commits,
                num_branches=num_branches,
                num_tags=num_tags,
            )
        return GitRepo(
            id=repo_id,
            created_at=created_at,
            updated_at=updated_at,
            sanitized_remote_uri=sanitized_remote_uri,
            remote_uri=remote_uri,
            cloned_path=cloned_path,
            last_scanned_at=last_scanned_at,
            num_commits=num_commits,
            num_branches=num_branches,
            num_tags=num_tags,
        )

    @staticmethod
    def create_from_path_scan(  # noqa: PLR0913
        *,
        remote_uri: AnyUrl,
        sanitized_remote_uri: AnyUrl,
        repo_path: Path,
        tracking_branch_name: str,
        last_scanned_at: datetime | None = None,
        num_commits: int = 0,
        num_branches: int = 0,
        num_tags: int = 0,
    ) -> GitRepo:
        """Create a GitRepo from a scanned local repository path."""
        return GitRepo(
            id=None,  # Let repository assign database ID
            sanitized_remote_uri=sanitized_remote_uri,
            remote_uri=remote_uri,
            tracking_config=TrackingConfig(
                type=TrackingType.BRANCH, name=tracking_branch_name
            ),
            cloned_path=repo_path,
            last_scanned_at=last_scanned_at,
            num_commits=num_commits,
            num_branches=num_branches,
            num_tags=num_tags,
        )
