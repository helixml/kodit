"""SQLAlchemy implementation of GitRepoRepository."""

from collections.abc import Callable
from typing import Any

from pydantic import AnyUrl
from sqlalchemy import delete, insert, select
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

            # 2. Bulk save commits
            await self._save_commits_bulk(session, repo)

            # 3. Bulk save files
            await self._save_files_bulk(session, repo)

            # 4. Bulk save branches
            await self._save_branches_bulk(session, repo)

            # 5. Save tracking branch
            await self._save_tracking_branch(session, repo)

            # 6. Bulk save tags
            await self._save_tags_bulk(session, repo)

            await session.flush()
            return repo

    async def _save_commits_bulk(self, session: AsyncSession, repo: GitRepo) -> None:
        """Bulk save commits using efficient batch operations."""
        if not repo.commits:
            return

        commit_shas = [commit.commit_sha for commit in repo.commits]

        # Get existing commits in bulk
        existing_commits_stmt = select(db_entities.GitCommit.commit_sha).where(
            db_entities.GitCommit.commit_sha.in_(commit_shas)
        )
        existing_commit_shas = set((await session.scalars(existing_commits_stmt)).all())

        # Prepare new commits for bulk insert
        new_commits = [
            {
                "commit_sha": commit.commit_sha,
                "repo_id": repo.id,
                "date": commit.date,
                "message": commit.message,
                "parent_commit_sha": commit.parent_commit_sha,
                "author": commit.author,
            }
            for commit in repo.commits
            if commit.commit_sha not in existing_commit_shas
        ]

        # Bulk insert new commits
        if new_commits:
            stmt = insert(db_entities.GitCommit).values(new_commits)
            await session.execute(stmt)

    async def _save_files_bulk(self, session: AsyncSession, repo: GitRepo) -> None:
        """Bulk save files using efficient batch operations."""
        if not repo.commits:
            return

        file_identifiers = [
            (commit.commit_sha, file.path)
            for commit in repo.commits
            for file in commit.files
        ]

        if not file_identifiers:
            return

        existing_file_keys = await self._get_existing_file_keys(
            session, file_identifiers
        )
        new_files = self._prepare_new_files(repo, existing_file_keys)
        await self._bulk_insert_files(session, new_files)

    async def _get_existing_file_keys(
        self, session: AsyncSession, file_identifiers: list[tuple[str, str]]
    ) -> set[tuple[str, str]]:
        """Get existing file keys in chunks to avoid SQL parameter limits."""
        chunk_size = 1000
        existing_file_keys = set()

        for i in range(0, len(file_identifiers), chunk_size):
            chunk = file_identifiers[i : i + chunk_size]
            commit_shas = [item[0] for item in chunk]
            paths = [item[1] for item in chunk]

            existing_files_stmt = select(
                db_entities.GitCommitFile.commit_sha, db_entities.GitCommitFile.path
            ).where(
                db_entities.GitCommitFile.commit_sha.in_(commit_shas),
                db_entities.GitCommitFile.path.in_(paths),
            )

            chunk_existing = await session.execute(existing_files_stmt)
            for commit_sha, path in chunk_existing:
                existing_file_keys.add((commit_sha, path))

        return existing_file_keys

    def _prepare_new_files(
        self, repo: GitRepo, existing_file_keys: set[tuple[str, str]]
    ) -> list[dict[str, Any]]:
        """Prepare new files for bulk insert."""
        new_files = []
        for commit in repo.commits:
            for file in commit.files:
                file_key = (commit.commit_sha, file.path)
                if file_key not in existing_file_keys:
                    new_files.append(
                        {
                            "commit_sha": commit.commit_sha,
                            "repo_id": repo.id,
                            "path": file.path,
                            "blob_sha": file.blob_sha,
                            "extension": file.extension,
                            "mime_type": file.mime_type,
                            "size": file.size,
                            "created_at": file.created_at,
                        }
                    )
        return new_files

    async def _bulk_insert_files(
        self, session: AsyncSession, new_files: list[dict[str, Any]]
    ) -> None:
        """Bulk insert new files in chunks."""
        if not new_files:
            return

        chunk_size = 1000
        for i in range(0, len(new_files), chunk_size):
            chunk = new_files[i : i + chunk_size]
            stmt = insert(db_entities.GitCommitFile).values(chunk)
            await session.execute(stmt)

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
                        db_entities.GitCommitFile.repo_id == db_repo.id
                    )
                )
            ).all()
        )
        tracking_branch = await session.scalar(
            select(db_entities.GitTrackingBranch).where(
                db_entities.GitTrackingBranch.repo_id == db_repo.id
            )
        )
        return self._mapper.to_domain_git_repo(
            db_repo=db_repo,
            db_branches=all_branches,
            db_commits=all_commits,
            db_tags=all_tags,
            db_commit_files=all_files,
            db_tracking_branch=tracking_branch,
        )
