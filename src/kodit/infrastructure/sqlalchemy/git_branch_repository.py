"""SQLAlchemy implementation of GitBranchRepository."""

from collections.abc import Callable

from sqlalchemy import delete, func, insert, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitBranch, GitCommit
from kodit.domain.protocols import GitBranchRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_git_branch_repository(
    session_factory: Callable[[], AsyncSession],
) -> GitBranchRepository:
    """Create a git branch repository."""
    return SqlAlchemyGitBranchRepository(session_factory=session_factory)


class SqlAlchemyGitBranchRepository(GitBranchRepository):
    """SQLAlchemy implementation of GitBranchRepository."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory

    async def get_by_name(self, branch_name: str, repo_id: int) -> GitBranch:
        """Get a branch by name and repository ID."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get the branch
            stmt = select(db_entities.GitBranch).where(
                db_entities.GitBranch.name == branch_name,
                db_entities.GitBranch.repo_id == repo_id,
            )
            db_branch = await session.scalar(stmt)
            if not db_branch:
                raise ValueError(f"Branch {branch_name} not found in repo {repo_id}")

            # Get the head commit
            commit_stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.commit_sha == db_branch.head_commit_sha
            )
            db_commit = await session.scalar(commit_stmt)
            if not db_commit:
                raise ValueError(f"Head commit {db_branch.head_commit_sha} not found")

            # Get files for the head commit
            files_stmt = select(db_entities.GitCommitFile).where(
                db_entities.GitCommitFile.commit_sha == db_branch.head_commit_sha
            )
            db_files = (await session.scalars(files_stmt)).all()

            from kodit.domain.entities.git import GitFile

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

            head_commit = GitCommit(
                commit_sha=db_commit.commit_sha,
                date=db_commit.date,
                message=db_commit.message,
                parent_commit_sha=db_commit.parent_commit_sha,
                files=domain_files,
                author=db_commit.author,
                created_at=db_commit.created_at,
                updated_at=db_commit.updated_at,
            )

            return GitBranch(
                repo_id=db_branch.repo_id,
                name=db_branch.name,
                head_commit=head_commit,
                created_at=db_branch.created_at,
                updated_at=db_branch.updated_at,
            )

    async def get_by_repo_id(self, repo_id: int) -> list[GitBranch]:
        """Get all branches for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get all branches for the repo
            branches_stmt = select(db_entities.GitBranch).where(
                db_entities.GitBranch.repo_id == repo_id
            )
            db_branches = (await session.scalars(branches_stmt)).all()

            if not db_branches:
                return []

            commit_shas = [branch.head_commit_sha for branch in db_branches]

            # Get all head commits for these branches
            commits_stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.commit_sha.in_(commit_shas)
            )
            db_commits = (await session.scalars(commits_stmt)).all()

            # Get all files for these commits
            files_stmt = select(db_entities.GitCommitFile).where(
                db_entities.GitCommitFile.commit_sha.in_(commit_shas)
            )
            db_files = (await session.scalars(files_stmt)).all()

            # Group files by commit SHA
            from kodit.domain.entities.git import GitFile

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

            # Create commit lookup
            commits_by_sha = {commit.commit_sha: commit for commit in db_commits}

            # Create domain branches
            domain_branches = []
            for db_branch in db_branches:
                db_commit = commits_by_sha.get(db_branch.head_commit_sha)
                if not db_commit:
                    continue

                commit_files = files_by_commit.get(db_branch.head_commit_sha, [])
                head_commit = GitCommit(
                    commit_sha=db_commit.commit_sha,
                    date=db_commit.date,
                    message=db_commit.message,
                    parent_commit_sha=db_commit.parent_commit_sha,
                    files=commit_files,
                    author=db_commit.author,
                    created_at=db_commit.created_at,
                    updated_at=db_commit.updated_at,
                )

                domain_branch = GitBranch(
                    repo_id=db_branch.repo_id,
                    name=db_branch.name,
                    head_commit=head_commit,
                    created_at=db_branch.created_at,
                    updated_at=db_branch.updated_at,
                )
                domain_branches.append(domain_branch)

            return domain_branches

    async def save(self, branch: GitBranch, repo_id: int) -> GitBranch:
        """Save a branch to a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Set repo_id on the branch
            branch.repo_id = repo_id

            # Check if branch already exists
            existing_branch = await session.get(
                db_entities.GitBranch, (repo_id, branch.name)
            )

            if existing_branch:
                # Update existing branch
                existing_branch.head_commit_sha = branch.head_commit.commit_sha
                if branch.updated_at:
                    existing_branch.updated_at = branch.updated_at
            else:
                # Create new branch
                db_branch = db_entities.GitBranch(
                    repo_id=repo_id,
                    name=branch.name,
                    head_commit_sha=branch.head_commit.commit_sha,
                )
                session.add(db_branch)

            return branch

    async def save_bulk(self, branches: list[GitBranch], repo_id: int) -> None:
        """Bulk save branches to a repository."""
        if not branches:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            [(repo_id, branch.name) for branch in branches]

            # Get existing branches in bulk
            existing_branches_stmt = select(db_entities.GitBranch).where(
                db_entities.GitBranch.repo_id == repo_id,
                db_entities.GitBranch.name.in_([branch.name for branch in branches]),
            )
            existing_branches = (await session.scalars(existing_branches_stmt)).all()
            existing_branch_names = {branch.name for branch in existing_branches}

            # Update existing branches
            for existing_branch in existing_branches:
                for branch in branches:
                    if (
                        branch.name == existing_branch.name
                        and existing_branch.head_commit_sha
                        != branch.head_commit.commit_sha
                    ):
                        existing_branch.head_commit_sha = branch.head_commit.commit_sha
                        break

            # Prepare new branches for bulk insert
            new_branches_data = [
                {
                    "repo_id": repo_id,
                    "name": branch.name,
                    "head_commit_sha": branch.head_commit.commit_sha,
                }
                for branch in branches
                if branch.name not in existing_branch_names
            ]

            # Bulk insert new branches in chunks to avoid parameter limits
            if new_branches_data:
                chunk_size = 1000  # Conservative chunk size for parameter limits
                for i in range(0, len(new_branches_data), chunk_size):
                    chunk = new_branches_data[i : i + chunk_size]
                    stmt = insert(db_entities.GitBranch).values(chunk)
                    await session.execute(stmt)

    async def exists(self, branch_name: str, repo_id: int) -> bool:
        """Check if a branch exists."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitBranch.name).where(
                db_entities.GitBranch.name == branch_name,
                db_entities.GitBranch.repo_id == repo_id,
            )
            result = await session.scalar(stmt)
            return result is not None

    async def delete_by_repo_id(self, repo_id: int) -> None:
        """Delete all branches for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Delete branches
            del_branches_stmt = delete(db_entities.GitBranch).where(
                db_entities.GitBranch.repo_id == repo_id
            )
            await session.execute(del_branches_stmt)

    async def count_by_repo_id(self, repo_id: int) -> int:
        """Count the number of branches for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(func.count()).select_from(db_entities.GitBranch).where(
                db_entities.GitBranch.repo_id == repo_id
            )
            result = await session.scalar(stmt)
            return result or 0
