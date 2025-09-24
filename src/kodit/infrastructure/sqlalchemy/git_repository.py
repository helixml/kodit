"""SQLAlchemy implementation of GitRepoRepository."""

from collections.abc import Callable

from pydantic import AnyUrl
from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitRepo
from kodit.domain.protocols import GitRepoRepository
from kodit.infrastructure.mappers.git_mapper import GitMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_git_repo_repository(
    session_factory: Callable[[], AsyncSession],
) -> GitRepoRepository:
    """Create a git repository."""
    return SqlAlchemyGitRepoRepository(session_factory=session_factory)


class SqlAlchemyGitRepoRepository(GitRepoRepository):
    """SQLAlchemy implementation of GitRepoRepository.

    This repository manages the GitRepo aggregate, including:
    - GitRepo entity
    - GitBranch entities
    - GitTag entities

    Note: Commits are now managed by the separate GitCommitRepository.
    """

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory

    @property
    def _mapper(self) -> GitMapper:
        return GitMapper()

    async def save(self, repo: GitRepo) -> GitRepo:
        """Save or update a repository with all its branches, commits, and tags."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # 1. Save or update the GitRepo entity
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
                existing_repo.num_commits = repo.num_commits
                existing_repo.num_branches = repo.num_branches
                existing_repo.num_tags = repo.num_tags
                db_repo = existing_repo
                repo.id = existing_repo.id  # Set the domain ID
            else:
                # Create new repo
                db_repo = db_entities.GitRepo(
                    sanitized_remote_uri=str(repo.sanitized_remote_uri),
                    remote_uri=str(repo.remote_uri),
                    cloned_path=repo.cloned_path,
                    last_scanned_at=repo.last_scanned_at,
                    num_commits=repo.num_commits,
                    num_branches=repo.num_branches,
                    num_tags=repo.num_tags,
                )
                session.add(db_repo)
                await session.flush()  # Get the new ID
                repo.id = db_repo.id  # Set the domain ID

            # 2. Save tracking branch
            await self._save_tracking_branch(session, repo)

            await session.flush()
            return repo



    async def _save_tracking_branch(self, session: AsyncSession, repo: GitRepo) -> None:
        """Save tracking branch if it doesn't exist."""
        if not repo.tracking_branch:
            return

        existing_tracking_branch = await session.get(
            db_entities.GitTrackingBranch, [repo.id, repo.tracking_branch.name]
        )
        if not existing_tracking_branch and repo.id is not None:
            db_tracking_branch = db_entities.GitTrackingBranch(
                repo_id=repo.id,
                name=repo.tracking_branch.name,
            )
            session.add(db_tracking_branch)


    async def get_by_id(self, repo_id: int) -> GitRepo:
        """Get repository by ID with all associated data."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_repo = await session.get(db_entities.GitRepo, repo_id)
            if not db_repo:
                raise ValueError(f"Repository with ID {repo_id} not found")

            return await self._load_complete_repo(session, db_repo)

    async def get_by_uri(self, sanitized_uri: AnyUrl) -> GitRepo:
        """Get repository by sanitized URI with all associated data."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitRepo).where(
                db_entities.GitRepo.sanitized_remote_uri == str(sanitized_uri)
            )
            db_repo = await session.scalar(stmt)
            if not db_repo:
                raise ValueError(f"Repository with URI {sanitized_uri} not found")

            return await self._load_complete_repo(session, db_repo)

    async def get_by_commit(self, commit_sha: str) -> GitRepo:
        """Get repository by commit SHA with all associated data."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Find the commit first
            stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.commit_sha == commit_sha
            )
            db_commit = await session.scalar(stmt)
            if not db_commit:
                raise ValueError(f"Commit with SHA {commit_sha} not found")

            # Get the repo
            db_repo = await session.get(db_entities.GitRepo, db_commit.repo_id)
            if not db_repo:
                raise ValueError(f"Repository with commit SHA {commit_sha} not found")

            return await self._load_complete_repo(session, db_repo)

    async def get_all(self) -> list[GitRepo]:
        """Get all repositories."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitRepo)
            db_repos = (await session.scalars(stmt)).all()

            repos = []
            for db_repo in db_repos:
                repo = await self._load_complete_repo(session, db_repo)
                repos.append(repo)

            return repos

    async def delete(self, sanitized_uri: AnyUrl) -> bool:
        """Delete only the repository entity itself.

        According to DDD principles, this repository should only delete
        the GitRepo entity it directly controls. Related entities (commits,
        branches, tags, snippets) should be deleted by their respective
        repositories before calling this method.
        """
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Find the repo
            stmt = select(db_entities.GitRepo).where(
                db_entities.GitRepo.sanitized_remote_uri == str(sanitized_uri)
            )
            db_repo = await session.scalar(stmt)
            if not db_repo:
                return False

            # Delete tracking branches first (they reference the repo)
            del_tracking_branches_stmt = delete(db_entities.GitTrackingBranch).where(
                db_entities.GitTrackingBranch.repo_id == db_repo.id
            )
            await session.execute(del_tracking_branches_stmt)

            # Delete only the repo entity itself
            # Foreign key constraints will prevent deletion if related entities exist
            del_stmt = delete(db_entities.GitRepo).where(
                db_entities.GitRepo.id == db_repo.id
            )
            await session.execute(del_stmt)
            return True


    async def _load_complete_repo(
        self, session: AsyncSession, db_repo: db_entities.GitRepo
    ) -> GitRepo:
        """Load a complete repo with all its associations."""
        all_tags = list(
            (
                await session.scalars(
                    select(db_entities.GitTag).where(
                        db_entities.GitTag.repo_id == db_repo.id
                    )
                )
            ).all()
        )
        tracking_branch = await session.scalar(
            select(db_entities.GitTrackingBranch).where(
                db_entities.GitTrackingBranch.repo_id == db_repo.id
            )
        )

        # Get tracking branch from branches table if needed
        db_tracking_branch_entity = None
        if tracking_branch:
            db_tracking_branch_entity = await session.scalar(
                select(db_entities.GitBranch).where(
                    db_entities.GitBranch.repo_id == db_repo.id,
                    db_entities.GitBranch.name == tracking_branch.name,
                )
            )

        # Get only commits needed for tags and tracking branch
        referenced_commit_shas = set()
        for tag in all_tags:
            referenced_commit_shas.add(tag.target_commit_sha)
        if db_tracking_branch_entity:
            referenced_commit_shas.add(db_tracking_branch_entity.head_commit_sha)

        # Load only the referenced commits
        referenced_commits = []
        referenced_files = []
        if referenced_commit_shas:
            referenced_commits = list(
                (
                    await session.scalars(
                        select(db_entities.GitCommit).where(
                            db_entities.GitCommit.commit_sha.in_(referenced_commit_shas)
                        )
                    )
                ).all()
            )
            referenced_files = list(
                (
                    await session.scalars(
                        select(db_entities.GitCommitFile).where(
                            db_entities.GitCommitFile.commit_sha.in_(referenced_commit_shas)
                        )
                    )
                ).all()
            )

        return self._mapper.to_domain_git_repo(
            db_repo=db_repo,
            db_tracking_branch_entity=db_tracking_branch_entity,
            db_commits=referenced_commits,
            db_tags=all_tags,
            db_commit_files=referenced_files,
            db_tracking_branch=tracking_branch,
        )
