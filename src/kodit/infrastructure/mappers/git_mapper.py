"""Mapping between domain Git entities and SQLAlchemy entities."""

from pathlib import Path

from pydantic import AnyUrl

import kodit.domain.entities.git as domain_git_entities
from kodit.domain.value_objects import Enrichment, EnrichmentType
from kodit.infrastructure.sqlalchemy import entities as db_entities


class GitMapper:
    """Mapper for converting between domain Git entities and database entities."""

    def to_domain_git_repo(  # noqa: PLR0913
        self,
        db_repo: db_entities.GitRepo,
        db_branches: list[db_entities.GitBranch],
        db_commits: list[db_entities.GitCommit],
        db_tags: list[db_entities.GitTag],
        db_files: list[db_entities.GitFile],
        commit_files_map: dict[str, list[str]],  # commit_sha -> [file_blob_sha]
        tracking_branch_name: str,
    ) -> domain_git_entities.GitRepo:
        """Convert SQLAlchemy GitRepo to domain GitRepo."""
        # Convert files
        domain_files = {}
        for db_file in db_files:
            domain_file = domain_git_entities.GitFile(
                created_at=db_file.created_at,
                updated_at=db_file.updated_at,
                blob_sha=db_file.blob_sha,
                path=db_file.path,
                mime_type=db_file.mime_type,
                size=db_file.size,
                extension=db_file.extension,
            )
            domain_files[db_file.blob_sha] = domain_file

        # Convert commits
        domain_commits = []
        for db_commit in db_commits:
            # Get files for this commit
            file_blob_shas = commit_files_map.get(db_commit.commit_sha, [])
            commit_files = [
                domain_files[sha] for sha in file_blob_shas if sha in domain_files
            ]

            domain_commit = domain_git_entities.GitCommit(
                created_at=db_commit.created_at,
                updated_at=db_commit.updated_at,
                commit_sha=db_commit.commit_sha,
                date=db_commit.date,
                message=db_commit.message,
                parent_commit_sha=db_commit.parent_commit_sha,
                files=commit_files,
                author=db_commit.author,
            )
            domain_commits.append(domain_commit)

        # Convert branches
        domain_branches = []
        tracking_branch = None
        for db_branch in db_branches:
            # Find head commit
            head_commit = next(
                (
                    c
                    for c in domain_commits
                    if c.commit_sha == db_branch.head_commit_sha
                ),
                None,
            )
            if not head_commit:
                continue

            domain_branch = domain_git_entities.GitBranch(
                id=db_branch.id,
                created_at=db_branch.created_at,
                updated_at=db_branch.updated_at,
                name=db_branch.name,
                head_commit=head_commit,
            )
            domain_branches.append(domain_branch)

            if db_branch.name == tracking_branch_name:
                tracking_branch = domain_branch

        # Convert tags
        domain_tags = []
        for db_tag in db_tags:
            domain_tag = domain_git_entities.GitTag(
                created_at=db_tag.created_at,
                updated_at=db_tag.updated_at,
                name=db_tag.name,
                target_commit_sha=db_tag.target_commit_sha,
            )
            domain_tags.append(domain_tag)

        # Create domain GitRepo
        if not tracking_branch:
            # Use first branch as fallback
            tracking_branch = domain_branches[0] if domain_branches else None

        return domain_git_entities.GitRepo(
            id=db_repo.id,
            created_at=db_repo.created_at,
            updated_at=db_repo.updated_at,
            sanitized_remote_uri=AnyUrl(db_repo.sanitized_remote_uri),
            branches=domain_branches,
            commits=domain_commits,
            tags=domain_tags,
            tracking_branch=tracking_branch,
            cloned_path=Path(db_repo.cloned_path) if db_repo.cloned_path else None,
            remote_uri=AnyUrl(db_repo.remote_uri),
            last_scanned_at=db_repo.last_scanned_at,
        )

    def to_domain_snippet_v2(
        self,
        db_snippet: db_entities.SnippetV2,
        derives_from: list[domain_git_entities.GitFile],
        db_enrichments: list[db_entities.Enrichment],
    ) -> domain_git_entities.SnippetV2:
        """Convert SQLAlchemy SnippetV2 to domain SnippetV2."""
        # Convert enrichments
        enrichments = []
        for db_enrichment in db_enrichments:
            # Map from SQLAlchemy enum to domain enum
            enrichment_type = EnrichmentType(db_enrichment.type.value)
            enrichment = Enrichment(
                type=enrichment_type,
                content=db_enrichment.content,
            )
            enrichments.append(enrichment)

        return domain_git_entities.SnippetV2(
            sha=db_snippet.sha,
            created_at=db_snippet.created_at,
            updated_at=db_snippet.updated_at,
            derives_from=derives_from,
            content=db_snippet.content,
            enrichments=enrichments,
            extension=db_snippet.extension,
        )

    def from_domain_snippet_v2(
        self, domain_snippet: domain_git_entities.SnippetV2
    ) -> db_entities.SnippetV2:
        """Convert domain SnippetV2 to SQLAlchemy SnippetV2."""
        return db_entities.SnippetV2(
            sha=domain_snippet.sha,
            content=domain_snippet.content,
            extension=domain_snippet.extension,
        )

    def from_domain_enrichments(
        self, snippet_sha: str, enrichments: list[Enrichment]
    ) -> list[db_entities.Enrichment]:
        """Convert domain enrichments to SQLAlchemy enrichments."""
        db_enrichments = []
        for enrichment in enrichments:
            # Map from domain enum to SQLAlchemy enum
            db_enrichment_type = db_entities.EnrichmentType(enrichment.type.value)
            db_enrichment = db_entities.Enrichment(
                snippet_sha=snippet_sha,
                type=db_enrichment_type,
                content=enrichment.content,
            )
            db_enrichments.append(db_enrichment)
        return db_enrichments

    def to_domain_commit_index(
        self,
        db_commit_index: db_entities.CommitIndex,
        snippets: list[domain_git_entities.SnippetV2],
    ) -> domain_git_entities.CommitIndex:
        """Convert SQLAlchemy CommitIndex to domain CommitIndex."""
        from kodit.domain.entities.git import IndexStatus

        # Map status
        status_map = {
            db_entities.IndexStatusType.PENDING: IndexStatus.PENDING,
            db_entities.IndexStatusType.IN_PROGRESS: IndexStatus.IN_PROGRESS,
            db_entities.IndexStatusType.COMPLETED: IndexStatus.COMPLETED,
            db_entities.IndexStatusType.FAILED: IndexStatus.FAILED,
        }

        return domain_git_entities.CommitIndex(
            commit_sha=db_commit_index.commit_sha,
            created_at=db_commit_index.created_at,
            updated_at=db_commit_index.updated_at,
            snippets=snippets,
            status=status_map[db_commit_index.status],
            indexed_at=db_commit_index.indexed_at,
            error_message=db_commit_index.error_message,
            files_processed=db_commit_index.files_processed,
            processing_time_seconds=float(db_commit_index.processing_time_seconds),
        )

    def from_domain_commit_index(
        self, domain_commit_index: domain_git_entities.CommitIndex
    ) -> db_entities.CommitIndex:
        """Convert domain CommitIndex to SQLAlchemy CommitIndex."""
        from kodit.domain.entities.git import IndexStatus

        # Map status
        status_map = {
            IndexStatus.PENDING: db_entities.IndexStatusType.PENDING,
            IndexStatus.IN_PROGRESS: db_entities.IndexStatusType.IN_PROGRESS,
            IndexStatus.COMPLETED: db_entities.IndexStatusType.COMPLETED,
            IndexStatus.FAILED: db_entities.IndexStatusType.FAILED,
        }

        return db_entities.CommitIndex(
            commit_sha=domain_commit_index.commit_sha,
            status=status_map[domain_commit_index.status],
            indexed_at=domain_commit_index.indexed_at,
            error_message=domain_commit_index.error_message,
            files_processed=domain_commit_index.files_processed,
            processing_time_seconds=domain_commit_index.processing_time_seconds,
        )
