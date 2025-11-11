"""Handler for creating commit description for a commit."""

from typing import TYPE_CHECKING, Any

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.enrichment import CommitEnrichmentAssociation
from kodit.domain.enrichments.history.commit_description.commit_description import (
    CommitDescriptionEnrichment,
)
from kodit.domain.enrichments.request import (
    EnrichmentRequest as GenericEnrichmentRequest,
)
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    GitRepoRepository,
)
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.domain.value_objects import TaskOperation, TrackableType

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )

COMMIT_DESCRIPTION_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a git commit diff.
Please provide a concise description of what changes were made and why.
"""


class CommitDescriptionHandler:
    """Handler for creating commit descriptions."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        scanner: GitRepositoryScanner,
        enricher_service: Enricher,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the commit description handler."""
        self.repo_repository = repo_repository
        self.scanner = scanner
        self.enricher_service = enricher_service
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute commit description generation operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.CREATE_COMMIT_DESCRIPTION_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Check if commit description already exists for this commit
            if await self.enrichment_query_service.has_commit_description_for_commit(
                commit_sha
            ):
                await step.skip("Commit description already exists for commit")
                return

            # Get repository path
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            await step.set_total(3)
            await step.set_current(1, "Getting commit diff")

            # Get the diff for this commit
            diff = await self.scanner.git_adapter.get_commit_diff(
                repo.cloned_path, commit_sha
            )

            if not diff or len(diff.strip()) == 0:
                await step.skip("No diff found for commit")
                return

            await step.set_current(2, "Enriching commit description with LLM")

            # Enrich the diff through the enricher
            enrichment_request = GenericEnrichmentRequest(
                id=commit_sha,
                text=diff,
                system_prompt=COMMIT_DESCRIPTION_SYSTEM_PROMPT,
            )

            enriched_content = ""
            async for response in self.enricher_service.enrich([enrichment_request]):
                enriched_content = response.text

            # Create and save commit description enrichment
            enrichment = await self.enrichment_v2_repository.save(
                CommitDescriptionEnrichment(
                    content=enriched_content,
                )
            )
            if not enrichment or not enrichment.id:
                raise ValueError(
                    f"Failed to save commit description enrichment for commit "
                    f"{commit_sha}"
                )
            await self.enrichment_association_repository.save(
                CommitEnrichmentAssociation(
                    enrichment_id=enrichment.id,
                    entity_id=commit_sha,
                )
            )

            await step.set_current(3, "Commit description enrichment completed")
