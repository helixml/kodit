"""SQLAlchemy implementation of SnippetRepositoryV2."""

import zlib
from collections.abc import Callable

from sqlalchemy import delete, insert, select
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

    async def _bulk_create_file_associations(
        self, session: AsyncSession, commit_sha: str, snippets: list[SnippetV2]
    ) -> None:
        """Bulk create snippet-file associations."""
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
        new_file_associations = []
        for snippet in snippets:
            for file in snippet.derives_from:
                association_key = (snippet.sha, file.path)
                if (association_key not in existing_snippet_files
                    and file.path in existing_files_map):
                    new_file_associations.append({
                        "snippet_sha": snippet.sha,
                        "blob_sha": existing_files_map[file.path],
                        "commit_sha": commit_sha,
                        "file_path": file.path,
                    })

        # Bulk insert new file associations in chunks to avoid parameter limits
        if new_file_associations:
            chunk_size = 1000  # Conservative chunk size for parameter limits
            for i in range(0, len(new_file_associations), chunk_size):
                chunk = new_file_associations[i : i + chunk_size]
                stmt = insert(db_entities.SnippetV2File).values(chunk)
                await session.execute(stmt)

    async def _bulk_update_enrichments(
        self, session: AsyncSession, snippets: list[SnippetV2]
    ) -> None:
        """Bulk update enrichments for snippets."""
        snippet_shas = [snippet.sha for snippet in snippets]

        # Get all existing enrichments for these snippets
        existing_enrichments_stmt = select(
            db_entities.Enrichment.snippet_sha,
            db_entities.Enrichment.type,
            db_entities.Enrichment.content
        ).where(
            db_entities.Enrichment.snippet_sha.in_(snippet_shas)
        )
        existing_enrichments = await session.execute(existing_enrichments_stmt)

        # Create lookup for existing enrichment hashes
        existing_enrichment_map = {}
        for snippet_sha, enrichment_type, content in existing_enrichments:
            content_hash = self._hash_string(content)
            key = (snippet_sha, enrichment_type)
            existing_enrichment_map[key] = content_hash

        # Collect enrichments to delete and add
        enrichments_to_delete = []
        enrichments_to_add = []

        for snippet in snippets:
            for enrichment in snippet.enrichments:
                key = (snippet.sha, db_entities.EnrichmentType(enrichment.type.value))
                new_hash = self._hash_string(enrichment.content)

                if key in existing_enrichment_map:
                    if existing_enrichment_map[key] != new_hash:
                        # Content changed, mark for deletion and re-addition
                        enrichments_to_delete.append(key)
                        enrichments_to_add.append({
                            "snippet_sha": snippet.sha,
                            "type": db_entities.EnrichmentType(enrichment.type.value),
                            "content": enrichment.content,
                        })
                else:
                    # New enrichment
                    enrichments_to_add.append({
                        "snippet_sha": snippet.sha,
                        "type": db_entities.EnrichmentType(enrichment.type.value),
                        "content": enrichment.content,
                    })

        # Bulk delete changed enrichments
        if enrichments_to_delete:
            for snippet_sha, enrichment_type in enrichments_to_delete:
                stmt = delete(db_entities.Enrichment).where(
                    db_entities.Enrichment.snippet_sha == snippet_sha,
                    db_entities.Enrichment.type == enrichment_type,
                )
                await session.execute(stmt)

        # Bulk insert new/updated enrichments in chunks to avoid parameter limits
        if enrichments_to_add:
            chunk_size = 1000  # Conservative chunk size for parameter limits
            for i in range(0, len(enrichments_to_add), chunk_size):
                chunk = enrichments_to_add[i : i + chunk_size]
                insert_stmt = insert(db_entities.Enrichment).values(chunk)
                await session.execute(insert_stmt)

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
