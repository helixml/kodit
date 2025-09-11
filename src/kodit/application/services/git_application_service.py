import structlog
from pydantic import AnyUrl

from kodit.domain.entities import GitRepo, WorkingCopy
from kodit.domain.protocols import (
    GitAdapter,
    GitBranchRepository,
    GitCommitRepository,
    GitRepoRepository,
    GitTagRepository,
)
from kodit.domain.services.git_repository_scanner import (
    GitRepoFactory,
    GitRepositoryScanner,
    RepositoryCloner,
    RepositoryInfo,
)


class GitApplicationService:
    """Updated application service using immutable approach."""

    def __init__(
        self,
        repo_repository: GitRepoRepository,
        commit_repository: GitCommitRepository,
        branch_repository: GitBranchRepository,
        scanner: GitRepositoryScanner,
        cloner: RepositoryCloner,
        git_adapter: GitAdapter,
        tag_repository: GitTagRepository,
    ) -> None:
        self.repo_repository = repo_repository
        self.commit_repository = commit_repository
        self.branch_repository = branch_repository
        self.scanner = scanner
        self.cloner = cloner
        self.git_adapter = git_adapter
        self.repo_factory = GitRepoFactory()
        self.tag_repository = tag_repository
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
