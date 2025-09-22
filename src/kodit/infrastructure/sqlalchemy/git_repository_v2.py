"""SQLAlchemy implementation of lightweight Git repositories.

This module contains repository implementations that follow DDD principles
with proper aggregate boundaries for improved performance.
"""

from collections.abc import Callable
from typing import Any

from pydantic import AnyUrl
from sqlalchemy import delete, insert, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git_v2 import (
    GitBranchV2,
    GitCommitV2,
    GitRepositoryV2,
    GitTagV2,
)
from kodit.domain.protocols_v2 import (
    GitBranchRepositoryV2,
    GitCommitRepositoryV2,
    GitRepositoryRepositoryV2,
    GitTagRepositoryV2,
)
from kodit.infrastructure.mappers.git_mapper_v2 import GitMapperV2
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_git_repository_repository_v2(
    session_factory: Callable[[], AsyncSession],
) -> GitRepositoryRepositoryV2:
    """Create a lightweight git repository."""
    return SqlAlchemyGitRepositoryRepositoryV2(session_factory=session_factory)


class SqlAlchemyGitRepositoryRepositoryV2(GitRepositoryRepositoryV2):
    """SQLAlchemy implementation of GitRepositoryRepositoryV2.

    This repository manages only the GitRepositoryV2 aggregate,
    focusing on essential repository metadata for optimal performance.
    """

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory

    @property
    def _mapper(self) -> GitMapperV2:
        return GitMapperV2()

    async def save(self, repo: GitRepositoryV2) -> GitRepositoryV2:
        """Save or update a repository (metadata only)."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Check if repo exists by URI (for new repos from domain)
            existing_repo_stmt = select(db_entities.GitRepo).where(
                db_entities.GitRepo.sanitized_remote_uri
                == str(repo.sanitized_remote_uri)
            )
            existing_repo = await session.scalar(existing_repo_stmt)

            if existing_repo:
                # Update existing repo found by URI
                existing_repo.remote_uri = str(repo.remote_uri)
                existing_repo.cloned_path = repo.cloned_path
                existing_repo.last_scanned_at = repo.last_scanned_at
                db_repo = existing_repo
                repo.id = existing_repo.id  # Set the domain ID
            else:
                # Create new repo
                db_repo = db_entities.GitRepo(
                    sanitized_remote_uri=str(repo.sanitized_remote_uri),
                    remote_uri=str(repo.remote_uri),
                    cloned_path=repo.cloned_path,
                    last_scanned_at=repo.last_scanned_at,
                )
                session.add(db_repo)
                await session.flush()  # Get the new ID
                repo.id = db_repo.id  # Set the domain ID

            await session.flush()
            return repo

    async def get_by_id(self, repo_id: int) -> GitRepositoryV2:
        """Get repository by ID (metadata only)."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_repo = await session.get(db_entities.GitRepo, repo_id)
            if not db_repo:
                raise ValueError(f"Repository with ID {repo_id} not found")

            # Get tracking branch name if it exists
            tracking_branch = await session.scalar(
                select(db_entities.GitTrackingBranch).where(
                    db_entities.GitTrackingBranch.repo_id == repo_id
                )
            )
            tracking_branch_name = tracking_branch.name if tracking_branch else None

            return self._mapper.to_domain_git_repository(db_repo, tracking_branch_name)

    async def get_by_uri(self, sanitized_uri: AnyUrl) -> GitRepositoryV2:
        """Get repository by sanitized URI (metadata only)."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitRepo).where(
                db_entities.GitRepo.sanitized_remote_uri == str(sanitized_uri)
            )
            db_repo = await session.scalar(stmt)
            if not db_repo:
                raise ValueError(f"Repository with URI {sanitized_uri} not found")

            # Get tracking branch name if it exists
            tracking_branch = await session.scalar(
                select(db_entities.GitTrackingBranch).where(
                    db_entities.GitTrackingBranch.repo_id == db_repo.id
                )
            )
            tracking_branch_name = tracking_branch.name if tracking_branch else None

            return self._mapper.to_domain_git_repository(db_repo, tracking_branch_name)

    async def get_all(self) -> list[GitRepositoryV2]:
        """Get all repositories (metadata only)."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitRepo)
            db_repos = (await session.scalars(stmt)).all()

            repositories = []
            for db_repo in db_repos:
                # Get tracking branch name if it exists
                tracking_branch = await session.scalar(
                    select(db_entities.GitTrackingBranch).where(
                        db_entities.GitTrackingBranch.repo_id == db_repo.id
                    )
                )
                tracking_branch_name = (
                    tracking_branch.name if tracking_branch else None
                )

                repo = self._mapper.to_domain_git_repository(
                    db_repo, tracking_branch_name
                )
                repositories.append(repo)

            return repositories

    async def exists_by_uri(self, sanitized_uri: AnyUrl) -> bool:
        """Check if repository exists by URI."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitRepo.id).where(
                db_entities.GitRepo.sanitized_remote_uri == str(sanitized_uri)
            )
            result = await session.scalar(stmt)
            return result is not None

    async def delete(self, sanitized_uri: AnyUrl) -> bool:
        """Delete a repository and all its associated data.

        Note: This method coordinates deletion across all related aggregates
        but maintains transactional consistency.
        """
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Find the repo
            stmt = select(db_entities.GitRepo).where(
                db_entities.GitRepo.sanitized_remote_uri == str(sanitized_uri)
            )
            db_repo = await session.scalar(stmt)
            if not db_repo:
                return False

            repo_id = db_repo.id

            # Delete in order to respect foreign keys
            # This temporarily handles cleanup across aggregates until we have
            # proper domain services for coordinating deletions

            # 1. Delete commit-file associations
            from sqlalchemy import delete

            commit_shas_stmt = select(db_entities.GitCommit.commit_sha).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            commit_shas = (await session.scalars(commit_shas_stmt)).all()

            for commit_sha in commit_shas:
                del_stmt = delete(db_entities.GitCommitFile).where(
                    db_entities.GitCommitFile.commit_sha == commit_sha
                )
                await session.execute(del_stmt)

            # 2. Delete branches
            del_stmt = delete(db_entities.GitBranch).where(
                db_entities.GitBranch.repo_id == repo_id
            )
            await session.execute(del_stmt)

            # 3. Delete tracking branch
            del_stmt = delete(db_entities.GitTrackingBranch).where(
                db_entities.GitTrackingBranch.repo_id == repo_id
            )
            await session.execute(del_stmt)

            # 4. Delete tags
            del_stmt = delete(db_entities.GitTag).where(
                db_entities.GitTag.repo_id == repo_id
            )
            await session.execute(del_stmt)

            # 5. Delete commits
            del_stmt = delete(db_entities.GitCommit).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            await session.execute(del_stmt)

            # 6. Delete the repo
            del_stmt = delete(db_entities.GitRepo).where(
                db_entities.GitRepo.id == repo_id
            )
            await session.execute(del_stmt)

            return True


def create_git_commit_repository_v2(
    session_factory: Callable[[], AsyncSession],
) -> GitCommitRepositoryV2:
    """Create a git commit repository."""
    return SqlAlchemyGitCommitRepositoryV2(session_factory=session_factory)


class SqlAlchemyGitCommitRepositoryV2(GitCommitRepositoryV2):
    """SQLAlchemy implementation of GitCommitRepositoryV2.

    This repository manages GitCommitV2 aggregates with their associated files,
    providing efficient operations for individual commits.
    """

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory

    @property
    def _mapper(self) -> GitMapperV2:
        return GitMapperV2()

    async def save_commits_bulk(self, commits: list[GitCommitV2]) -> None:
        """Bulk save commits for efficiency."""
        if not commits:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get existing commits
            commit_shas = [commit.commit_sha for commit in commits]
            existing_commits_stmt = select(db_entities.GitCommit.commit_sha).where(
                db_entities.GitCommit.commit_sha.in_(commit_shas)
            )
            existing_commit_shas = set(
                (await session.scalars(existing_commits_stmt)).all()
            )

            # Prepare new commits for bulk insert
            new_commits = []
            new_files = []

            for commit in commits:
                if commit.commit_sha not in existing_commit_shas:
                    # Add commit data
                    new_commits.append(
                        {
                            "commit_sha": commit.commit_sha,
                            "repo_id": commit.repo_id,
                            "date": commit.date,
                            "message": commit.message,
                            "parent_commit_sha": commit.parent_commit_sha,
                            "author": commit.author,
                        }
                    )

                    # Add file data for this commit
                    for file in commit.files:
                        new_files.append(
                            {
                                "commit_sha": commit.commit_sha,
                                "repo_id": commit.repo_id,
                                "path": file.path,
                                "blob_sha": file.blob_sha,
                                "extension": file.extension,
                                "mime_type": file.mime_type,
                                "size": file.size,
                                "created_at": file.created_at,
                            }
                        )

            # Bulk insert commits
            if new_commits:
                stmt = insert(db_entities.GitCommit).values(new_commits)
                await session.execute(stmt)

            # Bulk insert files in chunks
            if new_files:
                await self._bulk_insert_files(session, new_files)

    async def _bulk_insert_files(
        self, session: AsyncSession, new_files: list[dict[str, Any]]
    ) -> None:
        """Bulk insert new files in chunks."""
        chunk_size = 1000
        for i in range(0, len(new_files), chunk_size):
            chunk = new_files[i : i + chunk_size]
            stmt = insert(db_entities.GitCommitFile).values(chunk)
            await session.execute(stmt)

    async def get_by_sha(self, commit_sha: str) -> GitCommitV2:
        """Get a specific commit by its SHA with files."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get the commit
            commit_stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.commit_sha == commit_sha
            )
            db_commit = await session.scalar(commit_stmt)
            if not db_commit:
                raise ValueError(f"Commit with SHA {commit_sha} not found")

            # Get associated files
            files_stmt = select(db_entities.GitCommitFile).where(
                db_entities.GitCommitFile.commit_sha == commit_sha
            )
            db_files = (await session.scalars(files_stmt)).all()

            return self._mapper.to_domain_git_commit(db_commit, list(db_files))

    async def get_commits_for_repo(
        self, repo_id: int, limit: int | None = None, offset: int = 0
    ) -> list[GitCommitV2]:
        """Get commits for a repository with optional pagination."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get commits
            commits_stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            if limit:
                commits_stmt = commits_stmt.limit(limit).offset(offset)

            db_commits = (await session.scalars(commits_stmt)).all()

            if not db_commits:
                return []

            # Get all files for these commits
            commit_shas = [commit.commit_sha for commit in db_commits]
            files_stmt = select(db_entities.GitCommitFile).where(
                db_entities.GitCommitFile.commit_sha.in_(commit_shas)
            )
            db_files = (await session.scalars(files_stmt)).all()

            # Group files by commit SHA
            files_by_commit = {}
            for db_file in db_files:
                if db_file.commit_sha not in files_by_commit:
                    files_by_commit[db_file.commit_sha] = []
                files_by_commit[db_file.commit_sha].append(db_file)

            # Build domain commits
            domain_commits = []
            for db_commit in db_commits:
                commit_files = files_by_commit.get(db_commit.commit_sha, [])
                domain_commit = self._mapper.to_domain_git_commit(
                    db_commit, commit_files
                )
                domain_commits.append(domain_commit)

            return domain_commits

    async def get_commits_by_shas(self, commit_shas: list[str]) -> list[GitCommitV2]:
        """Get multiple commits by their SHAs."""
        if not commit_shas:
            return []

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get commits
            commits_stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.commit_sha.in_(commit_shas)
            )
            db_commits = (await session.scalars(commits_stmt)).all()

            if not db_commits:
                return []

            # Get all files for these commits
            files_stmt = select(db_entities.GitCommitFile).where(
                db_entities.GitCommitFile.commit_sha.in_(commit_shas)
            )
            db_files = (await session.scalars(files_stmt)).all()

            # Group files by commit SHA
            files_by_commit = {}
            for db_file in db_files:
                if db_file.commit_sha not in files_by_commit:
                    files_by_commit[db_file.commit_sha] = []
                files_by_commit[db_file.commit_sha].append(db_file)

            # Build domain commits
            domain_commits = []
            for db_commit in db_commits:
                commit_files = files_by_commit.get(db_commit.commit_sha, [])
                domain_commit = self._mapper.to_domain_git_commit(
                    db_commit, commit_files
                )
                domain_commits.append(domain_commit)

            return domain_commits

    async def get_commit_count_for_repo(self, repo_id: int) -> int:
        """Get total count of commits for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            from sqlalchemy import func

            stmt = select(func.count(db_entities.GitCommit.commit_sha)).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            result = await session.scalar(stmt)
            return result or 0

    async def delete_commits_for_repo(self, repo_id: int) -> int:
        """Delete all commits for a repository, return count deleted."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # First get the commits to get their SHAs
            commit_shas_stmt = select(db_entities.GitCommit.commit_sha).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            commit_shas = (await session.scalars(commit_shas_stmt)).all()

            if not commit_shas:
                return 0

            # Delete files first
            for commit_sha in commit_shas:
                del_files_stmt = delete(db_entities.GitCommitFile).where(
                    db_entities.GitCommitFile.commit_sha == commit_sha
                )
                await session.execute(del_files_stmt)

            # Delete commits
            del_commits_stmt = delete(db_entities.GitCommit).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            result = await session.execute(del_commits_stmt)

            return result.rowcount or 0

    async def commit_exists(self, commit_sha: str) -> bool:
        """Check if a commit exists."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitCommit.commit_sha).where(
                db_entities.GitCommit.commit_sha == commit_sha
            )
            result = await session.scalar(stmt)
            return result is not None

    async def get_repo_id_by_commit(self, commit_sha: str) -> int:
        """Get the repository ID that contains a specific commit."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitCommit.repo_id).where(
                db_entities.GitCommit.commit_sha == commit_sha
            )
            result = await session.scalar(stmt)
            if result is None:
                raise ValueError(f"Commit with SHA {commit_sha} not found")
            return result


def create_git_branch_repository_v2(
    session_factory: Callable[[], AsyncSession],
) -> GitBranchRepositoryV2:
    """Create a git branch repository."""
    return SqlAlchemyGitBranchRepositoryV2(session_factory=session_factory)


class SqlAlchemyGitBranchRepositoryV2(GitBranchRepositoryV2):
    """SQLAlchemy implementation of GitBranchRepositoryV2."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory

    @property
    def _mapper(self) -> GitMapperV2:
        return GitMapperV2()

    async def save_branches_bulk(self, branches: list[GitBranchV2]) -> None:
        """Bulk save branches for efficiency."""
        if not branches:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get existing branches for each repo
            repo_branch_pairs = [(branch.repo_id, branch.name) for branch in branches]

            if not repo_branch_pairs:
                return

            # Check existing branches (we need to check repo_id + name combinations)
            existing_branches = set()
            for repo_id, branch_name in repo_branch_pairs:
                existing_branch = await session.get(
                    db_entities.GitBranch, [repo_id, branch_name]
                )
                if existing_branch:
                    existing_branches.add((repo_id, branch_name))

            # Prepare new branches for bulk insert
            new_branches = []
            for branch in branches:
                if (branch.repo_id, branch.name) not in existing_branches:
                    new_branches.append(
                        {
                            "repo_id": branch.repo_id,
                            "name": branch.name,
                            "head_commit_sha": branch.head_commit_sha,
                        }
                    )

            # Bulk insert new branches
            if new_branches:
                stmt = insert(db_entities.GitBranch).values(new_branches)
                await session.execute(stmt)

    async def get_branches_for_repo(self, repo_id: int) -> list[GitBranchV2]:
        """Get all branches for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitBranch).where(
                db_entities.GitBranch.repo_id == repo_id
            )
            db_branches = (await session.scalars(stmt)).all()
            return [self._mapper.to_domain_git_branch(branch) for branch in db_branches]

    async def get_branch_by_name(self, repo_id: int, name: str) -> GitBranchV2:
        """Get a specific branch by repository ID and name."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_branch = await session.get(db_entities.GitBranch, [repo_id, name])
            if not db_branch:
                raise ValueError(f"Branch '{name}' not found in repository {repo_id}")
            return self._mapper.to_domain_git_branch(db_branch)

    async def get_tracking_branch(self, repo_id: int) -> GitBranchV2 | None:
        """Get the tracking branch for a repository (main/master)."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # First check if there's a specific tracking branch set
            tracking_branch_stmt = select(db_entities.GitTrackingBranch).where(
                db_entities.GitTrackingBranch.repo_id == repo_id
            )
            tracking_branch = await session.scalar(tracking_branch_stmt)

            if tracking_branch:
                db_branch = await session.get(
                    db_entities.GitBranch, [repo_id, tracking_branch.name]
                )
                if db_branch:
                    return self._mapper.to_domain_git_branch(db_branch)

            # Fallback to main/master logic
            for preferred_name in ["main", "master"]:
                db_branch = await session.get(
                    db_entities.GitBranch, [repo_id, preferred_name]
                )
                if db_branch:
                    return self._mapper.to_domain_git_branch(db_branch)

            # Return first available branch
            stmt = select(db_entities.GitBranch).where(
                db_entities.GitBranch.repo_id == repo_id
            ).limit(1)
            db_branch = await session.scalar(stmt)
            if db_branch:
                return self._mapper.to_domain_git_branch(db_branch)

            return None

    async def set_tracking_branch(self, repo_id: int, branch_name: str) -> None:
        """Set the tracking branch for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Check if tracking branch already exists
            existing = await session.get(
                db_entities.GitTrackingBranch, [repo_id, branch_name]
            )
            if not existing:
                tracking_branch = db_entities.GitTrackingBranch(
                    repo_id=repo_id, name=branch_name
                )
                session.add(tracking_branch)

    async def delete_branches_for_repo(self, repo_id: int) -> int:
        """Delete all branches for a repository, return count deleted."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Delete tracking branch first
            del_tracking_stmt = delete(db_entities.GitTrackingBranch).where(
                db_entities.GitTrackingBranch.repo_id == repo_id
            )
            await session.execute(del_tracking_stmt)

            # Delete branches
            del_stmt = delete(db_entities.GitBranch).where(
                db_entities.GitBranch.repo_id == repo_id
            )
            result = await session.execute(del_stmt)
            return result.rowcount or 0


def create_git_tag_repository_v2(
    session_factory: Callable[[], AsyncSession],
) -> GitTagRepositoryV2:
    """Create a git tag repository."""
    return SqlAlchemyGitTagRepositoryV2(session_factory=session_factory)


class SqlAlchemyGitTagRepositoryV2(GitTagRepositoryV2):
    """SQLAlchemy implementation of GitTagRepositoryV2."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory

    @property
    def _mapper(self) -> GitMapperV2:
        return GitMapperV2()

    async def save_tags_bulk(self, tags: list[GitTagV2]) -> None:
        """Bulk save tags for efficiency."""
        if not tags:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Check existing tags (repo_id + name combinations)
            existing_tags = set()
            for tag in tags:
                existing_tag = await session.get(
                    db_entities.GitTag, [tag.repo_id, tag.name]
                )
                if existing_tag:
                    existing_tags.add((tag.repo_id, tag.name))

            # Prepare new tags for bulk insert
            new_tags = []
            for tag in tags:
                if (tag.repo_id, tag.name) not in existing_tags:
                    new_tags.append(
                        {
                            "repo_id": tag.repo_id,
                            "name": tag.name,
                            "target_commit_sha": tag.target_commit_sha,
                        }
                    )

            # Bulk insert new tags
            if new_tags:
                stmt = insert(db_entities.GitTag).values(new_tags)
                await session.execute(stmt)

    async def get_tags_for_repo(self, repo_id: int) -> list[GitTagV2]:
        """Get all tags for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitTag).where(
                db_entities.GitTag.repo_id == repo_id
            )
            db_tags = (await session.scalars(stmt)).all()
            return [self._mapper.to_domain_git_tag(tag) for tag in db_tags]

    async def get_tag_by_name(self, repo_id: int, name: str) -> GitTagV2:
        """Get a specific tag by repository ID and name."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_tag = await session.get(db_entities.GitTag, [repo_id, name])
            if not db_tag:
                raise ValueError(f"Tag '{name}' not found in repository {repo_id}")
            return self._mapper.to_domain_git_tag(db_tag)

    async def get_version_tags_for_repo(self, repo_id: int) -> list[GitTagV2]:
        """Get only version tags for a repository."""
        all_tags = await self.get_tags_for_repo(repo_id)
        return [tag for tag in all_tags if tag.is_version_tag]

    async def delete_tags_for_repo(self, repo_id: int) -> int:
        """Delete all tags for a repository, return count deleted."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            del_stmt = delete(db_entities.GitTag).where(
                db_entities.GitTag.repo_id == repo_id
            )
            result = await session.execute(del_stmt)
            return result.rowcount or 0
