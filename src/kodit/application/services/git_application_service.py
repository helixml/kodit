"""Git repository application service with snippet extraction.

This module provides services for cloning, scanning, and extracting
code snippets from Git repositories, designed to integrate with the
indexing pipeline.
"""

import structlog
from pydantic import AnyUrl

from kodit.domain.entities import WorkingCopy
from kodit.domain.entities.git import GitRepo
from kodit.domain.protocols import (
    GitRepoRepository,
    SnippetRepositoryV2,
)
from kodit.domain.services.git_repository_service import (
    GitRepoFactory,
    GitRepositoryScanner,
    RepositoryCloner,
    RepositoryInfo,
)
from kodit.infrastructure.slicing.slicer import Slicer


class GitApplicationService:
    """Git application service."""

    def __init__(
        self,
        repo_repository: GitRepoRepository,
        scanner: GitRepositoryScanner,
        cloner: RepositoryCloner,
        snippet_repository: SnippetRepositoryV2,
    ) -> None:
        """Initialize the Git application service."""
        self.repo_repository = repo_repository
        self.scanner = scanner
        self.cloner = cloner
        self.repo_factory = GitRepoFactory()
        self.snippet_repository = snippet_repository
        self.slicer = Slicer()  # Add snippet extraction capability
        self._log = structlog.getLogger(__name__)

    async def clone_and_map_repository(self, remote_uri: AnyUrl) -> GitRepo:
        """Clone a new repository and perform initial mapping."""
        # Check if already exists
        sanitized_uri = WorkingCopy.sanitize_git_url(str(remote_uri))
        existing = await self.repo_repository.get_by_uri(sanitized_uri)
        if existing:
            raise ValueError(f"Repository {sanitized_uri} already exists")

        # Clone repository (returns immutable info)
        self._log.info(f"Cloning repository {remote_uri}")
        repo_info = await self.cloner.clone_repository(remote_uri)

        # Scan repository (returns immutable results)
        self._log.info(f"Starting scan of {remote_uri}")
        scan_result = await self.scanner.scan_repository(repo_info.cloned_path)

        # Create GitRepo from scan results
        repo = self.repo_factory.create_from_scan(repo_info, scan_result)

        # Persist everything through the aggregate root
        self._log.info(f"Persisting repository data for {remote_uri}")
        await self.repo_repository.save(repo)

        self._log.info(f"Successfully mapped repository {remote_uri}")
        return repo

    async def update_repository(self, sanitized_uri: AnyUrl) -> GitRepo:
        """Update existing repository with latest changes."""
        existing_repo = await self.repo_repository.get_by_uri(sanitized_uri)
        if not existing_repo:
            raise ValueError(f"Repository {sanitized_uri} not found")

        # Pull latest changes
        await self.cloner.pull_repository(existing_repo)

        # Create repository info from existing repo
        repo_info = RepositoryInfo(
            remote_uri=existing_repo.remote_uri,
            sanitized_remote_uri=existing_repo.sanitized_remote_uri,
            cloned_path=existing_repo.cloned_path,
        )

        # Rescan repository
        scan_result = await self.scanner.scan_repository(repo_info.cloned_path)

        # Create new GitRepo instance
        updated_repo = self.repo_factory.create_from_scan(repo_info, scan_result)

        # Update persistence through the aggregate root
        await self.repo_repository.save(updated_repo)

        return updated_repo

    async def rescan_existing_repository(self, repo: GitRepo) -> GitRepo:
        """Rescan an existing repository without pulling changes."""
        repo_info = RepositoryInfo(
            remote_uri=repo.remote_uri,
            sanitized_remote_uri=repo.sanitized_remote_uri,
            cloned_path=repo.cloned_path,
        )

        scan_result = await self.scanner.scan_repository(repo_info.cloned_path)
        return self.repo_factory.create_from_scan(repo_info, scan_result)

    async def clone_and_scan_with_snippets(
        self, remote_uri: AnyUrl, index_id: int
    ) -> None:
        """Clone a repository, scan it, extract snippets and persist them.

        This method is designed for pipeline integration. Snippets are
        persisted to the database for the next pipeline stage to retrieve.
        """
        # Check if already exists
        sanitized_uri = WorkingCopy.sanitize_git_url(str(remote_uri))
        existing = await self.repo_repository.get_by_uri(sanitized_uri)
        if existing:
            raise ValueError(f"Repository {sanitized_uri} already exists")

        # Clone repository
        self._log.info(f"Cloning repository {remote_uri}")
        repo_info = await self.cloner.clone_repository(remote_uri)

        # Scan repository
        self._log.info(f"Starting scan of {remote_uri}")
        scan_result = await self.scanner.scan_repository(repo_info.cloned_path)

        # Create GitRepo from scan results
        repo = self.repo_factory.create_from_scan(repo_info, scan_result)

        # Extract snippets
        self._log.info(f"Extracting snippets from {remote_uri}")
        snippets = self.slicer.extract_snippets_from_git_files(
            repo.tracking_branch.head_commit.files
        )

        # Persist repository data through the aggregate root
        self._log.info(f"Persisting repository data for {remote_uri}")
        await self.repo_repository.save(repo)

        # Persist snippets to database for next pipeline stage
        if snippets:
            await self.snippet_repository.save_snippets(
                repo.tracking_branch.head_commit.commit_sha, snippets
            )
            self._log.info(
                f"Successfully scanned repository {remote_uri}: "
                f"{len(snippets)} snippets extracted and persisted"
            )
        else:
            self._log.info(
                f"Successfully scanned repository {remote_uri}: No snippets extracted"
            )

    async def update_and_scan_with_snippets(
        self, sanitized_uri: AnyUrl, index_id: int
    ) -> None:
        """Update existing repository, extract and persist snippets.

        This method pulls latest changes, re-extracts snippets and persists
        them for pipeline integration.
        """
        existing_repo = await self.repo_repository.get_by_uri(sanitized_uri)
        if not existing_repo:
            raise ValueError(f"Repository {sanitized_uri} not found")

        # Pull latest changes
        await self.cloner.pull_repository(existing_repo)

        # Create repository info from existing repo
        repo_info = RepositoryInfo(
            remote_uri=existing_repo.remote_uri,
            sanitized_remote_uri=existing_repo.sanitized_remote_uri,
            cloned_path=existing_repo.cloned_path,
        )

        # Rescan repository
        scan_result = await self.scanner.scan_repository(repo_info.cloned_path)

        # Create new GitRepo instance
        updated_repo = self.repo_factory.create_from_scan(repo_info, scan_result)

        # Extract snippets
        self._log.info(f"Extracting snippets from updated repository {sanitized_uri}")
        snippets = self.slicer.extract_snippets_from_git_files(
            updated_repo.tracking_branch.head_commit.files
        )

        # Update persistence through the aggregate root
        await self.repo_repository.save(updated_repo)

        # Persist snippets to database for next pipeline stage
        if snippets:
            await self.snippet_repository.save_snippets(
                updated_repo.tracking_branch.head_commit.commit_sha, snippets
            )
            self._log.info(
                f"Successfully updated repository {sanitized_uri}: "
                f"{len(snippets)} snippets extracted and persisted"
            )
        else:
            self._log.info(
                f"Successfully updated repository {sanitized_uri}: "
                "No snippets extracted"
            )

    async def rescan_with_snippets(self, repo: GitRepo, index_id: int) -> None:
        """Rescan an existing repository, extract and persist snippets without pulling.

        This is useful for re-extracting snippets with updated extraction logic
        without fetching new changes from the remote. Snippets are persisted to
        the database for the next pipeline stage.
        """
        repo_info = RepositoryInfo(
            remote_uri=repo.remote_uri,
            sanitized_remote_uri=repo.sanitized_remote_uri,
            cloned_path=repo.cloned_path,
        )

        scan_result = await self.scanner.scan_repository(repo_info.cloned_path)
        rescanned_repo = self.repo_factory.create_from_scan(repo_info, scan_result)

        # Extract snippets
        self._log.info("Extracting snippets from rescanned repository")
        snippets = self.slicer.extract_snippets_from_git_files(
            rescanned_repo.tracking_branch.head_commit.files
        )

        # Persist snippets to database for next pipeline stage
        if snippets:
            await self.snippet_repository.save_snippets(
                rescanned_repo.tracking_branch.head_commit.commit_sha, snippets
            )
            self._log.info(
                f"Successfully rescanned repository: "
                f"{len(snippets)} snippets extracted and persisted"
            )
        else:
            self._log.info("Successfully rescanned repository: No snippets extracted")
