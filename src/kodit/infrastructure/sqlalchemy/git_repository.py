"""SQLAlchemy implementation of GitRepoRepository."""

from collections.abc import Callable

from pydantic import AnyUrl
from sqlalchemy import delete, insert, select
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
                )
                session.add(db_repo)
                await session.flush()  # Get the new ID
                repo.id = db_repo.id  # Set the domain ID

            # 2. Bulk save branches
            await self._save_branches_bulk(session, repo)

            # 3. Save tracking branch
            await self._save_tracking_branch(session, repo)

            # 4. Bulk save tags
            await self._save_tags_bulk(session, repo)

            await session.flush()
            return repo


    async def _save_branches_bulk(self, session: AsyncSession, repo: GitRepo) -> None:
        """Bulk save branches using efficient batch operations."""
        if not repo.branches:
            return

        branch_names = [branch.name for branch in repo.branches]

        # Get existing branches in bulk
        existing_branches_stmt = select(db_entities.GitBranch.name).where(
            db_entities.GitBranch.repo_id == repo.id,
            db_entities.GitBranch.name.in_(branch_names),
        )
        existing_branch_names = set(
            (await session.scalars(existing_branches_stmt)).all()
        )

        # Prepare new branches for bulk insert
        new_branches = [
            {
                "repo_id": repo.id,
                "name": branch.name,
                "head_commit_sha": branch.head_commit.commit_sha,
            }
            for branch in repo.branches
            if branch.name not in existing_branch_names
        ]

        # Bulk insert new branches
        if new_branches:
            stmt = insert(db_entities.GitBranch).values(new_branches)
            await session.execute(stmt)

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

    async def _save_tags_bulk(self, session: AsyncSession, repo: GitRepo) -> None:
        """Bulk save tags using efficient batch operations."""
        if not repo.tags:
            return

        tag_names = [tag.name for tag in repo.tags]

        # Get existing tags in bulk
        existing_tags_stmt = select(db_entities.GitTag.name).where(
            db_entities.GitTag.repo_id == repo.id,
            db_entities.GitTag.name.in_(tag_names),
        )
        existing_tag_names = set((await session.scalars(existing_tags_stmt)).all())

        # Prepare new tags for bulk insert
        new_tags = [
            {
                "repo_id": repo.id,
                "name": tag.name,
                "target_commit_sha": tag.target_commit.commit_sha,
            }
            for tag in repo.tags
            if tag.name not in existing_tag_names
        ]

        # Bulk insert new tags
        if new_tags:
            stmt = insert(db_entities.GitTag).values(new_tags)
            await session.execute(stmt)

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
        """Delete a repository and all its associated data."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Find the repo
            stmt = select(db_entities.GitRepo).where(
                db_entities.GitRepo.sanitized_remote_uri == str(sanitized_uri)
            )
            db_repo = await session.scalar(stmt)
            if not db_repo:
                return False

            repo_id = db_repo.id

            # Delete in order to respect foreign keys:
            # 1. Delete commit-file associations
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

            # 3. Delete tags
            del_stmt = delete(db_entities.GitTag).where(
                db_entities.GitTag.repo_id == repo_id
            )
            await session.execute(del_stmt)

            # 4. Delete commits
            del_stmt = delete(db_entities.GitCommit).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            await session.execute(del_stmt)

            # 5. Delete the repo
            del_stmt = delete(db_entities.GitRepo).where(
                db_entities.GitRepo.id == repo_id
            )
            await session.execute(del_stmt)

            # Note: We don't delete GitFiles as they might be referenced by other repos
            return True


    async def _load_complete_repo(
        self, session: AsyncSession, db_repo: db_entities.GitRepo
    ) -> GitRepo:
        """Load a complete repo with all its associations."""
        all_branches = list(
            (
                await session.scalars(
                    select(db_entities.GitBranch).where(
                        db_entities.GitBranch.repo_id == db_repo.id
                    )
                )
            ).all()
        )
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

        # Get only commits needed for branches and tags
        referenced_commit_shas = set()
        for branch in all_branches:
            referenced_commit_shas.add(branch.head_commit_sha)
        for tag in all_tags:
            referenced_commit_shas.add(tag.target_commit_sha)

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
            db_branches=all_branches,
            db_commits=referenced_commits,
            db_tags=all_tags,
            db_commit_files=referenced_files,
            db_tracking_branch=tracking_branch,
        )
