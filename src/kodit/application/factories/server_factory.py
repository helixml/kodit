"""Create a big object that contains all the application services."""

from collections.abc import Callable

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.reporting_factory import create_server_operation
from kodit.application.services.commit_indexing_application_service import (
    CommitIndexingApplicationService,
    CommitIndexQueryService,
)
from kodit.application.services.git_application_service import GitApplicationService
from kodit.application.services.reporting import ProgressTracker
from kodit.config import AppContext
from kodit.domain.protocols import (
    CommitIndexRepository,
    GitAdapter,
    GitBranchRepository,
    GitCommitRepository,
    GitRepoRepository,
    SnippetRepository,
    SnippetRepositoryV2,
    TaskStatusRepository,
)
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.git_repository_scanner import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.domain.services.index_service import IndexDomainService
from kodit.domain.value_objects import LanguageMapping
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.enrichment.enrichment_factory import (
    enrichment_domain_service_factory,
)
from kodit.infrastructure.memory.in_memory_commit_index_repository import (
    InMemoryCommitIndexRepository,
)
from kodit.infrastructure.memory.in_memory_git_repository import (
    InMemoryGitBranchRepository,
    InMemoryGitCommitRepository,
    InMemoryGitRepoRepository,
)
from kodit.infrastructure.memory.in_memory_snippet_v2_repository import (
    InMemorySnippetRepository,
)
from kodit.infrastructure.slicing.language_detection_service import (
    FileSystemLanguageDetectionService,
)
from kodit.infrastructure.sqlalchemy.snippet_repository import create_snippet_repository
from kodit.infrastructure.sqlalchemy.task_status_repository import (
    create_task_status_repository,
)


class ServerFactory:
    """Factory for creating server application services."""

    def __init__(
        self,
        app_context: AppContext,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Initialize the ServerFactory."""
        self.app_context = app_context
        self.session_factory = session_factory
        self._repo_repository = None
        self._branch_repository = None
        self._commit_repository = None
        self._commit_index_repository = None
        self._snippet_v2_repository = None
        self._domain_indexer = None
        self._git_adapter = None
        self._scanner = None
        self._cloner = None
        self._git_application_service = None
        self._commit_indexing_application_service = None
        self._snippet_repository = None
        self._enrichment_service = None
        self._task_status_repository = None
        self._operation = None

    def task_status_repository(self) -> TaskStatusRepository:
        """Create a TaskStatusRepository instance."""
        if not self._task_status_repository:
            self._task_status_repository = create_task_status_repository(
                session_factory=self.session_factory
            )
        return self._task_status_repository

    def operation(self) -> ProgressTracker:
        """Create a ProgressTracker instance."""
        if not self._operation:
            self._operation = create_server_operation(
                task_status_repository=self.task_status_repository()
            )
        return self._operation

    def git_application_service(self) -> GitApplicationService:
        """Create a GitApplicationService instance."""
        if not self._git_application_service:
            return GitApplicationService(
                repo_repository=self.repo_repository(),
                commit_repository=self.commit_repository(),
                branch_repository=self.branch_repository(),
                scanner=self.scanner(),
                cloner=self.cloner(),
                git_adapter=self.git_adapter(),
            )
        return self._git_application_service

    def commit_indexing_application_service(self) -> CommitIndexingApplicationService:
        """Create a CommitIndexingApplicationService instance."""
        if not self._commit_indexing_application_service:
            self._commit_indexing_application_service = (
                CommitIndexingApplicationService(
                    commit_index_repository=self.commit_index_repository(),
                    snippet_v2_repository=self.snippet_v2_repository(),
                    repo_repository=self.repo_repository(),
                    domain_indexer=self.domain_indexer(),
                    commit_repository=self.commit_repository(),
                    operation=self.operation(),
                )
            )

        return self._commit_indexing_application_service

    def commit_index_query_service(self) -> CommitIndexQueryService:
        """Create a CommitIndexQueryService instance."""
        if not self._commit_index_query_service:
            self._commit_index_query_service = CommitIndexQueryService(
                commit_index_repository=self.commit_index_repository(),
                snippet_repository=self.snippet_repository(),
            )
        return self._commit_index_query_service

    def repo_repository(self) -> GitRepoRepository:
        """Create a GitRepoRepository instance."""
        if not self._repo_repository:
            self._repo_repository = InMemoryGitRepoRepository()
        return self._repo_repository

    def branch_repository(self) -> GitBranchRepository:
        """Create a GitBranchRepository instance."""
        if not self._branch_repository:
            self._branch_repository = InMemoryGitBranchRepository()
        return self._branch_repository

    def commit_repository(self) -> GitCommitRepository:
        """Create a GitCommitRepository instance."""
        if not self._commit_repository:
            self._commit_repository = InMemoryGitCommitRepository(
                self.branch_repository()
            )
        return self._commit_repository

    def git_adapter(self) -> GitAdapter:
        """Create a GitAdapter instance."""
        if not self._git_adapter:
            self._git_adapter = GitPythonAdapter()
        return self._git_adapter

    def scanner(self) -> GitRepositoryScanner:
        """Create a GitRepositoryScanner instance."""
        if not self._scanner:
            self._scanner = GitRepositoryScanner(self.git_adapter())
        return self._scanner

    def cloner(self) -> RepositoryCloner:
        """Create a RepositoryCloner instance."""
        if not self._cloner:
            self._cloner = RepositoryCloner(
                self.git_adapter(), self.app_context.get_clone_dir()
            )
        return self._cloner

    def commit_index_repository(self) -> CommitIndexRepository:
        """Create a CommitIndexRepository instance."""
        if not self._commit_index_repository:
            self._commit_index_repository = InMemoryCommitIndexRepository()
        return self._commit_index_repository

    def snippet_v2_repository(self) -> SnippetRepositoryV2:
        """Create a SnippetRepositoryV2 instance."""
        if not self._snippet_v2_repository:
            self._snippet_v2_repository = InMemorySnippetRepository()
        return self._snippet_v2_repository

    def domain_indexer(self) -> IndexDomainService:
        """Create a IndexDomainService instance."""
        if not self._domain_indexer:
            # Use the unified language mapping from the domain layer
            language_map = LanguageMapping.get_extension_to_language_map()
            language_detector = FileSystemLanguageDetectionService(language_map)
            self._domain_indexer = IndexDomainService(
                language_detector=language_detector,
                enrichment_service=self.enrichment_service(),
                snippet_repository=self.snippet_repository(),
                clone_dir=self.app_context.get_clone_dir(),
            )
        return self._domain_indexer

    def snippet_repository(self) -> SnippetRepository:
        """Create a SnippetRepository instance."""
        if not self._snippet_repository:
            self._snippet_repository = create_snippet_repository(
                session_factory=self.session_factory
            )
        return self._snippet_repository

    def enrichment_service(self) -> EnrichmentDomainService:
        """Create a EnrichmentDomainService instance."""
        if not self._enrichment_service:
            self._enrichment_service = enrichment_domain_service_factory(
                self.app_context
            )
        return self._enrichment_service
