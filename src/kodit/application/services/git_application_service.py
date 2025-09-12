"""Git repository application service with snippet extraction.

This module provides services for cloning, scanning, and extracting
code snippets from Git repositories, designed to integrate with the
indexing pipeline.
"""

from collections import defaultdict

import structlog
from pydantic import AnyUrl

from kodit.domain.entities import (
    File,
    FileProcessingStatus,
    GitRepo,
    Snippet,
    WorkingCopy,
)
from kodit.domain.protocols import (
    GitAdapter,
    GitBranchRepository,
    GitCommitRepository,
    GitRepoRepository,
    GitTagRepository,
    SnippetRepository,
)
from kodit.domain.services.git_repository_scanner import (
    GitRepoFactory,
    GitRepositoryScanner,
    RepositoryCloner,
    RepositoryInfo,
)
from kodit.domain.value_objects import LanguageMapping
from kodit.infrastructure.slicing.slicer import Slicer

# GitScanResult removed - snippets are persisted to database
# for next pipeline stage to retrieve


class GitApplicationService:
    """Updated application service using immutable approach."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        commit_repository: GitCommitRepository,
        branch_repository: GitBranchRepository,
        scanner: GitRepositoryScanner,
        cloner: RepositoryCloner,
        git_adapter: GitAdapter,
        tag_repository: GitTagRepository,
        snippet_repository: SnippetRepository,
    ) -> None:
        """Initialize the Git application service."""
        self.repo_repository = repo_repository
        self.commit_repository = commit_repository
        self.branch_repository = branch_repository
        self.scanner = scanner
        self.cloner = cloner
        self.git_adapter = git_adapter
        self.repo_factory = GitRepoFactory()
        self.tag_repository = tag_repository
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

        # Persist everything
        self._log.info(f"Persisting repository data for {remote_uri}")
        await self.repo_repository.save(repo)
        await self.commit_repository.save_commits(
            sanitized_uri, scan_result.all_commits
        )
        await self.branch_repository.save_branches(sanitized_uri, repo.branches)
        await self.tag_repository.save_tags(sanitized_uri, repo.tags)

        self._log.info(f"Successfully mapped repository {remote_uri}")
        return repo

    async def update_repository(self, sanitized_uri: AnyUrl) -> GitRepo:
        """Update existing repository with latest changes."""
        existing_repo = await self.repo_repository.get_by_uri(sanitized_uri)
        if not existing_repo:
            raise ValueError(f"Repository {sanitized_uri} not found")

        # Pull latest changes
        await self.git_adapter.pull_repository(existing_repo.cloned_path)

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

        # Update persistence
        await self.repo_repository.save(updated_repo)
        await self.commit_repository.save_commits(
            sanitized_uri, scan_result.all_commits
        )
        await self.branch_repository.save_branches(sanitized_uri, updated_repo.branches)

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

    def _extract_snippets_from_repo(self, repo: GitRepo) -> list[Snippet]:
        """Extract snippets from all files in the repository.

        Returns a list of extracted snippets.
        """
        # Get unique files from the tracking branch's head commit
        if not repo.tracking_branch or not repo.tracking_branch.head_commit:
            self._log.warning("No tracking branch or head commit found")
            return []

        # Convert GitFile to File objects (simplified for snippet extraction)
        files: list[File] = []
        for git_file in repo.tracking_branch.head_commit.files:
            from pathlib import Path
            file_uri = AnyUrl(Path(git_file.path).as_uri())
            file = File(
                id=None,  # Will be assigned when persisted
                uri=file_uri,
                sha256=git_file.blob_sha,  # Using blob_sha as sha256 approximation
                authors=[],  # Would be populated from git history
                mime_type=git_file.mime_type,
                file_processing_status=FileProcessingStatus.ADDED,
            )
            files.append(file)

        # Group files by language
        lang_files_map: dict[str, list[File]] = defaultdict(list)
        for file in files:
            try:
                ext = file.extension()
                lang = LanguageMapping.get_language_for_extension(ext)
                lang_files_map[lang].append(file)
            except ValueError as e:
                self._log.debug(f"Skipping file {file.uri}: {e}")
                continue

        self._log.info(
            f"Extracting snippets for {len(files)} files "
            f"in {len(lang_files_map)} languages",
            languages=list(lang_files_map.keys()),
        )

        # Extract snippets for each language
        all_snippets = []
        for lang, lang_files in lang_files_map.items():
            if not lang_files:
                continue
            try:
                snippets = self.slicer.extract_snippets(lang_files, language=lang)
                all_snippets.extend(snippets)
                self._log.info(f"Extracted {len(snippets)} snippets for {lang}")
            except (ValueError, RuntimeError) as e:
                self._log.error(f"Failed to extract snippets for {lang}: {e}")
                continue

        self._log.info(f"Total snippets extracted: {len(all_snippets)}")
        return all_snippets

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
        snippets = self._extract_snippets_from_repo(repo)

        # Persist repository data
        self._log.info(f"Persisting repository data for {remote_uri}")
        await self.repo_repository.save(repo)
        await self.commit_repository.save_commits(
            sanitized_uri, scan_result.all_commits
        )
        await self.branch_repository.save_branches(sanitized_uri, repo.branches)
        await self.tag_repository.save_tags(sanitized_uri, repo.tags)

        # Persist snippets to database for next pipeline stage
        if snippets:
            await self.snippet_repository.add(snippets, index_id)
            self._log.info(
                f"Successfully scanned repository {remote_uri}: "
                f"{len(snippets)} snippets extracted and persisted"
            )
        else:
            self._log.info(
                f"Successfully scanned repository {remote_uri}: "
                "No snippets extracted"
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
        await self.git_adapter.pull_repository(existing_repo.cloned_path)

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
        snippets = self._extract_snippets_from_repo(updated_repo)

        # Update persistence
        await self.repo_repository.save(updated_repo)
        await self.commit_repository.save_commits(
            sanitized_uri, scan_result.all_commits
        )
        await self.branch_repository.save_branches(sanitized_uri, updated_repo.branches)
        await self.tag_repository.save_tags(sanitized_uri, updated_repo.tags)

        # Persist snippets to database for next pipeline stage
        if snippets:
            await self.snippet_repository.add(snippets, index_id)
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
        snippets = self._extract_snippets_from_repo(rescanned_repo)

        # Persist snippets to database for next pipeline stage
        if snippets:
            await self.snippet_repository.add(snippets, index_id)
            self._log.info(
                f"Successfully rescanned repository: "
                f"{len(snippets)} snippets extracted and persisted"
            )
        else:
            self._log.info("Successfully rescanned repository: No snippets extracted")
