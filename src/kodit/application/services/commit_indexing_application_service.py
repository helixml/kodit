"""Application services for commit indexing operations."""

from collections import defaultdict
from pathlib import Path

import structlog
from pydantic import AnyUrl

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.architecture.architecture import (
    ENRICHMENT_TYPE_ARCHITECTURE,
)
from kodit.domain.enrichments.architecture.physical.physical import (
    ENRICHMENT_SUBTYPE_PHYSICAL,
    PhysicalArchitectureEnrichment,
)
from kodit.domain.enrichments.development.development import ENRICHMENT_TYPE_DEVELOPMENT
from kodit.domain.enrichments.development.snippet.snippet import (
    ENRICHMENT_SUBTYPE_SNIPPET,
    ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY,
    SnippetEnrichment,
    SnippetEnrichmentSummary,
)
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.enrichment import EnrichmentAssociation, EnrichmentV2
from kodit.domain.enrichments.request import (
    EnrichmentRequest as GenericEnrichmentRequest,
)
from kodit.domain.enrichments.usage.api_docs import ENRICHMENT_SUBTYPE_API_DOCS
from kodit.domain.enrichments.usage.usage import ENRICHMENT_TYPE_USAGE
from kodit.domain.entities import Task
from kodit.domain.entities.git import GitFile, GitRepo, SnippetV2
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    GitBranchRepository,
    GitCommitRepository,
    GitFileRepository,
    GitRepoRepository,
    GitTagRepository,
)
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.domain.services.physical_architecture_service import (
    ARCHITECTURE_ENRICHMENT_SYSTEM_PROMPT,
    ARCHITECTURE_ENRICHMENT_TASK_PROMPT,
    PhysicalArchitectureService,
)
from kodit.domain.value_objects import (
    DeleteRequest,
    Document,
    IndexRequest,
    LanguageMapping,
    PrescribedOperations,
    QueuePriority,
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.slicing.api_doc_extractor import APIDocExtractor
from kodit.infrastructure.slicing.slicer import Slicer
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType
from kodit.infrastructure.sqlalchemy.query import (
    EnrichmentAssociationQueryBuilder,
    EnrichmentQueryBuilder,
    FilterOperator,
    QueryBuilder,
)

SUMMARIZATION_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""


class CommitIndexingApplicationService:
    """Application service for commit indexing operations."""

    def __init__(  # noqa: PLR0913
        self,
        repo_repository: GitRepoRepository,
        git_commit_repository: GitCommitRepository,
        git_file_repository: GitFileRepository,
        git_branch_repository: GitBranchRepository,
        git_tag_repository: GitTagRepository,
        operation: ProgressTracker,
        scanner: GitRepositoryScanner,
        cloner: RepositoryCloner,
        slicer: Slicer,
        queue: QueueService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        embedding_repository: SqlAlchemyEmbeddingRepository,
        architecture_service: PhysicalArchitectureService,
        enricher_service: Enricher,
        enrichment_v2_repository: EnrichmentV2Repository,
        enrichment_association_repository: EnrichmentAssociationRepository,
    ) -> None:
        """Initialize the commit indexing application service.

        Args:
            commit_index_repository: Repository for commit index data.
            snippet_v2_repository: Repository for snippet data.
            repo_repository: Repository for Git repository data.
            domain_indexer: Domain service for indexing operations.
            operation: Progress tracker for reporting operations.

        """
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.git_file_repository = git_file_repository
        self.git_branch_repository = git_branch_repository
        self.git_tag_repository = git_tag_repository
        self.operation = operation
        self.scanner = scanner
        self.cloner = cloner
        self.slicer = slicer
        self.queue = queue
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.embedding_repository = embedding_repository
        self.architecture_service = architecture_service
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.enricher_service = enricher_service
        self._log = structlog.get_logger(__name__)

    async def create_git_repository(self, remote_uri: AnyUrl) -> GitRepo:
        """Create a new Git repository."""
        async with self.operation.create_child(
            TaskOperation.CREATE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
        ):
            repo = GitRepoFactory.create_from_remote_uri(remote_uri)
            repo = await self.repo_repository.save(repo)
            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.CREATE_NEW_REPOSITORY,
                base_priority=QueuePriority.USER_INITIATED,
                payload={"repository_id": repo.id},
            )
            return repo

    async def delete_git_repository(self, repo_id: int) -> bool:
        """Delete a Git repository by ID."""
        repo = await self.repo_repository.get(repo_id)
        if not repo:
            return False

        # Use the proper deletion process that handles all dependencies
        await self.process_delete_repo(repo_id)
        return True

    # TODO(Phil): Make this polymorphic
    async def run_task(self, task: Task) -> None:  # noqa: PLR0912, C901
        """Run a task."""
        if task.type.is_repository_operation():
            repo_id = task.payload["repository_id"]
            if not repo_id:
                raise ValueError("Repository ID is required")
            if task.type == TaskOperation.CLONE_REPOSITORY:
                await self.process_clone_repo(repo_id)
            elif task.type == TaskOperation.SCAN_REPOSITORY:
                await self.process_scan_repo(repo_id)
            elif task.type == TaskOperation.DELETE_REPOSITORY:
                await self.process_delete_repo(repo_id)
            else:
                raise ValueError(f"Unknown task type: {task.type}")
        elif task.type.is_commit_operation():
            repository_id = task.payload["repository_id"]
            if not repository_id:
                raise ValueError("Repository ID is required")
            commit_sha = task.payload["commit_sha"]
            if not commit_sha:
                raise ValueError("Commit SHA is required")
            if task.type == TaskOperation.EXTRACT_SNIPPETS_FOR_COMMIT:
                await self.process_snippets_for_commit(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_BM25_INDEX_FOR_COMMIT:
                await self.process_bm25_index(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_CODE_EMBEDDINGS_FOR_COMMIT:
                await self.process_code_embeddings(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT:
                await self.process_enrich(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_SUMMARY_EMBEDDINGS_FOR_COMMIT:
                await self.process_summary_embeddings(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_ARCHITECTURE_ENRICHMENT_FOR_COMMIT:
                await self.process_architecture_discovery(repository_id, commit_sha)
            elif task.type == TaskOperation.CREATE_PUBLIC_API_DOCS_FOR_COMMIT:
                await self.process_api_docs(repository_id, commit_sha)
            else:
                raise ValueError(f"Unknown task type: {task.type}")
        else:
            raise ValueError(f"Unknown task type: {task.type}")

    async def process_clone_repo(self, repository_id: int) -> None:
        """Clone a repository."""
        async with self.operation.create_child(
            TaskOperation.CLONE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            repo = await self.repo_repository.get(repository_id)
            repo.cloned_path = await self.cloner.clone_repository(repo.remote_uri)
            await self.repo_repository.save(repo)

    async def process_scan_repo(self, repository_id: int) -> None:
        """Scan a repository."""
        async with self.operation.create_child(
            TaskOperation.SCAN_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            await step.set_total(7)
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            # Scan the repository to get all metadata
            await step.set_current(0, "Scanning repository")
            scan_result = await self.scanner.scan_repository(
                repo.cloned_path, repository_id
            )

            # Update repo with scan result (this sets num_commits, num_branches, etc.)
            await step.set_current(1, "Updating repository with scan result")
            repo.update_with_scan_result(scan_result)
            await self.repo_repository.save(repo)

            await step.set_current(2, "Saving files")
            all_files = [
                file for commit in scan_result.all_commits for file in commit.files
            ]
            await self.git_file_repository.save_bulk(all_files)

            await step.set_current(3, "Saving commits")
            await self.git_commit_repository.save_bulk(scan_result.all_commits)

            await step.set_current(4, "Saving branches")
            if scan_result.branches:
                await self.git_branch_repository.save_bulk(
                    scan_result.branches,
                )

            await step.set_current(5, "Saving tags")
            if scan_result.all_tags:
                await self.git_tag_repository.save_bulk(
                    scan_result.all_tags,
                )

            await step.set_current(6, "Enqueuing commit indexing tasks")
            if not repo.tracking_branch:
                raise ValueError(f"Repository {repository_id} has no tracking branch")
            commit_sha = repo.tracking_branch.head_commit_sha
            if not commit_sha:
                raise ValueError(f"Repository {repository_id} has no head commit")

            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.INDEX_COMMIT,
                base_priority=QueuePriority.USER_INITIATED,
                payload={"commit_sha": commit_sha, "repository_id": repository_id},
            )

    async def _delete_snippet_enrichments_for_commits(
        self, commit_shas: list[str]
    ) -> None:
        """Delete snippet enrichments and their indices for commits."""
        # Get all snippet enrichment IDs for these commits
        all_snippet_enrichment_ids = []
        for commit_sha in commit_shas:
            snippet_enrichments = await self._snippets_for_commit(commit_sha)
            enrichment_ids = [
                enrichment.id for enrichment in snippet_enrichments if enrichment.id
            ]
            all_snippet_enrichment_ids.extend(enrichment_ids)

        if not all_snippet_enrichment_ids:
            return

        # Delete from BM25 and embedding indices
        snippet_id_strings = [str(sid) for sid in all_snippet_enrichment_ids]
        delete_request = DeleteRequest(snippet_ids=snippet_id_strings)
        await self.bm25_service.delete_documents(delete_request)

        for snippet_id in all_snippet_enrichment_ids:
            await self.embedding_repository.delete_embeddings_by_snippet_id(
                str(snippet_id)
            )

        # Delete enrichment associations for snippets
        await self.enrichment_association_repository.delete_by_query(
            QueryBuilder()
            .filter("entity_type", FilterOperator.EQ, "snippet_v2")
            .filter("entity_id", FilterOperator.IN, snippet_id_strings)
        )

        # Delete the enrichments themselves
        await self.enrichment_v2_repository.delete_by_query(
            QueryBuilder().filter("id", FilterOperator.IN, all_snippet_enrichment_ids)
        )

    async def _delete_commit_enrichments(self, commit_shas: list[str]) -> None:
        """Delete commit-level enrichments for commits."""
        existing_enrichment_associations = (
            await self.enrichment_association_repository.find(
                QueryBuilder()
                .filter("entity_type", FilterOperator.EQ, "git_commit")
                .filter("entity_id", FilterOperator.IN, commit_shas)
            )
        )
        enrichment_ids = [a.enrichment_id for a in existing_enrichment_associations]
        if not enrichment_ids:
            return

        # Delete associations first
        await self.enrichment_association_repository.delete_by_query(
            QueryBuilder().filter("enrichment_id", FilterOperator.IN, enrichment_ids)
        )
        # Then delete enrichments
        await self.enrichment_v2_repository.delete_by_query(
            QueryBuilder().filter("id", FilterOperator.IN, enrichment_ids)
        )

    async def process_delete_repo(self, repository_id: int) -> None:
        """Delete a repository."""
        async with self.operation.create_child(
            TaskOperation.DELETE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            repo = await self.repo_repository.get(repository_id)
            if not repo:
                raise ValueError(f"Repository {repository_id} not found")

            # Get all commit SHAs for this repository
            commits = await self.git_commit_repository.find(
                QueryBuilder().filter("repo_id", FilterOperator.EQ, repository_id)
            )
            commit_shas = [commit.commit_sha for commit in commits]

            # Delete all enrichments and their indices
            if commit_shas:
                await self._delete_snippet_enrichments_for_commits(commit_shas)
                await self._delete_commit_enrichments(commit_shas)

            # Delete branches, tags, files, commits, and repository
            await self.git_branch_repository.delete_by_repo_id(repository_id)
            await self.git_tag_repository.delete_by_repo_id(repository_id)

            for commit_sha in commit_shas:
                await self.git_file_repository.delete_by_commit_sha(commit_sha)

            await self.git_commit_repository.delete_by_query(
                QueryBuilder().filter("repo_id", FilterOperator.EQ, repository_id)
            )

            if repo.id:
                await self.repo_repository.delete(repo)

    async def process_snippets_for_commit(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Generate snippets for a repository."""
        async with self.operation.create_child(
            operation=TaskOperation.EXTRACT_SNIPPETS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Find existing snippet enrichments for this commit
            existing_enrichments = await self._snippets_for_commit(commit_sha)
            if existing_enrichments:
                await step.skip("Snippets already extracted for commit")
                return

            commit = await self.git_commit_repository.get(commit_sha)

            # Load files on demand for snippet extraction (performance optimization)
            # Instead of using commit.files (which may be empty), load files directly
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            files_data = await self.scanner.git_adapter.get_commit_file_data(
                repo.cloned_path, commit_sha
            )

            # Create GitFile entities with absolute paths for the slicer
            files = []
            for file_data in files_data:
                # Extract extension from file path
                file_path = Path(file_data["path"])
                extension = file_path.suffix.lstrip(".")

                # Create absolute path for the slicer to read
                absolute_path = str(repo.cloned_path / file_data["path"])

                git_file = GitFile(
                    commit_sha=commit.commit_sha,
                    created_at=file_data.get("created_at", commit.date),
                    blob_sha=file_data["blob_sha"],
                    path=absolute_path,  # Use absolute path for file reading
                    mime_type=file_data.get("mime_type", "application/octet-stream"),
                    size=file_data.get("size", 0),
                    extension=extension,
                )
                files.append(git_file)

            # Create a set of languages to extract snippets for
            extensions = {file.extension for file in files}
            lang_files_map: dict[str, list[GitFile]] = defaultdict(list)
            for ext in extensions:
                try:
                    lang = LanguageMapping.get_language_for_extension(ext)
                    lang_files_map[lang].extend(
                        file for file in files if file.extension == ext
                    )
                except ValueError as e:
                    self._log.debug("Skipping", error=str(e))
                    continue

            # Extract snippets
            all_snippets: list[SnippetV2] = []
            slicer = Slicer()
            await step.set_total(len(lang_files_map.keys()))
            for i, (lang, lang_files) in enumerate(lang_files_map.items()):
                await step.set_current(i, f"Extracting snippets for {lang}")
                snippets = slicer.extract_snippets_from_git_files(
                    lang_files, language=lang
                )
                all_snippets.extend(snippets)

            # Deduplicate snippets by SHA before saving to prevent constraint violations
            unique_snippets: dict[str, SnippetV2] = {}
            for snippet in all_snippets:
                unique_snippets[snippet.sha] = snippet

            deduplicated_snippets = list(unique_snippets.values())

            commit_short = commit.commit_sha[:8]
            self._log.info(
                f"Extracted {len(all_snippets)} snippets, "
                f"deduplicated to {len(deduplicated_snippets)} for {commit_short}"
            )

            saved_enrichments = await self.enrichment_v2_repository.save_bulk(
                [
                    SnippetEnrichment(content=snippet.content)
                    for snippet in deduplicated_snippets
                ]
            )
            saved_associations = await self.enrichment_association_repository.save_bulk(
                [
                    EnrichmentAssociation(
                        enrichment_id=enrichment.id,
                        entity_type=db_entities.GitCommit.__tablename__,
                        entity_id=commit_sha,
                    )
                    for enrichment in saved_enrichments
                    if enrichment.id
                ]
            )
            self._log.info(
                f"Saved {len(saved_enrichments)} snippet enrichments and "
                f"{len(saved_associations)} associations for commit {commit_sha}"
            )

    async def process_bm25_index(self, repository_id: int, commit_sha: str) -> None:
        """Handle BM25_INDEX task - create keyword index."""
        async with self.operation.create_child(
            TaskOperation.CREATE_BM25_INDEX_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            existing_enrichments = await self._snippets_for_commit(commit_sha)
            await self.bm25_service.index_documents(
                IndexRequest(
                    documents=[
                        Document(snippet_id=str(snippet.id), text=snippet.content)
                        for snippet in existing_enrichments
                        if snippet.id
                    ]
                )
            )

    async def process_code_embeddings(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle CODE_EMBEDDINGS task - create code embeddings."""
        async with self.operation.create_child(
            TaskOperation.CREATE_CODE_EMBEDDINGS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            existing_enrichments = await self._snippets_for_commit(commit_sha)

            new_snippets = await self._new_snippets_for_type(
                existing_enrichments, EmbeddingType.CODE
            )
            if not new_snippets:
                await step.skip("All snippets already have code embeddings")
                return

            await step.set_total(len(new_snippets))
            processed = 0
            documents = [
                Document(snippet_id=str(snippet.id), text=snippet.content)
                for snippet in new_snippets
                if snippet.id
            ]
            async for result in self.code_search_service.index_documents(
                IndexRequest(documents=documents)
            ):
                processed += len(result)
                await step.set_current(processed, "Creating code embeddings for commit")

    async def process_enrich(self, repository_id: int, commit_sha: str) -> None:
        """Handle ENRICH task - enrich snippets and create text embeddings."""
        async with self.operation.create_child(
            TaskOperation.CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            existing_summary_enrichments = await self._summary_enrichments_for_commit(
                commit_sha
            )
            if existing_summary_enrichments:
                await step.skip("Summary enrichments already exist for commit")
                return

            all_snippets = await self._snippets_for_commit(commit_sha)
            if not all_snippets:
                await step.skip("No snippets to enrich")
                return

            # Enrich snippets
            await step.set_total(len(all_snippets))
            snippet_map = {
                str(snippet.id): snippet for snippet in all_snippets if snippet.id
            }

            enrichment_requests = [
                GenericEnrichmentRequest(
                    id=str(snippet_id),
                    text=snippet.content,
                    system_prompt=SUMMARIZATION_SYSTEM_PROMPT,
                )
                for snippet_id, snippet in snippet_map.items()
            ]

            processed = 0
            async for result in self.enricher_service.enrich(enrichment_requests):
                snippet = snippet_map[result.id]
                db_summary = await self.enrichment_v2_repository.save(
                    SnippetEnrichmentSummary(content=result.text)
                )
                if not db_summary.id:
                    raise ValueError(
                        f"Failed to save snippet enrichment for commit {commit_sha}"
                    )
                await self.enrichment_association_repository.save(
                    EnrichmentAssociation(
                        enrichment_id=db_summary.id,
                        entity_type=db_entities.EnrichmentV2.__tablename__,
                        entity_id=str(snippet.id),
                    )
                )
                processed += 1
                await step.set_current(processed, "Enriching snippets for commit")

    async def process_summary_embeddings(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle SUMMARY_EMBEDDINGS task - create summary embeddings."""
        async with self.operation.create_child(
            TaskOperation.CREATE_SUMMARY_EMBEDDINGS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Get all snippet enrichments for this commit
            all_snippet_enrichments = await self._snippets_for_commit(commit_sha)
            if not all_snippet_enrichments:
                await step.skip("No snippets to create summary embeddings")
                return

            # Get summary enrichments that point to these snippet enrichments
            query = EnrichmentAssociationQueryBuilder.for_enrichment_associations(
                entity_type=db_entities.EnrichmentV2.__tablename__,
                entity_ids=[
                    str(snippet.id) for snippet in all_snippet_enrichments if snippet.id
                ],
            )
            summary_enrichment_associations = (
                await self.enrichment_association_repository.find(query)
            )

            if not summary_enrichment_associations:
                await step.skip("No summary enrichments found for snippets")
                return

            # Get the actual summary enrichments
            summary_enrichments = await self.enrichment_v2_repository.find(
                QueryBuilder().filter(
                    "id",
                    FilterOperator.IN,
                    [
                        association.enrichment_id
                        for association in summary_enrichment_associations
                    ],
                )
            )

            # Check if embeddings already exist for these summaries
            new_summaries = await self._new_snippets_for_type(
                summary_enrichments, EmbeddingType.TEXT
            )
            if not new_summaries:
                await step.skip("All snippets already have text embeddings")
                return

            await step.set_total(len(new_summaries))
            processed = 0

            # Create documents from the summary enrichments
            documents_with_summaries = [
                Document(snippet_id=str(summary.id), text=summary.content)
                for summary in new_summaries
                if summary.id
            ]

            async for result in self.text_search_service.index_documents(
                IndexRequest(documents=documents_with_summaries)
            ):
                processed += len(result)
                await step.set_current(processed, "Creating text embeddings for commit")

    async def process_architecture_discovery(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Handle ARCHITECTURE_DISCOVERY task - discover physical architecture."""
        async with self.operation.create_child(
            TaskOperation.CREATE_ARCHITECTURE_ENRICHMENT_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            await step.set_total(3)

            # Check if architecture enrichment already exists for this commit
            existing_architecture_docs = await self._architecture_docs_for_commit(
                commit_sha
            )

            if existing_architecture_docs:
                await step.skip("Architecture enrichment already exists for commit")
                return

            # Get repository path
            repo = await self.repo_repository.get(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            await step.set_current(1, "Discovering physical architecture")

            # Discover architecture
            architecture_narrative = (
                await self.architecture_service.discover_architecture(repo.cloned_path)
            )

            await step.set_current(2, "Enriching architecture notes with LLM")

            # Enrich the architecture narrative through the enricher
            enrichment_request = GenericEnrichmentRequest(
                id=commit_sha,
                text=ARCHITECTURE_ENRICHMENT_TASK_PROMPT.format(
                    architecture_narrative=architecture_narrative,
                ),
                system_prompt=ARCHITECTURE_ENRICHMENT_SYSTEM_PROMPT,
            )

            enriched_content = ""
            async for response in self.enricher_service.enrich([enrichment_request]):
                enriched_content = response.text

            # Create and save architecture enrichment with enriched content
            enrichment = await self.enrichment_v2_repository.save(
                PhysicalArchitectureEnrichment(
                    content=enriched_content,
                )
            )
            if not enrichment or not enrichment.id:
                raise ValueError(
                    f"Failed to save architecture enrichment for commit {commit_sha}"
                )
            await self.enrichment_association_repository.save(
                EnrichmentAssociation(
                    enrichment_id=enrichment.id,  # type: ignore[arg-type]
                    entity_type="git_commit",
                    entity_id=commit_sha,
                )
            )

            await step.set_current(3, "Architecture enrichment completed")

    async def process_api_docs(self, repository_id: int, commit_sha: str) -> None:
        """Handle API_DOCS task - generate API documentation."""
        async with self.operation.create_child(
            TaskOperation.CREATE_PUBLIC_API_DOCS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Check if API docs already exist for this commit
            existing_api_docs = await self._api_docs_for_commit(commit_sha)

            if existing_api_docs:
                await step.skip("API docs already exist for commit")
                return

            # Get repository for metadata
            repo = await self.repo_repository.get(repository_id)
            if not repo:
                raise ValueError(f"Repository {repository_id} not found")
            str(repo.sanitized_remote_uri)

            commit = await self.git_commit_repository.get(commit_sha)

            # Group files by language
            lang_files_map: dict[str, list[GitFile]] = defaultdict(list)
            for file in commit.files:
                try:
                    lang = LanguageMapping.get_language_for_extension(file.extension)
                except ValueError:
                    continue
                lang_files_map[lang].append(file)

            all_enrichments = []
            extractor = APIDocExtractor()

            await step.set_total(len(lang_files_map))
            for i, (lang, lang_files) in enumerate(lang_files_map.items()):
                await step.set_current(i, f"Extracting API docs for {lang}")
                enrichments = extractor.extract_api_docs(
                    files=lang_files,
                    language=lang,
                    include_private=False,
                )
                all_enrichments.extend(enrichments)

            # Save all enrichments
            if all_enrichments:
                saved_enrichments = await self.enrichment_v2_repository.save_bulk(
                    all_enrichments  # type: ignore[arg-type]
                )
                await self.enrichment_association_repository.save_bulk(
                    [
                        EnrichmentAssociation(
                            enrichment_id=enrichment.id,  # type: ignore[arg-type]
                            entity_type="git_commit",
                            entity_id=commit_sha,
                        )
                        for enrichment in saved_enrichments
                        if enrichment.id is not None
                    ]
                )

    async def _new_snippets_for_type(
        self, all_snippets: list[EnrichmentV2], embedding_type: EmbeddingType
    ) -> list[EnrichmentV2]:
        """Get new snippets for a given type."""
        existing_embeddings = (
            await self.embedding_repository.list_embeddings_by_snippet_ids_and_type(
                [str(s.id) for s in all_snippets], embedding_type
            )
        )
        existing_embeddings_by_snippet_id = {
            embedding.snippet_id: embedding for embedding in existing_embeddings
        }
        return [
            s for s in all_snippets if s.id not in existing_embeddings_by_snippet_id
        ]

    # TODO(phil): this should probably be a separate application service
    async def _snippets_for_commit(self, commit_sha: str) -> list[EnrichmentV2]:
        existing_associations = (
            await self.enrichment_association_repository.associations_for_commit(
                commit_sha=commit_sha,
            )
        )
        existing_enrichments = await self.enrichment_v2_repository.find(
            EnrichmentQueryBuilder.for_enrichment(
                enrichment_type=ENRICHMENT_TYPE_DEVELOPMENT,
                enrichment_subtype=ENRICHMENT_SUBTYPE_SNIPPET,
            ).filter(
                "id",
                FilterOperator.IN,
                [association.enrichment_id for association in existing_associations],
            )
        )
        self._log.info(
            f"Found {len(existing_associations)} enrichment associations and "
            f"{len(existing_enrichments)} snippets for commit {commit_sha}"
        )
        return existing_enrichments

    async def _api_docs_for_commit(self, commit_sha: str) -> list[EnrichmentV2]:
        """Get the API docs for the given commit."""
        existing_enrichment_associations = (
            await self.enrichment_association_repository.associations_for_commit(
                commit_sha=commit_sha,
            )
        )

        return await self.enrichment_v2_repository.find(
            QueryBuilder()
            .filter(
                "id",
                FilterOperator.IN,
                [
                    association.enrichment_id
                    for association in existing_enrichment_associations
                ],
            )
            .filter(
                db_entities.EnrichmentV2.type.key,
                FilterOperator.EQ,
                ENRICHMENT_TYPE_USAGE,
            )
            .filter(
                db_entities.EnrichmentV2.subtype.key,
                FilterOperator.EQ,
                ENRICHMENT_SUBTYPE_API_DOCS,
            )
        )

    async def _architecture_docs_for_commit(
        self, commit_sha: str
    ) -> list[EnrichmentV2]:
        """Get the architecture docs for the given commit."""
        existing_enrichment_associations = (
            await self.enrichment_association_repository.associations_for_commit(
                commit_sha=commit_sha,
            )
        )

        return await self.enrichment_v2_repository.find(
            QueryBuilder()
            .filter(
                "id",
                FilterOperator.IN,
                [
                    association.enrichment_id
                    for association in existing_enrichment_associations
                ],
            )
            .filter(
                db_entities.EnrichmentV2.type.key,
                FilterOperator.EQ,
                ENRICHMENT_TYPE_ARCHITECTURE,
            )
            .filter(
                db_entities.EnrichmentV2.subtype.key,
                FilterOperator.EQ,
                ENRICHMENT_SUBTYPE_PHYSICAL,
            )
        )

    async def _summary_enrichments_for_commit(
        self, commit_sha: str
    ) -> list[EnrichmentV2]:
        """Get the summary enrichments for the given commit."""
        existing_enrichment_associations = (
            await self.enrichment_association_repository.associations_for_commit(
                commit_sha=commit_sha,
            )
        )
        return await self.enrichment_v2_repository.find(
            QueryBuilder()
            .filter(
                "id",
                FilterOperator.IN,
                [
                    association.enrichment_id
                    for association in existing_enrichment_associations
                ],
            )
            .filter(
                db_entities.EnrichmentV2.type.key,
                FilterOperator.EQ,
                ENRICHMENT_TYPE_DEVELOPMENT,
            )
            .filter(
                db_entities.EnrichmentV2.subtype.key,
                FilterOperator.EQ,
                ENRICHMENT_SUBTYPE_SNIPPET_SUMMARY,
            )
        )
