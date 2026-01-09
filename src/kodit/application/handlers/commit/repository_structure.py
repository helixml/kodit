"""Handler for repository structure discovery for a commit."""

from typing import TYPE_CHECKING, Any

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.architecture.repository_structure.repository_structure import (  # noqa: E501
    RepositoryStructureEnrichment,
)
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.enrichment import CommitEnrichmentAssociation
from kodit.domain.enrichments.request import (
    EnrichmentRequest as GenericEnrichmentRequest,
)
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    GitRepoRepository,
)
from kodit.domain.services.repository_structure_service import (
    REPOSITORY_STRUCTURE_ENRICHMENT_SYSTEM_PROMPT,
    REPOSITORY_STRUCTURE_ENRICHMENT_TASK_PROMPT,
    RepositoryStructureService,
)
from kodit.domain.value_objects import TaskOperation, TrackableType

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )


class RepositoryStructureHandler:
    """Handler for discovering repository structure for a commit."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        repository_structure_service: RepositoryStructureService,
        enricher_service: Enricher,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the repository structure handler."""
        self.repo_repository = repo_repository
        self.repository_structure_service = repository_structure_service
        self.enricher_service = enricher_service
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute repository structure discovery operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.CREATE_REPOSITORY_STRUCTURE_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Check if repository structure enrichment already exists for this commit
            if await self.enrichment_query_service.has_repository_structure_for_commit(
                commit_sha
            ):
                await step.skip(
                    "Repository structure enrichment already exists for commit"
                )
                return

            # Get repository path
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            await step.set_total(3)

            await step.set_current(1, "Discovering repository structure")

            # Discover repository structure
            repository_tree = (
                await self.repository_structure_service.discover_structure(
                    repo.cloned_path,
                    repo_url=str(repo.sanitized_remote_uri),
                )
            )

            await step.set_current(2, "Collapsing and summarizing structure with LLM")

            # Enrich the repository tree through the enricher
            enrichment_request = GenericEnrichmentRequest(
                id=commit_sha,
                text=REPOSITORY_STRUCTURE_ENRICHMENT_TASK_PROMPT.format(
                    repository_tree=repository_tree,
                ),
                system_prompt=REPOSITORY_STRUCTURE_ENRICHMENT_SYSTEM_PROMPT,
            )

            enriched_content = ""
            async for response in self.enricher_service.enrich([enrichment_request]):
                enriched_content = response.text

            # Create and save repository structure enrichment with enriched content
            enrichment = await self.enrichment_v2_repository.save(
                RepositoryStructureEnrichment(
                    content=enriched_content,
                )
            )
            if not enrichment or not enrichment.id:
                raise ValueError(
                    f"Failed to save repository structure enrichment "
                    f"for commit {commit_sha}"
                )
            await self.enrichment_association_repository.save(
                CommitEnrichmentAssociation(
                    enrichment_id=enrichment.id,
                    entity_id=commit_sha,
                )
            )

            await step.set_current(3, "Repository structure enrichment completed")
