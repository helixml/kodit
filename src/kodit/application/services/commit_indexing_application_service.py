"""Application services for commit indexing operations."""

from collections import defaultdict

import structlog
from pydantic import AnyUrl

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities import Task
from kodit.domain.entities.git import GitFile, GitRepo, SnippetV2
from kodit.domain.protocols import (
    GitRepoRepository,
    SnippetRepositoryV2,
)
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.domain.value_objects import (
    Document,
    Enrichment,
    EnrichmentIndexRequest,
    EnrichmentRequest,
    EnrichmentType,
    IndexRequest,
    LanguageMapping,
    PrescribedOperations,
    QueuePriority,
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.slicing.slicer import Slicer
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType


class CommitIndexingApplicationService:
    """Application service for commit indexing operations."""

    def __init__(  # noqa: PLR0913
        self,
        snippet_v2_repository: SnippetRepositoryV2,
        repo_repository: GitRepoRepository,
        operation: ProgressTracker,
        scanner: GitRepositoryScanner,
        cloner: RepositoryCloner,
        snippet_repository: SnippetRepositoryV2,
        slicer: Slicer,
        queue: QueueService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        enrichment_service: EnrichmentDomainService,
        embedding_repository: SqlAlchemyEmbeddingRepository,
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
        self.operation = operation
        self.scanner = scanner
        self.cloner = cloner
        self.snippet_repository = snippet_repository
        self.slicer = slicer
        self.queue = queue
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.enrichment_service = enrichment_service
        self.embedding_repository = embedding_repository
        self._log = structlog.get_logger(__name__)

    async def create_git_repository(self, remote_uri: AnyUrl) -> GitRepo:
        """Create a new Git repository."""
        async with self.operation.create_child(
            TaskOperation.CREATE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
        ):
            repo = GitRepo.from_remote_uri(remote_uri)
            repo = await self.repo_repository.save(repo)
            await self.queue.enqueue_tasks(
                tasks=PrescribedOperations.CREATE_NEW_REPOSITORY,
                base_priority=QueuePriority.USER_INITIATED,
                payload={"repository_id": repo.id},
            )
            return repo

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
        ):
            repo = await self.repo_repository.get_by_id(repository_id)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repository_id} has never been cloned")
            repo.update_with_scan_result(
                await self.scanner.scan_repository(repo.cloned_path)
            )
            await self.repo_repository.save(repo)

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

    async def process_snippets_for_commit(
        self, repository_id: int, commit_sha: str
    ) -> None:
        """Generate snippets for a repository."""
        async with self.operation.create_child(
            operation=TaskOperation.EXTRACT_SNIPPETS_FOR_COMMIT,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repository_id,
        ) as step:
            commit = await self.repo_repository.get_commit_by_sha(commit_sha)

            # Create a set of languages to extract snippets for
            extensions = {file.extension for file in commit.files}
            lang_files_map: dict[str, list[GitFile]] = defaultdict(list)
            for ext in extensions:
                try:
                    lang = LanguageMapping.get_language_for_extension(ext)
                    lang_files_map[lang].extend(
                        file for file in commit.files if file.extension == ext
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

            self._log.info(
                f"Saving {len(all_snippets)} snippets for commit {commit.commit_sha}"
            )
            await self.snippet_repository.save_snippets(commit.commit_sha, all_snippets)

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

            enrichment_request = EnrichmentIndexRequest(
                requests=[
                    EnrichmentRequest(snippet_id=snippet_id, text=snippet.content)
                    for snippet_id, snippet in snippet_map.items()
                ]
            )

            processed = 0
            async for result in self.enrichment_service.enrich_documents(
                enrichment_request
            ):
                snippet = snippet_map[result.snippet_id]
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
