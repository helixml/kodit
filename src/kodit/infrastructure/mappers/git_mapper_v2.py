"""Mappers for converting between SQLAlchemy and Domain entities (V2)."""


from pydantic import AnyUrl

from kodit.domain.entities.git_v2 import (
    GitBranchV2,
    GitCommitV2,
    GitFileV2,
    GitRepositoryV2,
    GitTagV2,
)
from kodit.infrastructure.sqlalchemy import entities as db_entities


class GitMapperV2:
    """Mapper for Git domain entities V2."""

    def to_domain_git_repository(
        self,
        db_repo: db_entities.GitRepo,
        tracking_branch_name: str | None = None,
    ) -> GitRepositoryV2:
        """Convert SQLAlchemy GitRepo to domain GitRepositoryV2."""
        return GitRepositoryV2(
            id=db_repo.id,
            created_at=db_repo.created_at,
            updated_at=db_repo.updated_at,
            sanitized_remote_uri=AnyUrl(db_repo.sanitized_remote_uri),
            remote_uri=AnyUrl(db_repo.remote_uri),
            cloned_path=db_repo.cloned_path,
            tracking_branch_name=tracking_branch_name,
            last_scanned_at=db_repo.last_scanned_at,
        )

    def to_domain_git_commit(
        self,
        db_commit: db_entities.GitCommit,
        db_files: list[db_entities.GitCommitFile],
    ) -> GitCommitV2:
        """Convert SQLAlchemy GitCommit to domain GitCommitV2."""
        domain_files = [
            GitFileV2(
                created_at=db_file.created_at,
                blob_sha=db_file.blob_sha,
                path=db_file.path,
                mime_type=db_file.mime_type,
                size=db_file.size,
                extension=db_file.extension,
            )
            for db_file in db_files
        ]

        return GitCommitV2(
            created_at=db_commit.created_at,
            updated_at=db_commit.updated_at,
            commit_sha=db_commit.commit_sha,
            repo_id=db_commit.repo_id,
            date=db_commit.date,
            message=db_commit.message,
            parent_commit_sha=db_commit.parent_commit_sha,
            files=domain_files,
            author=db_commit.author,
        )

    def to_domain_git_branch(self, db_branch: db_entities.GitBranch) -> GitBranchV2:
        """Convert SQLAlchemy GitBranch to domain GitBranchV2."""
        return GitBranchV2(
            repo_id=db_branch.repo_id,
            name=db_branch.name,
            created_at=db_branch.created_at,
            updated_at=db_branch.updated_at,
            head_commit_sha=db_branch.head_commit_sha,
        )

    def to_domain_git_tag(self, db_tag: db_entities.GitTag) -> GitTagV2:
        """Convert SQLAlchemy GitTag to domain GitTagV2."""
        return GitTagV2(
            created_at=db_tag.created_at,
            updated_at=db_tag.updated_at,
            repo_id=db_tag.repo_id,
            name=db_tag.name,
            target_commit_sha=db_tag.target_commit_sha,
        )

    def to_db_git_commit(self, domain_commit: GitCommitV2) -> db_entities.GitCommit:
        """Convert domain GitCommitV2 to SQLAlchemy GitCommit."""
        return db_entities.GitCommit(
            commit_sha=domain_commit.commit_sha,
            repo_id=domain_commit.repo_id,
            date=domain_commit.date,
            message=domain_commit.message,
            parent_commit_sha=domain_commit.parent_commit_sha,
            author=domain_commit.author,
        )

    def to_db_git_commit_files(
        self, domain_commit: GitCommitV2
    ) -> list[db_entities.GitCommitFile]:
        """Convert domain GitCommitV2 files to SQLAlchemy GitCommitFile list."""
        return [
            db_entities.GitCommitFile(
                commit_sha=domain_commit.commit_sha,
                path=file.path,
                blob_sha=file.blob_sha,
                mime_type=file.mime_type,
                extension=file.extension,
                size=file.size,
                created_at=file.created_at,
            )
            for file in domain_commit.files
        ]

    def to_db_git_branch(self, domain_branch: GitBranchV2) -> db_entities.GitBranch:
        """Convert domain GitBranchV2 to SQLAlchemy GitBranch."""
        return db_entities.GitBranch(
            repo_id=domain_branch.repo_id,
            name=domain_branch.name,
            head_commit_sha=domain_branch.head_commit_sha,
        )

    def to_db_git_tag(self, domain_tag: GitTagV2) -> db_entities.GitTag:
        """Convert domain GitTagV2 to SQLAlchemy GitTag."""
        return db_entities.GitTag(
            repo_id=domain_tag.repo_id,
            name=domain_tag.name,
            target_commit_sha=domain_tag.target_commit_sha,
        )
