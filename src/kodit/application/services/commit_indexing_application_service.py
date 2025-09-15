"""Application services for commit indexing operations."""

from collections import defaultdict

import structlog
from pydantic import AnyUrl

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities import Task
from kodit.domain.entities.git import GitFile, GitRepo, SnippetV2
from kodit.domain.protocols import (
    CommitIndexRepository,
    GitRepoRepository,
    SnippetRepositoryV2,
)
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.domain.services.index_service import IndexDomainService
from kodit.domain.value_objects import (
    Document,
    IndexRequest,
    LanguageMapping,
    PrescribedOperations,
    QueuePriority,
    TaskOperation,
    TrackableType,
)
from kodit.infrastructure.slicing.slicer import Slicer


class CommitIndexingApplicationService:
    """Application service for commit indexing operations."""

    def __init__(  # noqa: PLR0913
        self,
        commit_index_repository: CommitIndexRepository,
        snippet_v2_repository: SnippetRepositoryV2,
        repo_repository: GitRepoRepository,
        domain_indexer: IndexDomainService,
        operation: ProgressTracker,
        scanner: GitRepositoryScanner,
        cloner: RepositoryCloner,
        snippet_repository: SnippetRepositoryV2,
        slicer: Slicer,
        queue: QueueService,
        bm25_service: BM25DomainService,
    ) -> None:
        """Initialize the commit indexing application service.

        Args:
            commit_index_repository: Repository for commit index data.
            snippet_v2_repository: Repository for snippet data.
            repo_repository: Repository for Git repository data.
            domain_indexer: Domain service for indexing operations.
            operation: Progress tracker for reporting operations.

        """
        self.commit_index_repository = commit_index_repository
        self.snippet_repository = snippet_v2_repository
        self.repo_repository = repo_repository
        self.domain_indexer = domain_indexer
        self.operation = operation
        self.scanner = scanner
        self.cloner = cloner
        self.snippet_repository = snippet_repository
        self.slicer = slicer
        self.queue = queue
        self.bm25_service = bm25_service
        self._log = structlog.get_logger(__name__)

    async def create_git_repository(self, remote_uri: AnyUrl) -> GitRepo:
        """Create a new Git repository."""
        async with self.operation.create_child(TaskOperation.CREATE_REPOSITORY):
            repo = GitRepo.from_remote_uri(remote_uri)
            return await self.repo_repository.save(repo)

    async def queue_repository_tasks(
        self,
        repository_id: int | None,
        tasks: list[TaskOperation],
        base_priority: QueuePriority,
    ) -> None:
        """Queue repository tasks."""
        if repository_id is None:
            raise ValueError("Repository ID is required")
        priority_offset = len(tasks) * 10
        for task in tasks:
            await self.queue.enqueue_task(
                Task.create(
                    task,
                    base_priority + priority_offset,
                    {"repository_id": repository_id},
                )
            )
            priority_offset -= 10

    # TODO(Phil): reduce this duplication
    async def queue_commit_tasks(
        self,
        commit_sha: str,
        tasks: list[TaskOperation],
        base_priority: QueuePriority,
    ) -> None:
        """Queue repository tasks."""
        priority_offset = len(tasks) * 10
        for task in tasks:
            await self.queue.enqueue_task(
                Task.create(
                    task,
                    base_priority + priority_offset,
                    {"commit_sha": commit_sha},
                )
            )
            priority_offset -= 10

    # TODO(Phil): Make this polymorphic
    async def run_task(self, task: Task) -> None:
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
            commit_sha = task.payload["commit_sha"]
            if not commit_sha:
                raise ValueError("Commit SHA is required")
            if task.type == TaskOperation.EXTRACT_SNIPPETS_FOR_COMMIT:
                await self.process_snippets_for_commit(commit_sha)
            elif task.type == TaskOperation.CREATE_BM25_INDEX_FOR_COMMIT:
                await self.process_bm25_index(commit_sha)
            else:
                raise ValueError(f"Unknown task type: {task.type}")
        else:
            raise ValueError(f"Unknown task type: {task.type}")

    async def process_clone_repo(self, repository_id: int) -> None:
        """Clone a repository."""
        async with self.operation.create_child(TaskOperation.CLONE_REPOSITORY):
            repo = await self.repo_repository.get_by_id(repository_id)
            repo.cloned_path = await self.cloner.clone_repository(repo.remote_uri)
            await self.repo_repository.save(repo)

    async def process_scan_repo(self, repository_id: int) -> None:
        """Scan a repository."""
        async with self.operation.create_child(TaskOperation.SCAN_REPOSITORY):
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

        await self.queue_commit_tasks(
            commit_sha,
            PrescribedOperations.INDEX_COMMIT,
            QueuePriority.USER_INITIATED,
        )

    async def process_snippets_for_commit(self, commit_sha: str) -> None:
        """Generate snippets for a repository."""
        async with self.operation.create_child(
            operation=TaskOperation.EXTRACT_SNIPPETS_FOR_COMMIT,
            trackable_type=TrackableType.COMMIT,
        ) as step:
            repo = await self.repo_repository.get_by_commit(commit_sha)
            if not repo.cloned_path:
                raise ValueError(f"Repository {repo.id} has never been cloned")

            commit = await self.repo_repository.get_commit_by_sha(commit_sha)

            # Detect languages
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

            # Persist snippets to database for next pipeline stage
            await self.snippet_repository.save_snippets(commit.commit_sha, all_snippets)

    async def process_bm25_index(self, commit_sha: str) -> None:
        """Handle BM25_INDEX task - create keyword index."""
        async with self.operation.create_child(
            TaskOperation.CREATE_BM25_INDEX,
            trackable_type=TrackableType.INDEX,
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
