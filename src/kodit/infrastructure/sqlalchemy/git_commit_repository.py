"""SQLAlchemy implementation of GitCommitRepository."""

from collections.abc import Callable

from sqlalchemy import delete, func, insert, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit, GitFile
from kodit.domain.protocols import GitCommitRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_git_commit_repository(
    session_factory: Callable[[], AsyncSession],
) -> GitCommitRepository:
    """Create a git commit repository."""
    return SqlAlchemyGitCommitRepository(session_factory=session_factory)


class SqlAlchemyGitCommitRepository(GitCommitRepository):
    """SQLAlchemy implementation of GitCommitRepository."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory

    async def get_by_sha(self, commit_sha: str) -> GitCommit:
        """Get a commit by its SHA."""
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

    async def get_by_repo_id(self, repo_id: int) -> list[GitCommit]:
        """Get all commits for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get all commits for the repo
            commits_stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            db_commits = (await session.scalars(commits_stmt)).all()

            if not db_commits:
                return []

            commit_shas = [commit.commit_sha for commit in db_commits]

            # Get all files for these commits
            files_stmt = select(db_entities.GitCommitFile).where(
                db_entities.GitCommitFile.commit_sha.in_(commit_shas)
            )
            db_files = (await session.scalars(files_stmt)).all()

            # Group files by commit SHA
            files_by_commit: dict[str, list[GitFile]] = {}
            for db_file in db_files:
                if db_file.commit_sha not in files_by_commit:
                    files_by_commit[db_file.commit_sha] = []

                domain_file = GitFile(
                    blob_sha=db_file.blob_sha,
                    path=db_file.path,
                    mime_type=db_file.mime_type,
                    size=db_file.size,
                    extension=db_file.extension,
                    created_at=db_file.created_at,
                )
                files_by_commit[db_file.commit_sha].append(domain_file)

            # Create domain commits
            domain_commits = []
            for db_commit in db_commits:
                commit_files = files_by_commit.get(db_commit.commit_sha, [])
                domain_commit = GitCommit(
                    commit_sha=db_commit.commit_sha,
                    date=db_commit.date,
                    message=db_commit.message,
                    parent_commit_sha=db_commit.parent_commit_sha,
                    files=commit_files,
                    author=db_commit.author,
                )
                domain_commits.append(domain_commit)

            return domain_commits

    async def save(self, commit: GitCommit, repo_id: int) -> GitCommit:
        """Save a commit to a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Check if commit already exists
            existing_commit = await session.get(
                db_entities.GitCommit, commit.commit_sha
            )

            if not existing_commit:
                # Create new commit
                db_commit = db_entities.GitCommit(
                    commit_sha=commit.commit_sha,
                    repo_id=repo_id,
                    date=commit.date,
                    message=commit.message,
                    parent_commit_sha=commit.parent_commit_sha,
                    author=commit.author,
                )
                session.add(db_commit)
                await session.flush()

                # Save associated files
                await self._save_commit_files(session, commit)

            return commit

    async def save_bulk(self, commits: list[GitCommit], repo_id: int) -> None:
        """Bulk save commits to a repository."""
        if not commits:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            commit_shas = [commit.commit_sha for commit in commits]

            # Get existing commits in bulk
            existing_commits_stmt = select(db_entities.GitCommit.commit_sha).where(
                db_entities.GitCommit.commit_sha.in_(commit_shas)
            )
            existing_commit_shas = set(
                (await session.scalars(existing_commits_stmt)).all()
            )

            # Prepare new commits for bulk insert
            new_commits_data = []
            new_commits_objects = []
            for commit in commits:
                if commit.commit_sha not in existing_commit_shas:
                    new_commits_data.append({
                        "commit_sha": commit.commit_sha,
                        "repo_id": repo_id,
                        "date": commit.date,
                        "message": commit.message,
                        "parent_commit_sha": commit.parent_commit_sha,
                        "author": commit.author,
                    })
                    new_commits_objects.append(commit)

            # Bulk insert new commits in chunks to avoid parameter limits
            if new_commits_data:
                chunk_size = 1000  # Conservative chunk size for parameter limits
                for i in range(0, len(new_commits_data), chunk_size):
                    chunk = new_commits_data[i : i + chunk_size]
                    stmt = insert(db_entities.GitCommit).values(chunk)
                    await session.execute(stmt)

                # Bulk save files for new commits
                await self._save_commits_files_bulk(session, new_commits_objects)

    async def exists(self, commit_sha: str) -> bool:
        """Check if a commit exists."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitCommit.commit_sha).where(
                db_entities.GitCommit.commit_sha == commit_sha
            )
            result = await session.scalar(stmt)
            return result is not None

    async def delete_by_repo_id(self, repo_id: int) -> None:
        """Delete all commits for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get all commit SHAs for this repo
            commit_shas_stmt = select(db_entities.GitCommit.commit_sha).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            commit_shas = (await session.scalars(commit_shas_stmt)).all()

            # Delete snippet file associations first (they reference commit files)
            for commit_sha in commit_shas:
                del_snippet_files_stmt = delete(db_entities.SnippetV2File).where(
                    db_entities.SnippetV2File.commit_sha == commit_sha
                )
                await session.execute(del_snippet_files_stmt)

            # Delete commit files second (foreign key constraint)
            for commit_sha in commit_shas:
                del_files_stmt = delete(db_entities.GitCommitFile).where(
                    db_entities.GitCommitFile.commit_sha == commit_sha
                )
                await session.execute(del_files_stmt)

            # Delete commits
            del_commits_stmt = delete(db_entities.GitCommit).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            await session.execute(del_commits_stmt)

    async def count_by_repo_id(self, repo_id: int) -> int:
        """Count the number of commits for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(func.count()).select_from(db_entities.GitCommit).where(
                db_entities.GitCommit.repo_id == repo_id
            )
            result = await session.scalar(stmt)
            return result or 0

    async def _save_commit_files(
        self, session: AsyncSession, commit: GitCommit
    ) -> None:
        """Save files for a single commit."""
        if not commit.files:
            return

        # Check which files already exist
        existing_files_stmt = select(
            db_entities.GitCommitFile.commit_sha,
            db_entities.GitCommitFile.path
        ).where(
            db_entities.GitCommitFile.commit_sha == commit.commit_sha
        )
        existing_file_keys = set(await session.execute(existing_files_stmt))

        # Prepare new files for insert
        new_files = []
        for file in commit.files:
            file_key = (commit.commit_sha, file.path)
            if file_key not in existing_file_keys:
                new_files.append({
                    "commit_sha": commit.commit_sha,
                    "path": file.path,
                    "blob_sha": file.blob_sha,
                    "extension": file.extension,
                    "mime_type": file.mime_type,
                    "size": file.size,
                    "created_at": file.created_at,
                })

        # Bulk insert new files in chunks to avoid parameter limits
        if new_files:
            chunk_size = 1000  # Conservative chunk size for parameter limits
            for i in range(0, len(new_files), chunk_size):
                chunk = new_files[i : i + chunk_size]
                stmt = insert(db_entities.GitCommitFile).values(chunk)
                await session.execute(stmt)

    async def _save_commits_files_bulk(
        self, session: AsyncSession, commits: list[GitCommit]
    ) -> None:
        """Bulk save files for multiple commits."""
        all_file_identifiers = [
            (commit.commit_sha, file.path)
            for commit in commits
            for file in commit.files
        ]

        if not all_file_identifiers:
            return

        # Get existing files in chunks to avoid SQL parameter limits
        existing_file_keys = await self._get_existing_file_keys_bulk(
            session, all_file_identifiers
        )

        # Prepare new files for bulk insert
        new_files = []
        for commit in commits:
            for file in commit.files:
                file_key = (commit.commit_sha, file.path)
                if file_key not in existing_file_keys:
                    new_files.append({
                        "commit_sha": commit.commit_sha,
                        "path": file.path,
                        "blob_sha": file.blob_sha,
                        "extension": file.extension,
                        "mime_type": file.mime_type,
                        "size": file.size,
                        "created_at": file.created_at,
                    })

        # Bulk insert new files in chunks
        if new_files:
            chunk_size = 1000
            for i in range(0, len(new_files), chunk_size):
                chunk = new_files[i : i + chunk_size]
                stmt = insert(db_entities.GitCommitFile).values(chunk)
                await session.execute(stmt)

    async def _get_existing_file_keys_bulk(
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
