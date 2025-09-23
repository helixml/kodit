"""Mapping between domain Git entities and SQLAlchemy entities."""

from collections import defaultdict
from pathlib import Path

from pydantic import AnyUrl

import kodit.domain.entities.git as domain_git_entities
from kodit.infrastructure.sqlalchemy import entities as db_entities


class GitMapper:
    """Mapper for converting between domain Git entities and database entities."""

    def to_domain_commits(
        self,
        db_commits: list[db_entities.GitCommit],
        db_commit_files: list[db_entities.GitCommitFile],
    ) -> list[domain_git_entities.GitCommit]:
        """Convert SQLAlchemy GitCommit to domain GitCommit."""
        commit_files_map = defaultdict(list)
        for file in db_commit_files:
            commit_files_map[file.commit_sha].append(file.blob_sha)

        commit_domain_files_map = defaultdict(list)
        for file in db_commit_files:
            commit_domain_files_map[file.commit_sha].append(
                domain_git_entities.GitFile(
                    created_at=file.created_at,
                    blob_sha=file.blob_sha,
                    path=file.path,
                    mime_type=file.mime_type,
                    size=file.size,
                    extension=file.extension,
                )
            )

        domain_commits = []
        for db_commit in db_commits:
            domain_commit = domain_git_entities.GitCommit(
                created_at=db_commit.created_at,
                updated_at=db_commit.updated_at,
                commit_sha=db_commit.commit_sha,
                date=db_commit.date,
                message=db_commit.message,
                parent_commit_sha=db_commit.parent_commit_sha,
                files=commit_domain_files_map[db_commit.commit_sha],
                author=db_commit.author,
            )
            domain_commits.append(domain_commit)
        return domain_commits

    def to_domain_branches(
        self,
        db_branches: list[db_entities.GitBranch],
        domain_commits: list[domain_git_entities.GitCommit],
    ) -> list[domain_git_entities.GitBranch]:
        """Convert SQLAlchemy GitBranch to domain GitBranch."""
        commit_map = {commit.commit_sha: commit for commit in domain_commits}
        domain_branches = []
        for db_branch in db_branches:
            if db_branch.head_commit_sha not in commit_map:
                raise ValueError(
                    f"Commit {db_branch.head_commit_sha} for "
                    f"branch {db_branch.name} not found in commits: {commit_map.keys()}"
                )
            domain_branch = domain_git_entities.GitBranch(
                repo_id=db_branch.repo_id,
                name=db_branch.name,
                created_at=db_branch.created_at,
                updated_at=db_branch.updated_at,
                head_commit=commit_map[db_branch.head_commit_sha],
            )
            domain_branches.append(domain_branch)
        return domain_branches

    def to_domain_tags(
        self,
        db_tags: list[db_entities.GitTag],
        domain_commits: list[domain_git_entities.GitCommit],
    ) -> list[domain_git_entities.GitTag]:
        """Convert SQLAlchemy GitTag to domain GitTag."""
        commit_map = {commit.commit_sha: commit for commit in domain_commits}
        domain_tags = []
        for db_tag in db_tags:
            if db_tag.target_commit_sha not in commit_map:
                raise ValueError(
                    f"Commit {db_tag.target_commit_sha} for tag {db_tag.name} not found"
                )
            domain_tag = domain_git_entities.GitTag(
                created_at=db_tag.created_at,
                updated_at=db_tag.updated_at,
                repo_id=db_tag.repo_id,
                name=db_tag.name,
                target_commit=commit_map[db_tag.target_commit_sha],
            )
            domain_tags.append(domain_tag)
        return domain_tags

    def to_domain_tracking_branch(
        self,
        db_tracking_branch: db_entities.GitTrackingBranch | None,
        db_tracking_branch_entity: db_entities.GitBranch | None,
        domain_commits: list[domain_git_entities.GitCommit],
    ) -> domain_git_entities.GitBranch | None:
        """Convert SQLAlchemy GitTrackingBranch to domain GitBranch."""
        if db_tracking_branch is None or db_tracking_branch_entity is None:
            return None

        commit_map = {commit.commit_sha: commit for commit in domain_commits}
        if db_tracking_branch_entity.head_commit_sha not in commit_map:
            raise ValueError(
                f"Commit {db_tracking_branch_entity.head_commit_sha} for "
                f"tracking branch {db_tracking_branch.name} not found"
            )

        return domain_git_entities.GitBranch(
            repo_id=db_tracking_branch_entity.repo_id,
            name=db_tracking_branch_entity.name,
            created_at=db_tracking_branch_entity.created_at,
            updated_at=db_tracking_branch_entity.updated_at,
            head_commit=commit_map[db_tracking_branch_entity.head_commit_sha],
        )

    def to_domain_git_repo(  # noqa: PLR0913
        self,
        db_repo: db_entities.GitRepo,
        db_tracking_branch_entity: db_entities.GitBranch | None,
        db_commits: list[db_entities.GitCommit],
        db_tags: list[db_entities.GitTag],
        db_commit_files: list[db_entities.GitCommitFile],
        db_tracking_branch: db_entities.GitTrackingBranch | None,
    ) -> domain_git_entities.GitRepo:
        """Convert SQLAlchemy GitRepo to domain GitRepo."""
        # Build commits needed for tags and tracking branch
        domain_commits = self.to_domain_commits(
            db_commits=db_commits, db_commit_files=db_commit_files
        )
        self.to_domain_tags(
            db_tags=db_tags, domain_commits=domain_commits
        )
        tracking_branch = self.to_domain_tracking_branch(
            db_tracking_branch=db_tracking_branch,
            db_tracking_branch_entity=db_tracking_branch_entity,
            domain_commits=domain_commits,
        )

        from kodit.domain.factories.git_repo_factory import GitRepoFactory

        return GitRepoFactory.create_from_components(
            repo_id=db_repo.id,
            created_at=db_repo.created_at,
            updated_at=db_repo.updated_at,
            sanitized_remote_uri=AnyUrl(db_repo.sanitized_remote_uri),
            remote_uri=AnyUrl(db_repo.remote_uri),
            tracking_branch=tracking_branch,
            cloned_path=Path(db_repo.cloned_path) if db_repo.cloned_path else None,
            last_scanned_at=db_repo.last_scanned_at,
            num_commits=db_repo.num_commits,
            num_branches=db_repo.num_branches,
            num_tags=db_repo.num_tags,
        )

    def to_domain_commit_index(
        self,
        db_commit_index: db_entities.CommitIndex,
        snippets: list[domain_git_entities.SnippetV2],
    ) -> domain_git_entities.CommitIndex:
        """Convert SQLAlchemy CommitIndex to domain CommitIndex."""
        return domain_git_entities.CommitIndex(
            commit_sha=db_commit_index.commit_sha,
            created_at=db_commit_index.created_at,
            updated_at=db_commit_index.updated_at,
            snippets=snippets,
            status=domain_git_entities.IndexStatus(db_commit_index.status),
            indexed_at=db_commit_index.indexed_at,
            error_message=db_commit_index.error_message,
            files_processed=db_commit_index.files_processed,
            processing_time_seconds=float(db_commit_index.processing_time_seconds),
        )

    def from_domain_commit_index(
        self, domain_commit_index: domain_git_entities.CommitIndex
    ) -> db_entities.CommitIndex:
        """Convert domain CommitIndex to SQLAlchemy CommitIndex."""
        return db_entities.CommitIndex(
            commit_sha=domain_commit_index.commit_sha,
            status=domain_commit_index.status,
            indexed_at=domain_commit_index.indexed_at,
            error_message=domain_commit_index.error_message,
            files_processed=domain_commit_index.files_processed,
            processing_time_seconds=domain_commit_index.processing_time_seconds,
        )
