"""SQLAlchemy implementation of GitRepoRepository."""

from collections.abc import Callable

from pydantic import AnyUrl
from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import GitCommit, GitRepo
from kodit.domain.protocols import GitRepoRepository
from kodit.infrastructure.mappers.git_mapper import GitMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_git_repo_repository(
    session_factory: Callable[[], AsyncSession],
) -> GitRepoRepository:
    """Create a git repository."""
    uow = SqlAlchemyUnitOfWork(session_factory=session_factory)
    return SqlAlchemyGitRepoRepository(uow)


class SqlAlchemyGitRepoRepository(GitRepoRepository):
    """SQLAlchemy implementation of GitRepoRepository.

    This repository manages the complete GitRepo aggregate, including:
    - GitRepo entity
    - GitBranch entities
    - GitCommit entities
    - GitTag entities
    - GitFile entities and associations
    """

    def __init__(self, uow: SqlAlchemyUnitOfWork) -> None:
        """Initialize the repository."""
        self.uow = uow

    @property
    def _mapper(self) -> GitMapper:
        return GitMapper()

    @property
    def _session(self) -> AsyncSession:
        if self.uow.session is None:
            raise RuntimeError("UnitOfWork must be used within async context")
        return self.uow.session

    async def save(self, repo: GitRepo) -> None:  # noqa: C901, PLR0912, PLR0915
        """Save or update a repository with all its branches, commits, and tags."""
        async with self.uow:
            # 1. Save or update the GitRepo entity
            if repo.id:
                # Update existing repo
                existing_repo = await self._session.get(db_entities.GitRepo, repo.id)
                if existing_repo:
                    existing_repo.sanitized_remote_uri = str(repo.sanitized_remote_uri)
                    existing_repo.remote_uri = str(repo.remote_uri)
                    existing_repo.cloned_path = str(repo.cloned_path)
                    existing_repo.last_scanned_at = repo.last_scanned_at
                    existing_repo.total_unique_commits = repo.total_unique_commits
                    db_repo = existing_repo
                else:
                    raise ValueError(f"Repository with ID {repo.id} not found")
            else:
                # Check if repo exists by URI (for new repos from domain)
                existing_repo_stmt = select(db_entities.GitRepo).where(
                    db_entities.GitRepo.sanitized_remote_uri
                    == str(repo.sanitized_remote_uri)
                )
                existing_repo = await self._session.scalar(existing_repo_stmt)

                if existing_repo:
                    # Update existing repo found by URI
                    existing_repo.remote_uri = str(repo.remote_uri)
                    existing_repo.cloned_path = str(repo.cloned_path)
                    existing_repo.last_scanned_at = repo.last_scanned_at
                    existing_repo.total_unique_commits = repo.total_unique_commits
                    db_repo = existing_repo
                    repo.id = existing_repo.id  # Set the domain ID
                else:
                    # Create new repo
                    db_repo = db_entities.GitRepo(
                        sanitized_remote_uri=str(repo.sanitized_remote_uri),
                        remote_uri=str(repo.remote_uri),
                        cloned_path=str(repo.cloned_path),
                        last_scanned_at=repo.last_scanned_at,
                        total_unique_commits=repo.total_unique_commits,
                    )
                    self._session.add(db_repo)
                    await self._session.flush()  # Get the new ID
                    repo.id = db_repo.id  # Set the domain ID

            await self._session.flush()
            repo_id = db_repo.id

            # 2. Save files (they don't have foreign keys to repo, so save first)
            all_files = {}
            for commit in repo.commits:
                for file in commit.files:
                    if file.blob_sha not in all_files:
                        all_files[file.blob_sha] = file

            for file in all_files.values():
                existing_file = await self._session.get(
                    db_entities.GitFile, file.blob_sha
                )
                if not existing_file:
                    db_file = db_entities.GitFile(
                        blob_sha=file.blob_sha,
                        path=file.path,
                        mime_type=file.mime_type,
                        size=file.size,
                    )
                    self._session.add(db_file)

            await self._session.flush()

            # 3. Save commits
            for commit in repo.commits:
                existing_commit = await self._session.get(
                    db_entities.GitCommit, commit.commit_sha
                )
                if not existing_commit:
                    db_commit = db_entities.GitCommit(
                        commit_sha=commit.commit_sha,
                        repo_id=repo_id,
                        date=commit.date,
                        message=commit.message,
                        parent_commit_sha=commit.parent_commit_sha,
                        author=commit.author,
                    )
                    self._session.add(db_commit)

            await self._session.flush()

            # 4. Save commit-file associations
            for commit in repo.commits:
                # Delete existing associations for this commit
                stmt = delete(db_entities.GitCommitFile).where(
                    db_entities.GitCommitFile.commit_sha == commit.commit_sha
                )
                await self._session.execute(stmt)

                # Add new associations
                for file in commit.files:
                    db_commit_file = db_entities.GitCommitFile(
                        commit_sha=commit.commit_sha,
                        file_blob_sha=file.blob_sha,
                    )
                    self._session.add(db_commit_file)

            await self._session.flush()

            # 5. Save branches
            # Delete existing branches for this repo
            stmt = delete(db_entities.GitBranch).where(
                db_entities.GitBranch.repo_id == repo_id
            )
            await self._session.execute(stmt)

            # Add new branches
            for branch in repo.branches:
                db_branch = db_entities.GitBranch(
                    repo_id=repo_id,
                    name=branch.name,
                    head_commit_sha=branch.head_commit.commit_sha,
                )
                self._session.add(db_branch)

            await self._session.flush()

            # 6. Save tags
            # Delete existing tags for this repo
            stmt = delete(db_entities.GitTag).where(
                db_entities.GitTag.repo_id == repo_id
            )
            await self._session.execute(stmt)

            # Add new tags
            for tag in repo.tags:
                db_tag = db_entities.GitTag(
                    repo_id=repo_id,
                    name=tag.name,
                    target_commit_sha=tag.target_commit_sha,
                )
                self._session.add(db_tag)

    async def get_by_id(self, repo_id: int) -> GitRepo | None:
        """Get repository by ID with all associated data."""
        async with self.uow:
            db_repo = await self._session.get(db_entities.GitRepo, repo_id)
            if not db_repo:
                return None

            return await self._load_complete_repo(db_repo)

    async def get_by_uri(self, sanitized_uri: AnyUrl) -> GitRepo | None:
        """Get repository by sanitized URI with all associated data."""
        async with self.uow:
            stmt = select(db_entities.GitRepo).where(
                db_entities.GitRepo.sanitized_remote_uri == str(sanitized_uri)
            )
            db_repo = await self._session.scalar(stmt)
            if not db_repo:
                return None

            return await self._load_complete_repo(db_repo)

    async def get_by_commit(self, commit_sha: str) -> GitRepo | None:
        """Get repository by commit SHA with all associated data."""
        async with self.uow:
            # Find the commit first
            stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.commit_sha == commit_sha
            )
            db_commit = await self._session.scalar(stmt)
            if not db_commit:
                return None

            # Get the repo
            db_repo = await self._session.get(db_entities.GitRepo, db_commit.repo_id)
            if not db_repo:
                return None

            return await self._load_complete_repo(db_repo)

    async def get_all(self) -> list[GitRepo]:
        """Get all repositories."""
        async with self.uow:
            stmt = select(db_entities.GitRepo)
            db_repos = (await self._session.scalars(stmt)).all()

            repos = []
            for db_repo in db_repos:
                repo = await self._load_complete_repo(db_repo)
                repos.append(repo)

            return repos

    async def delete(self, sanitized_uri: AnyUrl) -> bool:
        """Delete a repository and all its associated data."""
        async with self.uow:
            # Find the repo
            stmt = select(db_entities.GitRepo).where(
                db_entities.GitRepo.sanitized_remote_uri == str(sanitized_uri)
            )
            db_repo = await self._session.scalar(stmt)
            if not db_repo:
                return False

            repo_id = db_repo.id

            # Delete in order to respect foreign keys:
            # 1. Delete commit-file associations
            commit_shas_stmt = select(db_entities.GitCommit.commit_sha).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            commit_shas = (await self._session.scalars(commit_shas_stmt)).all()

            for commit_sha in commit_shas:
                del_stmt = delete(db_entities.GitCommitFile).where(
                    db_entities.GitCommitFile.commit_sha == commit_sha
                )
                await self._session.execute(del_stmt)

            # 2. Delete branches
            del_stmt = delete(db_entities.GitBranch).where(
                db_entities.GitBranch.repo_id == repo_id
            )
            await self._session.execute(del_stmt)

            # 3. Delete tags
            del_stmt = delete(db_entities.GitTag).where(
                db_entities.GitTag.repo_id == repo_id
            )
            await self._session.execute(del_stmt)

            # 4. Delete commits
            del_stmt = delete(db_entities.GitCommit).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            await self._session.execute(del_stmt)

            # 5. Delete the repo
            del_stmt = delete(db_entities.GitRepo).where(
                db_entities.GitRepo.id == repo_id
            )
            await self._session.execute(del_stmt)

            # Note: We don't delete GitFiles as they might be referenced by other repos
            return True

    async def get_commit_by_sha(self, commit_sha: str) -> GitCommit | None:
        """Get a specific commit by its SHA across all repositories."""
        async with self.uow:
            # Get the commit
            stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.commit_sha == commit_sha
            )
            db_commit = await self._session.scalar(stmt)
            if not db_commit:
                return None

            # Get associated files
            files_stmt = (
                select(db_entities.GitFile)
                .join(
                    db_entities.GitCommitFile,
                    db_entities.GitFile.blob_sha
                    == db_entities.GitCommitFile.file_blob_sha,
                )
                .where(db_entities.GitCommitFile.commit_sha == commit_sha)
            )
            db_files = (await self._session.scalars(files_stmt)).all()

            # Convert to domain entities
            from kodit.domain.entities import GitFile

            domain_files = []
            for db_file in db_files:
                domain_file = GitFile(
                    blob_sha=db_file.blob_sha,
                    path=db_file.path,
                    mime_type=db_file.mime_type,
                    size=db_file.size,
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

    async def _load_complete_repo(self, db_repo: db_entities.GitRepo) -> GitRepo:
        """Load a complete repo with all its associations."""
        repo_id = db_repo.id

        # Load branches
        branches_stmt = select(db_entities.GitBranch).where(
            db_entities.GitBranch.repo_id == repo_id
        )
        db_branches = (await self._session.scalars(branches_stmt)).all()

        # Load commits
        commits_stmt = select(db_entities.GitCommit).where(
            db_entities.GitCommit.repo_id == repo_id
        )
        db_commits = (await self._session.scalars(commits_stmt)).all()

        # Load tags
        tags_stmt = select(db_entities.GitTag).where(
            db_entities.GitTag.repo_id == repo_id
        )
        db_tags = (await self._session.scalars(tags_stmt)).all()

        # Load all files for all commits in this repo
        files_stmt = (
            select(db_entities.GitFile)
            .join(
                db_entities.GitCommitFile,
                db_entities.GitFile.blob_sha == db_entities.GitCommitFile.file_blob_sha,
            )
            .join(
                db_entities.GitCommit,
                db_entities.GitCommitFile.commit_sha
                == db_entities.GitCommit.commit_sha,
            )
            .where(db_entities.GitCommit.repo_id == repo_id)
        )
        db_files = (await self._session.scalars(files_stmt)).all()

        # Load commit-file associations
        commit_files_stmt = (
            select(
                db_entities.GitCommitFile.commit_sha,
                db_entities.GitCommitFile.file_blob_sha,
            )
            .join(
                db_entities.GitCommit,
                db_entities.GitCommitFile.commit_sha
                == db_entities.GitCommit.commit_sha,
            )
            .where(db_entities.GitCommit.repo_id == repo_id)
        )
        commit_file_pairs = (await self._session.execute(commit_files_stmt)).all()

        # Build commit -> files mapping
        commit_files_map: dict[str, list[str]] = {}
        for commit_sha, file_blob_sha in commit_file_pairs:
            if commit_sha not in commit_files_map:
                commit_files_map[commit_sha] = []
            commit_files_map[commit_sha].append(file_blob_sha)

        # Use mapper to convert to domain entity
        # For tracking branch, we'll use the first branch as fallback
        tracking_branch_name = db_branches[0].name if db_branches else "main"

        return self._mapper.to_domain_git_repo(
            db_repo=db_repo,
            db_branches=list(db_branches),
            db_commits=list(db_commits),
            db_tags=list(db_tags),
            db_files=list(db_files),
            commit_files_map=commit_files_map,
            tracking_branch_name=tracking_branch_name,
        )
