"""SQLAlchemy implementation of SnippetRepositoryV2."""

import zlib
from collections.abc import Callable
from datetime import datetime
from typing import TypedDict

from sqlalchemy import delete, insert, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enrichments.development.snippet.snippet import SnippetEnrichment
from kodit.domain.entities.git import SnippetV2
from kodit.domain.protocols import SnippetRepositoryV2
from kodit.domain.value_objects import MultiSearchRequest
from kodit.infrastructure.mappers.snippet_mapper import SnippetMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.enrichment_v2_repository import (
    EnrichmentV2Repository,
)
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


class _GitFileData(TypedDict):
    """Type for GitCommitFile creation data."""

    commit_sha: str
    path: str
    blob_sha: str
    mime_type: str
    size: int
    extension: str
    created_at: datetime


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
        self._enrichment_repo = EnrichmentV2Repository(session_factory)

    @property
    def _mapper(self) -> SnippetMapper:
        return SnippetMapper()

    async def save_snippets(self, commit_sha: str, snippets: list[SnippetV2]) -> None:
        """Batch save snippets for a commit."""
        if not snippets:
            return

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Bulk operations for better performance
            await self._bulk_save_snippets(session, snippets)
            await self._bulk_create_commit_associations(session, commit_sha, snippets)
            await self._bulk_create_file_associations(session, commit_sha, snippets)
            await self._bulk_update_enrichments(session, snippets)

    async def _bulk_save_snippets(
        self, session: AsyncSession, snippets: list[SnippetV2]
    ) -> None:
        """Bulk save snippets using efficient batch operations."""
        snippet_shas = [snippet.sha for snippet in snippets]

        # Get existing snippets in bulk
        existing_snippets_stmt = select(db_entities.SnippetV2.sha).where(
            db_entities.SnippetV2.sha.in_(snippet_shas)
        )
        existing_snippet_shas = set(
            (await session.scalars(existing_snippets_stmt)).all()
        )

        # Prepare new snippets for bulk insert
        new_snippets = [
            {
                "sha": snippet.sha,
                "content": snippet.content,
                "extension": snippet.extension,
            }
            for snippet in snippets
            if snippet.sha not in existing_snippet_shas
        ]

        # Bulk insert new snippets in chunks to avoid parameter limits
        if new_snippets:
            chunk_size = 1000  # Conservative chunk size for parameter limits
            for i in range(0, len(new_snippets), chunk_size):
                chunk = new_snippets[i : i + chunk_size]
                stmt = insert(db_entities.SnippetV2).values(chunk)
                await session.execute(stmt)

    async def _bulk_create_commit_associations(
        self, session: AsyncSession, commit_sha: str, snippets: list[SnippetV2]
    ) -> None:
        """Bulk create commit-snippet associations."""
        snippet_shas = [snippet.sha for snippet in snippets]

        # Get existing associations in bulk
        existing_associations_stmt = select(
            db_entities.CommitSnippetV2.snippet_sha
        ).where(
            db_entities.CommitSnippetV2.commit_sha == commit_sha,
            db_entities.CommitSnippetV2.snippet_sha.in_(snippet_shas)
        )
        existing_association_shas = set(
            (await session.scalars(existing_associations_stmt)).all()
        )

        # Prepare new associations for bulk insert
        new_associations = [
            {
                "commit_sha": commit_sha,
                "snippet_sha": snippet.sha,
            }
            for snippet in snippets
            if snippet.sha not in existing_association_shas
        ]

        # Bulk insert new associations in chunks to avoid parameter limits
        if new_associations:
            chunk_size = 1000  # Conservative chunk size for parameter limits
            for i in range(0, len(new_associations), chunk_size):
                chunk = new_associations[i : i + chunk_size]
                stmt = insert(db_entities.CommitSnippetV2).values(chunk)
                await session.execute(stmt)

    async def _bulk_create_file_associations(  # noqa: C901
        self, session: AsyncSession, commit_sha: str, snippets: list[SnippetV2]
    ) -> None:
        """Bulk create snippet-file associations.

        Creates SnippetV2File records linking snippets to GitCommitFile records.
        If a GitCommitFile doesn't exist, it creates it automatically to prevent
        losing file associations during enrichment cycles.
        """
        # Collect all file paths from all snippets
        file_paths = set()
        for snippet in snippets:
            for file in snippet.derives_from:
                file_paths.add(file.path)

        if not file_paths:
            return

        # Get existing files in bulk
        existing_files_stmt = select(
            db_entities.GitCommitFile.path,
            db_entities.GitCommitFile.blob_sha
        ).where(
            db_entities.GitCommitFile.commit_sha == commit_sha,
            db_entities.GitCommitFile.path.in_(list(file_paths))
        )
        existing_files_result = await session.execute(existing_files_stmt)
        existing_files_map: dict[str, str] = {
            row[0]: row[1] for row in existing_files_result.fetchall()
        }

        # Get existing snippet-file associations to avoid duplicates
        snippet_shas = [snippet.sha for snippet in snippets]
        existing_snippet_files_stmt = select(
            db_entities.SnippetV2File.snippet_sha,
            db_entities.SnippetV2File.file_path
        ).where(
            db_entities.SnippetV2File.commit_sha == commit_sha,
            db_entities.SnippetV2File.snippet_sha.in_(snippet_shas)
        )
        existing_snippet_files = set(await session.execute(existing_snippet_files_stmt))

        # Prepare new file associations
        new_file_associations: list[dict[str, str]] = []
        missing_git_files: list[_GitFileData] = []

        for snippet in snippets:
            for file in snippet.derives_from:
                association_key = (snippet.sha, file.path)
                if association_key not in existing_snippet_files:
                    if file.path in existing_files_map:
                        # GitCommitFile exists, use its blob_sha
                        new_file_associations.append({
                            "snippet_sha": snippet.sha,
                            "blob_sha": existing_files_map[file.path],
                            "commit_sha": commit_sha,
                            "file_path": file.path,
                        })
                    else:
                        # GitCommitFile doesn't exist - create it and the association
                        missing_git_files.append({
                            "commit_sha": commit_sha,
                            "path": file.path,
                            "blob_sha": file.blob_sha,
                            "mime_type": file.mime_type,
                            "size": file.size,
                            "extension": file.extension,
                            "created_at": file.created_at,
                        })
                        new_file_associations.append({
                            "snippet_sha": snippet.sha,
                            "blob_sha": file.blob_sha,
                            "commit_sha": commit_sha,
                            "file_path": file.path,
                        })
                        # Add to map so subsequent snippets can find it
                        existing_files_map[file.path] = file.blob_sha

        # Create missing GitCommitFile records
        if missing_git_files:
            for git_file_data in missing_git_files:
                git_file = db_entities.GitCommitFile(
                    commit_sha=git_file_data["commit_sha"],
                    path=git_file_data["path"],
                    blob_sha=git_file_data["blob_sha"],
                    mime_type=git_file_data["mime_type"],
                    size=git_file_data["size"],
                    extension=git_file_data["extension"],
                    created_at=git_file_data["created_at"],
                )
                session.add(git_file)
            await session.flush()

        # Bulk insert new file associations in chunks to avoid parameter limits
        if new_file_associations:
            chunk_size = 1000  # Conservative chunk size for parameter limits
            for i in range(0, len(new_file_associations), chunk_size):
                chunk = new_file_associations[i : i + chunk_size]
                stmt = insert(db_entities.SnippetV2File).values(chunk)
                await session.execute(stmt)

    async def _bulk_update_enrichments(
        self, session: AsyncSession, snippets: list[SnippetV2]  # noqa: ARG002
    ) -> None:
        """Bulk update enrichments for snippets using new enrichment_v2."""
        # Collect all enrichments from snippets using list comprehension
        snippet_enrichments = [
            SnippetEnrichment(
                entity_id=snippet.sha,
                content=enrichment.content,
            )
            for snippet in snippets
            for enrichment in snippet.enrichments
        ]

        if snippet_enrichments:
            # First delete existing enrichments for these snippets
            snippet_shas = [snippet.sha for snippet in snippets]
            await self._enrichment_repo.bulk_delete_enrichments(
                entity_type="snippet_v2",
                entity_ids=snippet_shas,
            )

            # Then save the new enrichments
            await self._enrichment_repo.bulk_save_enrichments(snippet_enrichments)

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
        # For now, enrichments are not yet implemented with the new schema
        # This method will need to be updated once we migrate to EnrichmentV2

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
        db_files_list = list(db_files)

        # Get enrichments for this snippet
        db_enrichments = await self._enrichment_repo.enrichments_for_entity_type(
            entity_type="snippet_v2",
            entity_ids=[db_snippet.sha],
        )

        return self._mapper.to_domain_snippet_v2(
            db_snippet=db_snippet,
            db_files=db_files_list,
            db_enrichments=db_enrichments,
        )
