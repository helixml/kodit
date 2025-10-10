"""Application services for commit indexing operations."""

from collections import defaultdict
from pathlib import Path

import structlog
from pydantic import AnyUrl

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.enrichments.architecture.physical.physical import (
    PhysicalArchitectureEnrichment,
)
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.request import (
    EnrichmentRequest as GenericEnrichmentRequest,
)
from kodit.domain.enrichments.usage.api_docs import ENRICHMENT_SUBTYPE_API_DOCS
from kodit.domain.enrichments.usage.usage import ENRICHMENT_TYPE_USAGE
from kodit.domain.entities import Task
from kodit.domain.entities.git import GitFile, GitRepo, SnippetV2
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.protocols import (
    GitBranchRepository,
    GitCommitRepository,
    GitRepoRepository,
    GitTagRepository,
    SnippetRepositoryV2,
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
    Enrichment,
    EnrichmentType,
    IndexRequest,
    LanguageMapping,
    PrescribedOperations,
    QueuePriority,
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.slicing.api_doc_extractor import APIDocExtractor
from kodit.infrastructure.slicing.slicer import Slicer
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.enrichment_v2_repository import (
    EnrichmentV2Repository,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType

SUMMARIZATION_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""


class CommitIndexingApplicationService:
    """Application service for commit indexing operations."""

    def __init__(  # noqa: PLR0913
        self,
        snippet_v2_repository: SnippetRepositoryV2,
        repo_repository: GitRepoRepository,
        git_commit_repository: GitCommitRepository,
        git_branch_repository: GitBranchRepository,
        git_tag_repository: GitTagRepository,
        operation: ProgressTracker,
        scanner: GitRepositoryScanner,
        cloner: RepositoryCloner,
        snippet_repository: SnippetRepositoryV2,
        slicer: Slicer,
        queue: QueueService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        embedding_repository: SqlAlchemyEmbeddingRepository,
        architecture_service: PhysicalArchitectureService,
        enrichment_v2_repository: EnrichmentV2Repository,
        enricher_service: Enricher,
    ) -> None:
        """Initialize the commit indexing application service.

        Args:
            commit_index_repository: Repository for commit index data.
            snippet_v2_repository: Repository for snippet data.
            repo_repository: Repository for Git repository data.
            domain_indexer: Domain service for indexing operations.
            operation: Progress tracker for reporting operations.

        """
        self.snippet_repository = snippet_v2_repository
        self.repo_repository = repo_repository
        self.git_commit_repository = git_commit_repository
        self.git_branch_repository = git_branch_repository
        self.git_tag_repository = git_tag_repository
        self.operation = operation
        self.scanner = scanner
        self.cloner = cloner
        self.snippet_repository = snippet_repository
        self.slicer = slicer
        self.queue = queue
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.embedding_repository = embedding_repository
        self.architecture_service = architecture_service
        self.enrichment_v2_repository = enrichment_v2_repository
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
        repo = await self.repo_repository.get_by_id(repo_id)
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
            repo = await self.repo_repository.get_by_id(repository_id)
            repo.cloned_path = await self.cloner.clone_repository(repo.remote_uri)
            await self.repo_repository.save(repo)

    async def process_scan_repo(self, repository_id: int) -> None:
        """Scan a repository."""
        async with self.operation.create_child(
            TaskOperation.SCAN_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            await step.set_total(6)
            repo = await self.repo_repository.get_by_id(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")

            # Scan the repository to get all metadata
            await step.set_current(0, "Scanning repository")
            scan_result = await self.scanner.scan_repository(repo.cloned_path)

            # Update repo with scan result (this sets num_commits, num_branches, etc.)
            await step.set_current(1, "Updating repository with scan result")
            repo.update_with_scan_result(scan_result)
            await self.repo_repository.save(repo)

            # Save commits, branches, and tags to their dedicated repositories
            await step.set_current(2, "Saving commits")
            if scan_result.all_commits:
                await self.git_commit_repository.save_bulk(
                    scan_result.all_commits, repository_id
                )

            await step.set_current(3, "Saving branches")
            if scan_result.branches:
                await self.git_branch_repository.save_bulk(
                    scan_result.branches, repository_id
                )

            await step.set_current(4, "Saving tags")
            if scan_result.all_tags:
                await self.git_tag_repository.save_bulk(
                    scan_result.all_tags, repository_id
                )

            await step.set_current(5, "Enqueuing commit indexing tasks")
            if not repo.tracking_branch:
                raise ValueError(f"Repository {repository_id} has no tracking branch")
            commit_sha = repo.tracking_branch.head_commit.commit_sha
            if not commit_sha:
                raise ValueError(f"Repository {repository_id} has no head commit")

            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.INDEX_COMMIT,
                base_priority=QueuePriority.USER_INITIATED,
                payload={"commit_sha": commit_sha, "repository_id": repository_id},
            )

    async def process_delete_repo(self, repository_id: int) -> None:
        """Delete a repository."""
        async with self.operation.create_child(
            TaskOperation.DELETE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            repo = await self.repo_repository.get_by_id(repository_id)
            if not repo:
                raise ValueError(f"Repository {repository_id} not found")

            # Get all commit SHAs for this repository first (needed for cleanup)
            commits = await self.git_commit_repository.get_by_repo_id(repository_id)
            commit_shas = [commit.commit_sha for commit in commits]

            # Step 1: Get all snippet IDs that are associated with these commits FIRST
            # (before deleting the associations)
            all_snippet_ids = []
            if commit_shas:
                for commit_sha in commit_shas:
                    snippets = await self.snippet_repository.get_snippets_for_commit(
                        commit_sha
                    )
                    all_snippet_ids.extend(
                        [snippet.id for snippet in snippets if snippet.id]
                    )

            # Step 2: Delete from BM25 and embedding indices
            if all_snippet_ids:
                # Convert to strings as DeleteRequest expects list[str]
                snippet_id_strings = [str(snippet_id) for snippet_id in all_snippet_ids]
                delete_request = DeleteRequest(snippet_ids=snippet_id_strings)
                await self.bm25_service.delete_documents(delete_request)

                # Delete embeddings for each snippet
                for snippet_id in all_snippet_ids:
                    await self.embedding_repository.delete_embeddings_by_snippet_id(
                        snippet_id
                    )

            # Step 3: Delete enrichments for all commits
            if commit_shas:
                await self.enrichment_v2_repository.bulk_delete_enrichments(
                    entity_type="git_commit",
                    entity_ids=commit_shas,
                )

            # Step 4: Delete snippet associations for all commits
            for commit_sha in commit_shas:
                await self.snippet_repository.delete_snippets_for_commit(commit_sha)

            # Step 5: Delete branches (they reference commits via head_commit_sha)
            await self.git_branch_repository.delete_by_repo_id(repository_id)

            # Step 6: Delete tags (they reference commits via target_commit_sha)
            await self.git_tag_repository.delete_by_repo_id(repository_id)

            # Step 7: Delete commits and their files
            await self.git_commit_repository.delete_by_repo_id(repository_id)

            # Step 8: Finally delete the repository
            await self.repo_repository.delete(repo.sanitized_remote_uri)

    async def process_snippets_for_commit(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Generate snippets for a repository."""
        async with self.operation.create_child(
            operation=TaskOperation.EXTRACT_SNIPPETS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            # Have we already processed this commit? If yes, skip.
            if await self.snippet_repository.get_snippets_for_commit(commit_sha):
                await step.skip("All snippets already extracted for commit")
                return

            commit = await self.git_commit_repository.get_by_sha(commit_sha)

            # Load files on demand for snippet extraction (performance optimization)
            # Instead of using commit.files (which may be empty), load files directly
            repo = await self.repo_repository.get_by_id(repository_id)
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
            await self.snippet_repository.save_snippets(
                commit.commit_sha, deduplicated_snippets
            )

    async def process_bm25_index(self, repository_id: int, commit_sha: str) -> None:
        """Handle BM25_INDEX task - create keyword index."""
        async with self.operation.create_child(
            TaskOperation.CREATE_BM25_INDEX_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ):
            snippets = await self.snippet_repository.get_snippets_for_commit(commit_sha)

            await self.bm25_service.index_documents(
                IndexRequest(
                    documents=[
                        Document(snippet_id=snippet.id, text=snippet.content)
                        for snippet in snippets
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
            all_snippets = await self.snippet_repository.get_snippets_for_commit(
                commit_sha
            )

            new_snippets = await self._new_snippets_for_type(
                all_snippets, EmbeddingType.CODE
            )
            if not new_snippets:
                await step.skip("All snippets already have code embeddings")
                return

            await step.set_total(len(new_snippets))
            processed = 0
            documents = [
                Document(snippet_id=snippet.id, text=snippet.content)
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
            all_snippets = await self.snippet_repository.get_snippets_for_commit(
                commit_sha
            )

            # Find snippets without a summary enrichment
            snippets_without_summary = [
                snippet
                for snippet in all_snippets
                if not snippet.enrichments
                or not next(
                    enrichment
                    for enrichment in snippet.enrichments
                    if enrichment.type == EnrichmentType.SUMMARIZATION
                )
            ]
            if not snippets_without_summary:
                await step.skip("All snippets already have a summary enrichment")
                return

            # Enrich snippets
            await step.set_total(len(snippets_without_summary))
            snippet_map = {
                snippet.id: snippet
                for snippet in snippets_without_summary
                if snippet.id
            }

            enrichment_requests = [
                GenericEnrichmentRequest(
                    id=snippet_id,
                    text=snippet.content,
                    system_prompt=SUMMARIZATION_SYSTEM_PROMPT,
                )
                for snippet_id, snippet in snippet_map.items()
            ]

            processed = 0
            async for result in self.enricher_service.enrich(enrichment_requests):
                snippet = snippet_map[result.id]
                snippet.enrichments.append(
                    Enrichment(type=EnrichmentType.SUMMARIZATION, content=result.text)
                )

                await self.snippet_repository.save_snippets(commit_sha, [snippet])

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
            snippets = await self.snippet_repository.get_snippets_for_commit(commit_sha)

            new_snippets = await self._new_snippets_for_type(
                snippets, EmbeddingType.TEXT
            )
            if not new_snippets:
                await step.skip("All snippets already have text embeddings")
                return

            await step.set_total(len(new_snippets))
            processed = 0

            def _summary_from_enrichments(enrichments: list[Enrichment]) -> str:
                if not enrichments:
                    return ""
                return next(
                    enrichment.content
                    for enrichment in enrichments
                    if enrichment.type == EnrichmentType.SUMMARIZATION
                )

            snippet_summary_map = {
                snippet.id: _summary_from_enrichments(snippet.enrichments)
                for snippet in snippets
                if snippet.id
            }
            if len(snippet_summary_map) == 0:
                await step.skip("No snippets with summaries to create text embeddings")
                return

            documents_with_summaries = [
                Document(snippet_id=snippet_id, text=snippet_summary)
                for snippet_id, snippet_summary in snippet_summary_map.items()
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
            enrichment_repo = self.enrichment_v2_repository
            existing_enrichments = await enrichment_repo.enrichments_for_entity_type(
                entity_type="git_commit",
                entity_ids=[commit_sha],
            )

            # Check if architecture enrichment already exists
            has_architecture = any(
                enrichment.type == "architecture" for enrichment in existing_enrichments
            )

            if has_architecture:
                await step.skip("Architecture enrichment already exists for commit")
                return

            # Get repository path
            repo = await self.repo_repository.get_by_id(repository_id)
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
            architecture_enrichment = PhysicalArchitectureEnrichment(
                entity_id=commit_sha,
                content=enriched_content,
            )

            await self.enrichment_v2_repository.bulk_save_enrichments(
                [architecture_enrichment]
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
            existing_enrichments = (
                await self.enrichment_v2_repository.enrichments_for_entity_type(
                    entity_type="git_commit",
                    entity_ids=[commit_sha],
                )
            )

            has_api_docs = any(
                e.type == ENRICHMENT_TYPE_USAGE
                and e.subtype == ENRICHMENT_SUBTYPE_API_DOCS
                for e in existing_enrichments
            )

            if has_api_docs:
                await step.skip("API docs already exist for commit")
                return

            # Get repository for metadata
            repo = await self.repo_repository.get_by_id(repository_id)
            if not repo:
                raise ValueError(f"Repository {repository_id} not found")
            str(repo.sanitized_remote_uri)

            commit = await self.git_commit_repository.get_by_sha(commit_sha)

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
                    lang_files,
                    lang,
                    commit_sha,
                    include_private=False,
                )
                all_enrichments.extend(enrichments)

            # Save all enrichments
            if all_enrichments:
                await self.enrichment_v2_repository.bulk_save_enrichments(
                    all_enrichments
                )

    async def _new_snippets_for_type(
        self, all_snippets: list[SnippetV2], embedding_type: EmbeddingType
    ) -> list[SnippetV2]:
        """Get new snippets for a given type."""
        existing_embeddings = (
            await self.embedding_repository.list_embeddings_by_snippet_ids_and_type(
                [s.id for s in all_snippets], embedding_type
            )
        )
        existing_embeddings_by_snippet_id = {
            embedding.snippet_id: embedding for embedding in existing_embeddings
        }
        return [
            s for s in all_snippets if s.id not in existing_embeddings_by_snippet_id
        ]
