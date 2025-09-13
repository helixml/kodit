"""SQLAlchemy implementation of CommitIndexRepository."""

from collections.abc import Callable

from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import CommitIndex
from kodit.domain.protocols import CommitIndexRepository
from kodit.infrastructure.mappers.git_mapper import GitMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.snippet_v2_repository import (
    SqlAlchemySnippetRepositoryV2,
)
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_commit_index_repository(
    session_factory: Callable[[], AsyncSession],
) -> CommitIndexRepository:
    """Create a commit index repository."""
    uow = SqlAlchemyUnitOfWork(session_factory=session_factory)
    return SqlAlchemyCommitIndexRepository(uow)


class SqlAlchemyCommitIndexRepository(CommitIndexRepository):
    """SQLAlchemy implementation of CommitIndexRepository."""

    def __init__(self, uow: SqlAlchemyUnitOfWork) -> None:
        """Initialize the repository."""
        self.uow = uow
        self._snippet_repo = SqlAlchemySnippetRepositoryV2(uow)

    @property
    def _mapper(self) -> GitMapper:
        return GitMapper()

    @property
    def _session(self) -> AsyncSession:
        if self.uow.session is None:
            raise RuntimeError("UnitOfWork must be used within async context")
        return self.uow.session

    async def save(self, commit_index: CommitIndex) -> None:
        """Save or update a commit index."""
        async with self.uow:
            # Check if commit index already exists
            existing_index = await self._session.get(
                db_entities.CommitIndex, commit_index.commit_sha
            )

            if existing_index:
                # Update existing index
                db_commit_index = self._mapper.from_domain_commit_index(commit_index)
                existing_index.status = db_commit_index.status
                existing_index.indexed_at = db_commit_index.indexed_at
                existing_index.error_message = db_commit_index.error_message
                existing_index.files_processed = db_commit_index.files_processed
                existing_index.processing_time_seconds = (
                    db_commit_index.processing_time_seconds
                )
                existing_index.updated_at = db_commit_index.updated_at
            else:
                # Create new index
                db_commit_index = self._mapper.from_domain_commit_index(commit_index)
                self._session.add(db_commit_index)
                await self._session.flush()

            # Save snippets using the snippet repository
            if commit_index.snippets:
                await self._snippet_repo.save_snippets(
                    commit_index.commit_sha, commit_index.snippets
                )

    async def get_by_commit(self, commit_sha: str) -> CommitIndex | None:
        """Get index data for a specific commit."""
        async with self.uow:
            # Get the commit index
            db_commit_index = await self._session.get(
                db_entities.CommitIndex, commit_sha
            )
            if not db_commit_index:
                return None

            # Get associated snippets
            snippets = await self._snippet_repo.get_snippets_for_commit(commit_sha)

            # Convert to domain entity
            return self._mapper.to_domain_commit_index(db_commit_index, snippets)

    async def get_indexed_commits_for_repo(self, repo_uri: str) -> list[CommitIndex]:
        """Get all indexed commits for a repository."""
        async with self.uow:
            # Get all commit indexes for commits that belong to this repo
            # We need to join through git_commits to filter by repo
            stmt = (
                select(db_entities.CommitIndex)
                .join(
                    db_entities.GitCommit,
                    db_entities.CommitIndex.commit_sha
                    == db_entities.GitCommit.commit_sha,
                )
                .join(
                    db_entities.GitRepo,
                    db_entities.GitCommit.repo_id == db_entities.GitRepo.id,
                )
                .where(db_entities.GitRepo.sanitized_remote_uri == repo_uri)
            )

            db_commit_indexes = (await self._session.scalars(stmt)).all()

            commit_indexes = []
            for db_commit_index in db_commit_indexes:
                # Get snippets for this commit
                snippets = await self._snippet_repo.get_snippets_for_commit(
                    db_commit_index.commit_sha
                )

                # Convert to domain entity
                domain_commit_index = self._mapper.to_domain_commit_index(
                    db_commit_index, snippets
                )
                commit_indexes.append(domain_commit_index)

            return commit_indexes

    async def delete(self, commit_sha: str) -> bool:
        """Delete index data for a commit."""
        async with self.uow:
            # Delete snippets first
            await self._snippet_repo.delete_snippets_for_commit(commit_sha)

            # Delete the commit index
            stmt = delete(db_entities.CommitIndex).where(
                db_entities.CommitIndex.commit_sha == commit_sha
            )
            result = await self._session.execute(stmt)

            return result.rowcount > 0
