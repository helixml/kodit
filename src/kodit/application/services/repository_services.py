"""Bundle of repository services for data access."""

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from kodit.domain.protocols import (
        EnrichmentAssociationRepository,
        EnrichmentV2Repository,
        GitBranchRepository,
        GitCommitRepository,
        GitFileRepository,
        GitRepoRepository,
        GitTagRepository,
    )
    from kodit.infrastructure.sqlalchemy.embedding_repository import (
        SqlAlchemyEmbeddingRepository,
    )


class RepositoryServices:
    """Bundles all repository services for data access.

    This is a Parameter Object pattern to reduce constructor complexity.
    """

    def __init__(
        self,
        repo_repository: "GitRepoRepository",
        git_commit_repository: "GitCommitRepository",
        git_file_repository: "GitFileRepository",
        git_branch_repository: "GitBranchRepository",
        git_tag_repository: "GitTagRepository",
        enrichment_v2_repository: "EnrichmentV2Repository",
        enrichment_association_repository: "EnrichmentAssociationRepository",
        embedding_repository: "SqlAlchemyEmbeddingRepository",
    ) -> None:
        """Initialize repository services bundle."""
        self.repo = repo_repository
        self.git_commit = git_commit_repository
        self.git_file = git_file_repository
        self.git_branch = git_branch_repository
        self.git_tag = git_tag_repository
        self.enrichment_v2 = enrichment_v2_repository
        self.enrichment_association = enrichment_association_repository
        self.embedding = embedding_repository
