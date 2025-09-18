"""SQLAlchemy implementation of SnippetRepositoryV2."""

import zlib
from collections.abc import Callable

from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import SnippetV2
from kodit.domain.protocols import SnippetRepositoryV2
from kodit.domain.value_objects import MultiSearchRequest
from kodit.infrastructure.mappers.snippet_mapper import SnippetMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_snippet_v2_repository(
    session_factory: Callable[[], AsyncSession],
) -> SnippetRepositoryV2:
    """Create a snippet v2 repository."""
    return SqlAlchemySnippetRepositoryV2(session_factory=session_factory)


class SqlAlchemySnippetRepositoryV2(SnippetRepositoryV2):
    """SQLAlchemy implementation of SnippetRepositoryV2."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory

    @property
    def _mapper(self) -> SnippetMapper:
        return SnippetMapper()

    async def save_snippets(self, commit_sha: str, snippets: list[SnippetV2]) -> None:
        """Batch save snippets for a commit."""
        if not snippets:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            for domain_snippet in snippets:
                db_snippet = await self._get_or_create_raw_snippet(
                    session, commit_sha, domain_snippet
                )
                await self._update_enrichments_if_changed(
                    session, db_snippet, domain_snippet
                )
                await session.flush()

    async def _get_or_create_raw_snippet(
        self, session: AsyncSession, commit_sha: str, domain_snippet: SnippetV2
    ) -> db_entities.SnippetV2:
        """Get or create a SnippetV2 in the database."""
        db_snippet = await session.get(db_entities.SnippetV2, domain_snippet.sha)
        if not db_snippet:
            db_snippet = self._mapper.from_domain_snippet_v2(domain_snippet)
            session.add(db_snippet)
            await session.flush()

            # Associate snippet with commit
            commit_association = db_entities.CommitSnippetV2(
                commit_sha=commit_sha,
                snippet_sha=db_snippet.sha,
            )
            session.add(commit_association)

            # Associate snippet with files
            for file in domain_snippet.derives_from:
                # Find the file in the database (which should have been created during
                # the scan)
                db_file = await session.get(
                    db_entities.GitCommitFile, (commit_sha, file.path)
                )
                if not db_file:
                    raise ValueError(
                        f"File {file.path} not found for commit {commit_sha}"
                    )
                db_association = db_entities.SnippetV2File(
                    snippet_sha=db_snippet.sha,
                    blob_sha=db_file.blob_sha,
                    commit_sha=commit_sha,
                    file_path=file.path,
                )
                session.add(db_association)
        return db_snippet

    async def _update_enrichments_if_changed(
        self,
        session: AsyncSession,
        db_snippet: db_entities.SnippetV2,
        domain_snippet: SnippetV2,
    ) -> None:
        """Update enrichments if they have changed."""
        current_enrichments = await session.scalars(
            select(db_entities.Enrichment).where(
                db_entities.Enrichment.snippet_sha == db_snippet.sha
            )
        )
        current_enrichment_shas = {
            self._hash_string(enrichment.content)
            for enrichment in list(current_enrichments)
        }
        for enrichment in domain_snippet.enrichments:
            if self._hash_string(enrichment.content) in current_enrichment_shas:
                continue

            # If not present, delete the existing enrichment for this type if it exists
            stmt = delete(db_entities.Enrichment).where(
                db_entities.Enrichment.snippet_sha == db_snippet.sha,
                db_entities.Enrichment.type
                == db_entities.EnrichmentType(enrichment.type.value),
            )
            await session.execute(stmt)

            db_enrichment = db_entities.Enrichment(
                snippet_sha=db_snippet.sha,
                type=db_entities.EnrichmentType(enrichment.type.value),
                content=enrichment.content,
            )
            session.add(db_enrichment)

    async def get_snippets_for_commit(self, commit_sha: str) -> list[SnippetV2]:
        """Get all snippets for a specific commit."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get snippets for the commit through the association table
            snippet_associations = (
                await session.scalars(
                    select(db_entities.CommitSnippetV2).where(
                        db_entities.CommitSnippetV2.commit_sha == commit_sha
                    )
                )
            ).all()
            if not snippet_associations:
                print("### No snippet associations found for commit", commit_sha)
                return []
            db_snippets = (
                await session.scalars(
                    select(db_entities.SnippetV2).where(
                        db_entities.SnippetV2.sha.in_(
                            [
                                association.snippet_sha
                                for association in snippet_associations
                            ]
                        )
                    )
                )
            ).all()

            return [
                await self._to_domain_snippet_v2(session, db_snippet)
                for db_snippet in db_snippets
            ]

    async def delete_snippets_for_commit(self, commit_sha: str) -> None:
        """Delete all snippet associations for a commit."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Note: We only delete the commit-snippet associations,
            # not the snippets themselves as they might be used by other commits
            stmt = delete(db_entities.CommitSnippetV2).where(
                db_entities.CommitSnippetV2.commit_sha == commit_sha
            )
            await session.execute(stmt)

    def _hash_string(self, string: str) -> int:
        """Hash a string."""
        return zlib.crc32(string.encode())

    async def search(self, request: MultiSearchRequest) -> list[SnippetV2]:
        """Search snippets with filters."""
        raise NotImplementedError("Not implemented")

        # Build base query joining all necessary tables
        query = (
            select(
                db_entities.SnippetV2,
                db_entities.GitCommit,
                db_entities.GitFile,
                db_entities.GitRepo,
            )
            .join(
                db_entities.CommitSnippetV2,
                db_entities.SnippetV2.sha == db_entities.CommitSnippetV2.snippet_sha,
            )
            .join(
                db_entities.GitCommit,
                db_entities.CommitSnippetV2.commit_sha
                == db_entities.GitCommit.commit_sha,
            )
            .join(
                db_entities.SnippetV2File,
                db_entities.SnippetV2.sha == db_entities.SnippetV2File.snippet_sha,
            )
            .join(
                db_entities.GitCommitFile,
                db_entities.SnippetV2.sha == db_entities.Enrichment.snippet_sha,
            )
            .join(
                db_entities.GitFile,
                db_entities.SnippetV2File.file_blob_sha == db_entities.GitFile.blob_sha,
            )
            .join(
                db_entities.GitRepo,
                db_entities.GitCommitFile.file_blob_sha == db_entities.GitRepo.id,
            )
        )

        # Apply filters if provided
        if request.filters:
            if request.filters.source_repo:
                query = query.where(
                    db_entities.GitRepo.sanitized_remote_uri.ilike(
                        f"%{request.filters.source_repo}%"
                    )
                )

            if request.filters.file_path:
                query = query.where(
                    db_entities.GitFile.path.ilike(f"%{request.filters.file_path}%")
                )

            # TODO(Phil): Double check that git timestamps are correctly populated
            if request.filters.created_after:
                query = query.where(
                    db_entities.GitFile.created_at >= request.filters.created_after
                )

            if request.filters.created_before:
                query = query.where(
                    db_entities.GitFile.created_at <= request.filters.created_before
                )

        # Apply limit
        query = query.limit(request.top_k)

        # Execute query
        async with SqlAlchemyUnitOfWork(self.session_factory):
            result = await self._session.scalars(query)
            db_snippets = result.all()

            return [
                self._mapper.to_domain_snippet_v2(
                    db_snippet=snippet,
                    derives_from=git_file,
                    db_enrichments=[],
                )
                for snippet, git_commit, git_file, git_repo in db_snippets
            ]

    async def get_by_ids(self, ids: list[str]) -> list[SnippetV2]:
        """Get snippets by their IDs."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Get snippets for the commit through the association table
            db_snippets = (
                await session.scalars(
                    select(db_entities.SnippetV2).where(
                        db_entities.SnippetV2.sha.in_(ids)
                    )
                )
            ).all()

            return [
                await self._to_domain_snippet_v2(session, db_snippet)
                for db_snippet in db_snippets
            ]

    async def _to_domain_snippet_v2(
        self, session: AsyncSession, db_snippet: db_entities.SnippetV2
    ) -> SnippetV2:
        """Convert a SQLAlchemy SnippetV2 to a domain SnippetV2."""
        # Files it derives from
        db_files = await session.scalars(
            select(db_entities.GitCommitFile)
            .join(
                db_entities.SnippetV2File,
                (db_entities.GitCommitFile.path == db_entities.SnippetV2File.file_path)
                & (
                    db_entities.GitCommitFile.commit_sha
                    == db_entities.SnippetV2File.commit_sha
                ),
            )
            .where(db_entities.SnippetV2File.snippet_sha == db_snippet.sha)
        )

        # Enrichments related to this snippet
        db_enrichments = await session.scalars(
            select(db_entities.Enrichment).where(
                db_entities.Enrichment.snippet_sha == db_snippet.sha
            )
        )

        return self._mapper.to_domain_snippet_v2(
            db_snippet=db_snippet,
            db_files=list(db_files),
            db_enrichments=list(db_enrichments),
        )
