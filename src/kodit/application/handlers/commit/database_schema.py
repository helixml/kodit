"""Handler for discovering database schema for a commit."""

from typing import TYPE_CHECKING, Any

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.architecture.database_schema.database_schema import (
    DatabaseSchemaEnrichment,
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
from kodit.domain.value_objects import TaskOperation, TrackableType
from kodit.infrastructure.database_schema.database_schema_detector import (
    DatabaseSchemaDetector,
)

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )

DATABASE_SCHEMA_SYSTEM_PROMPT = """
You are an expert database architect and documentation specialist.
Your task is to create clear, visual documentation of database schemas.
"""

DATABASE_SCHEMA_TASK_PROMPT = """
You will be provided with a database schema discovery report.
Please create comprehensive database schema documentation.

<schema_report>
{schema_report}
</schema_report>

**Return the following:**

## Entity List

For each table/entity, write one line:
- **[Table Name]**: [brief description of what it stores]

## Mermaid ERD

Create a Mermaid Entity Relationship Diagram showing:
- All entities (tables)
- Key relationships between entities (if apparent from names or common patterns)
- Use standard ERD notation

Example format:
```mermaid
erDiagram
    User ||--o{{ Order : places
    User {{
        int id PK
        string email
        string name
    }}
    Order {{
        int id PK
        int user_id FK
        datetime created_at
    }}
```

If specific field details aren't available, show just the entity boxes and
relationships.

## Key Observations

Answer these questions in 1-2 sentences each:
1. What is the primary data model pattern (e.g., user-centric,
   event-sourced, multi-tenant)?
2. What migration strategy is being used?
3. Are there any notable database design patterns or concerns?

## Rules:
- Be concise and focus on the high-level structure
- Infer reasonable relationships from table names when explicit information
  isn't available
- If no database schema is found, state that clearly
- Keep entity descriptions to 10 words or less
"""


class DatabaseSchemaHandler:
    """Handler for discovering database schema for a commit."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        database_schema_detector: DatabaseSchemaDetector,
        enricher_service: Enricher,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
        enrichment_query_service: "EnrichmentQueryService",
        operation: ProgressTracker,
    ) -> None:
        """Initialize the database schema handler."""
        self.repo_repository = repo_repository
        self.database_schema_detector = database_schema_detector
        self.enricher_service = enricher_service
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enrichment_query_service = enrichment_query_service
        self.operation = operation

    async def execute(self, payload: dict[str, Any]) -> None:
        """Execute database schema discovery operation."""
        repository_id = payload["repository_id"]
        commit_sha = payload["commit_sha"]

        async with self.operation.create_child(
            TaskOperation.CREATE_DATABASE_SCHEMA_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Check if database schema already exists for this commit
            if await self.enrichment_query_service.has_database_schema_for_commit(
                commit_sha
            ):
                await step.skip("Database schema already exists for commit")
                return

            # Get repository path
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            await step.set_total(3)
            await step.set_current(1, "Discovering database schemas")

            # Discover database schemas
            schema_report = await self.database_schema_detector.discover_schemas(
                repo.cloned_path
            )

            if "No database schemas detected" in schema_report:
                await step.skip("No database schemas found in repository")
                return

            await step.set_current(2, "Enriching schema documentation with LLM")

            # Enrich the schema report through the enricher
            enrichment_request = GenericEnrichmentRequest(
                id=commit_sha,
                text=DATABASE_SCHEMA_TASK_PROMPT.format(schema_report=schema_report),
                system_prompt=DATABASE_SCHEMA_SYSTEM_PROMPT,
            )

            enriched_content = ""
            async for response in self.enricher_service.enrich([enrichment_request]):
                enriched_content = response.text

            # Create and save database schema enrichment
            enrichment = await self.enrichment_v2_repository.save(
                DatabaseSchemaEnrichment(
                    content=enriched_content,
                )
            )
            if not enrichment or not enrichment.id:
                raise ValueError(
                    f"Failed to save database schema enrichment for commit {commit_sha}"
                )
            await self.enrichment_association_repository.save(
                CommitEnrichmentAssociation(
                    enrichment_id=enrichment.id,
                    entity_id=commit_sha,
                )
            )

            await step.set_current(3, "Database schema enrichment completed")
