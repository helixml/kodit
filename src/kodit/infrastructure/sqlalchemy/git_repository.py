"""SQLAlchemy implementation of GitRepoRepository."""

from collections.abc import Callable

from pydantic import AnyUrl
from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit, GitFile, GitRepo
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

    This repository manages the complete GitRepo aggregate, including:
    - GitRepo entity
    - GitBranch entities
    - GitCommit entities
    - GitTag entities
    - GitFile entities and associations
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

            # 2. Save commits

            for commit in repo.commits:
                existing_commit = await session.get(
                    db_entities.GitCommit, commit.commit_sha
                )
                if not existing_commit:
                    db_commit = db_entities.GitCommit(
                        commit_sha=commit.commit_sha,
                        repo_id=repo.id,
                        date=commit.date,
                        message=commit.message,
                        parent_commit_sha=commit.parent_commit_sha,
                        author=commit.author,
                    )
                    session.add(db_commit)
            await session.flush()

            # 3. Save files
            for commit in repo.commits:
                for file in commit.files:
                    existing_file = await session.get(
                        db_entities.GitCommitFile, (commit.commit_sha, file.path)
                    )
                    if not existing_file:
                        db_file = db_entities.GitCommitFile(
                            commit_sha=commit.commit_sha,
                            path=file.path,
                            blob_sha=file.blob_sha,
                            extension=file.extension,
                            mime_type=file.mime_type,
                            size=file.size,
                            created_at=file.created_at,
                        )
                        session.add(db_file)
            # No need for a flush since nothing relies on blob IDs

            # 5. Save branches
            for branch in repo.branches:
                existing_branch = await session.get(
                    db_entities.GitBranch, (repo.id, branch.name)
                )
                if not existing_branch:
                    db_branch = db_entities.GitBranch(
                        repo_id=repo.id,
                        name=branch.name,
                        head_commit_sha=branch.head_commit.commit_sha,
                    )
                    session.add(db_branch)

            # 6. Save tracking branch
            if repo.tracking_branch:
                existing_tracking_branch = await session.get(
                    db_entities.GitTrackingBranch, repo.id
                )
                if not existing_tracking_branch:
                    db_tracking_branch = db_entities.GitTrackingBranch(
                        repo_id=repo.id,
                        name=repo.tracking_branch.name,
                    )
                    session.add(db_tracking_branch)

            # 7. Save tags
            for tag in repo.tags:
                existing_tag = await session.get(
                    db_entities.GitTag, (repo.id, tag.name)
                )
                if not existing_tag:
                    db_tag = db_entities.GitTag(
                        repo_id=repo.id,
                        name=tag.name,
                        target_commit_sha=tag.target_commit.commit_sha,
                    )
                    session.add(db_tag)
            await session.flush()
            return repo

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

    async def get_commit_by_sha(self, commit_sha: str) -> GitCommit:
        """Get a specific commit by its SHA across all repositories."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get the commit
            stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.commit_sha == commit_sha
            )
            db_commit = await session.scalar(stmt)
            if not db_commit:
                raise ValueError(f"Commit with SHA {commit_sha} not found")

            # Get associated files
            files_stmt = select(db_entities.GitCommitFile).where(
                db_entities.GitCommitFile.commit_sha == commit_sha
            )
            db_files = (await session.scalars(files_stmt)).all()

            domain_files = []
            for db_file in db_files:
                domain_file = GitFile(
                    blob_sha=db_file.blob_sha,
                    path=db_file.path,
                    mime_type=db_file.mime_type,
                    size=db_file.size,
                    extension=db_file.extension,
                    created_at=db_file.created_at,
                )
                domain_files.append(domain_file)

            return GitCommit(
                commit_sha=db_commit.commit_sha,
                date=db_commit.date,
                message=db_commit.message,
                parent_commit_sha=db_commit.parent_commit_sha,
                files=domain_files,
                author=db_commit.author,
            )

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
        all_commits = list(
            (
                await session.scalars(
                    select(db_entities.GitCommit).where(
                        db_entities.GitCommit.repo_id == db_repo.id
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
        all_files = list(
            (
                await session.scalars(
                    select(db_entities.GitCommitFile).where(
                        db_entities.GitCommitFile.commit_sha.in_(
                            [commit.commit_sha for commit in all_commits]
                        )
                    )
                )
            ).all()
        )
        tracking_branch = await session.get(db_entities.GitTrackingBranch, db_repo.id)
        return self._mapper.to_domain_git_repo(
            db_repo=db_repo,
            db_branches=all_branches,
            db_commits=all_commits,
            db_tags=all_tags,
            db_commit_files=all_files,
            db_tracking_branch=tracking_branch,
        )
