"""Task handler protocols and implementations for the Command Pattern."""

from typing import TYPE_CHECKING, Protocol

from kodit.application.services.repository_deletion_service import (
    RepositoryDeletionService,
)
from kodit.application.services.repository_lifecycle_service import (
    RepositoryLifecycleService,
)
from kodit.domain.entities import Task

if TYPE_CHECKING:
    from kodit.application.services.commit_indexing_application_service import (
            CommitIndexingApplicationService,
        )


class TaskHandler(Protocol):
    """Protocol for task handlers."""

    async def handle(self, task: Task) -> None:
        """Handle a task."""
        ...


class RepositoryTaskHandler:
    """Handles repository lifecycle tasks."""

    def __init__(
        self,
        lifecycle_service: RepositoryLifecycleService,
        deletion_service: RepositoryDeletionService,
    ) -> None:
        """Initialize the repository task handler."""
        self.lifecycle_service = lifecycle_service
        self.deletion_service = deletion_service

    async def handle(self, task: Task) -> None:
        """Handle repository tasks."""
        from kodit.domain.value_objects import TaskOperation

        repo_id = task.payload["repository_id"]
        if not repo_id:
            raise ValueError("Repository ID is required")

        if task.type == TaskOperation.CLONE_REPOSITORY:
            await self.lifecycle_service.clone_repository(repo_id)
        elif task.type == TaskOperation.SYNC_REPOSITORY:
            await self.lifecycle_service.sync_repository(repo_id)
        elif task.type == TaskOperation.DELETE_REPOSITORY:
            await self.deletion_service.delete_repository(repo_id)
        else:
            raise ValueError(f"Unknown repository task type: {task.type}")


class CommitTaskHandler:
    """Handles commit-level tasks."""

    def __init__(self, commit_service: "CommitIndexingApplicationService") -> None:
        """Initialize the commit task handler."""
        self.commit_service = commit_service

    async def handle(self, task: Task) -> None:  # noqa: C901, PLR0912
        """Handle commit tasks."""
        from kodit.domain.value_objects import TaskOperation

        repository_id = task.payload["repository_id"]
        if not repository_id:
            raise ValueError("Repository ID is required")
        commit_sha = task.payload["commit_sha"]
        if not commit_sha:
            raise ValueError("Commit SHA is required")

        if task.type == TaskOperation.SCAN_COMMIT:
            await self.commit_service.process_scan_commit(repository_id, commit_sha)
        elif task.type == TaskOperation.EXTRACT_SNIPPETS_FOR_COMMIT:
            await self.commit_service.process_snippets_for_commit(
                repository_id, commit_sha
            )
        elif task.type == TaskOperation.CREATE_BM25_INDEX_FOR_COMMIT:
            await self.commit_service.process_bm25_index(repository_id, commit_sha)
        elif task.type == TaskOperation.CREATE_CODE_EMBEDDINGS_FOR_COMMIT:
            await self.commit_service.process_code_embeddings(repository_id, commit_sha)
        elif task.type == TaskOperation.CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT:
            await self.commit_service.process_enrich(repository_id, commit_sha)
        elif task.type == TaskOperation.CREATE_SUMMARY_EMBEDDINGS_FOR_COMMIT:
            await self.commit_service.process_summary_embeddings(
                repository_id, commit_sha
            )
        elif task.type == TaskOperation.CREATE_ARCHITECTURE_ENRICHMENT_FOR_COMMIT:
            await self.commit_service.process_architecture_discovery(
                repository_id, commit_sha
            )
        elif task.type == TaskOperation.CREATE_PUBLIC_API_DOCS_FOR_COMMIT:
            await self.commit_service.process_api_docs(repository_id, commit_sha)
        elif task.type == TaskOperation.CREATE_COMMIT_DESCRIPTION_FOR_COMMIT:
            await self.commit_service.process_commit_description(
                repository_id, commit_sha
            )
        elif task.type == TaskOperation.CREATE_DATABASE_SCHEMA_FOR_COMMIT:
            await self.commit_service.process_database_schema(repository_id, commit_sha)
        elif task.type == TaskOperation.CREATE_COOKBOOK_FOR_COMMIT:
            await self.commit_service.process_cookbook(repository_id, commit_sha)
        else:
            raise ValueError(f"Unknown commit task type: {task.type}")


# Forward reference for type hints
if False:  # TYPE_CHECKING
    pass
