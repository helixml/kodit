"""SQLAlchemy implementation of GitTagRepository."""

from collections.abc import Callable

from sqlalchemy import delete, func, insert, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit, GitFile, GitTag
from kodit.domain.protocols import GitTagRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_git_tag_repository(
    session_factory: Callable[[], AsyncSession],
) -> GitTagRepository:
    """Create a git tag repository."""
    return SqlAlchemyGitTagRepository(session_factory=session_factory)


class SqlAlchemyGitTagRepository(GitTagRepository):
    """SQLAlchemy implementation of GitTagRepository."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory

    async def get_by_name(self, tag_name: str, repo_id: int) -> GitTag:
        """Get a tag by name and repository ID."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get the tag
            stmt = select(db_entities.GitTag).where(
                db_entities.GitTag.name == tag_name,
                db_entities.GitTag.repo_id == repo_id,
            )
            db_tag = await session.scalar(stmt)
            if not db_tag:
                raise ValueError(f"Tag {tag_name} not found in repo {repo_id}")

            # Get the target commit
            commit_stmt = select(db_entities.GitCommit).where(
                db_entities.GitCommit.commit_sha == db_tag.target_commit_sha
            )
            db_commit = await session.scalar(commit_stmt)
            if not db_commit:
                raise ValueError(f"Target commit {db_tag.target_commit_sha} not found")

            # Get files for the target commit
            files_stmt = select(db_entities.GitCommitFile).where(
                db_entities.GitCommitFile.commit_sha == db_tag.target_commit_sha
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

            target_commit = GitCommit(
                commit_sha=db_commit.commit_sha,
                date=db_commit.date,
                message=db_commit.message,
                parent_commit_sha=db_commit.parent_commit_sha,
                files=domain_files,
                author=db_commit.author,
                created_at=db_commit.created_at,
                updated_at=db_commit.updated_at,
            )

            return GitTag(
                repo_id=db_tag.repo_id,
                name=db_tag.name,
                target_commit=target_commit,
                created_at=db_tag.created_at,
                updated_at=db_tag.updated_at,
            )

    async def get_by_repo_id(self, repo_id: int) -> list[GitTag]:
        """Get all tags for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get all tags for the repo
            tags_stmt = select(db_entities.GitTag).where(
                db_entities.GitTag.repo_id == repo_id
            )
            db_tags = (await session.scalars(tags_stmt)).all()

            if not db_tags:
                return []

            commit_shas = [tag.target_commit_sha for tag in db_tags]

            # Get all target commits for these tags
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

            # Create domain tags
            domain_tags = []
            for db_tag in db_tags:
                db_commit = commits_by_sha.get(db_tag.target_commit_sha)
                if not db_commit:
                    continue

                commit_files = files_by_commit.get(db_tag.target_commit_sha, [])
                target_commit = GitCommit(
                    commit_sha=db_commit.commit_sha,
                    date=db_commit.date,
                    message=db_commit.message,
                    parent_commit_sha=db_commit.parent_commit_sha,
                    files=commit_files,
                    author=db_commit.author,
                    created_at=db_commit.created_at,
                    updated_at=db_commit.updated_at,
                )

                domain_tag = GitTag(
                    repo_id=db_tag.repo_id,
                    name=db_tag.name,
                    target_commit=target_commit,
                    created_at=db_tag.created_at,
                    updated_at=db_tag.updated_at,
                )
                domain_tags.append(domain_tag)

            return domain_tags

    async def save(self, tag: GitTag, repo_id: int) -> GitTag:
        """Save a tag to a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Set repo_id on the tag
            tag.repo_id = repo_id

            # Check if tag already exists
            existing_tag = await session.get(
                db_entities.GitTag, (repo_id, tag.name)
            )

            if existing_tag:
                # Update existing tag
                existing_tag.target_commit_sha = tag.target_commit.commit_sha
                if tag.updated_at:
                    existing_tag.updated_at = tag.updated_at
            else:
                # Create new tag
                db_tag = db_entities.GitTag(
                    repo_id=repo_id,
                    name=tag.name,
                    target_commit_sha=tag.target_commit.commit_sha,
                )
                session.add(db_tag)

            return tag

    async def save_bulk(self, tags: list[GitTag], repo_id: int) -> None:
        """Bulk save tags to a repository."""
        if not tags:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get existing tags in bulk
            existing_tags_stmt = select(db_entities.GitTag).where(
                db_entities.GitTag.repo_id == repo_id,
                db_entities.GitTag.name.in_([tag.name for tag in tags]),
            )
            existing_tags = (await session.scalars(existing_tags_stmt)).all()
            existing_tag_names = {tag.name for tag in existing_tags}

            # Update existing tags
            for existing_tag in existing_tags:
                for tag in tags:
                    if (
                        tag.name == existing_tag.name
                        and existing_tag.target_commit_sha
                        != tag.target_commit.commit_sha
                    ):
                        existing_tag.target_commit_sha = tag.target_commit.commit_sha
                        break

            # Prepare new tags for bulk insert
            new_tags_data = [
                {
                    "repo_id": repo_id,
                    "name": tag.name,
                    "target_commit_sha": tag.target_commit.commit_sha,
                }
                for tag in tags
                if tag.name not in existing_tag_names
            ]

            # Bulk insert new tags in chunks to avoid parameter limits
            if new_tags_data:
                chunk_size = 1000  # Conservative chunk size for parameter limits
                for i in range(0, len(new_tags_data), chunk_size):
                    chunk = new_tags_data[i : i + chunk_size]
                    stmt = insert(db_entities.GitTag).values(chunk)
                    await session.execute(stmt)

    async def exists(self, tag_name: str, repo_id: int) -> bool:
        """Check if a tag exists."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.GitTag.name).where(
                db_entities.GitTag.name == tag_name,
                db_entities.GitTag.repo_id == repo_id,
            )
            result = await session.scalar(stmt)
            return result is not None

    async def delete_by_repo_id(self, repo_id: int) -> None:
        """Delete all tags for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Delete tags
            del_tags_stmt = delete(db_entities.GitTag).where(
                db_entities.GitTag.repo_id == repo_id
            )
            await session.execute(del_tags_stmt)

    async def count_by_repo_id(self, repo_id: int) -> int:
        """Count the number of tags for a repository."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(func.count()).select_from(db_entities.GitTag).where(
                db_entities.GitTag.repo_id == repo_id
            )
            result = await session.scalar(stmt)
            return result or 0
