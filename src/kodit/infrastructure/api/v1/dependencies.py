"""FastAPI dependencies for the REST API."""

from collections.abc import Callable
from typing import Annotated, cast

from fastapi import Depends, Request
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.server_factory import ServerFactory
from kodit.application.services.code_search_application_service import (
    CodeSearchApplicationService,
)
from kodit.application.services.commit_indexing_application_service import (
    CommitIndexingApplicationService,
)
from kodit.application.services.enrichment_query_service import (
    EnrichmentQueryService,
)
from kodit.application.services.queue_service import QueueService
from kodit.application.services.repository_query_service import (
    RepositoryQueryService,
)
from kodit.config import AppContext
from kodit.domain.protocols import (
    GitBranchRepository,
    GitCommitRepository,
    GitFileRepository,
    GitRepoRepository,
    GitTagRepository,
)
from kodit.domain.services.task_status_query_service import TaskStatusQueryService
from kodit.infrastructure.sqlalchemy.task_status_repository import (
    create_task_status_repository,
)


def get_app_context(request: Request) -> AppContext:
    """Get the app context dependency."""
    app_context = cast("AppContext", request.state.app_context)
    if app_context is None:
        raise RuntimeError("App context not initialized")
    return app_context


AppContextDep = Annotated[AppContext, Depends(get_app_context)]


def get_server_factory(request: Request) -> ServerFactory:
    """Get the server factory dependency."""
    server_factory = cast("ServerFactory", request.state.server_factory)
    if server_factory is None:
        raise RuntimeError("Server factory not initialized")
    return server_factory


ServerFactoryDep = Annotated[ServerFactory, Depends(get_server_factory)]


async def get_db_session_factory(
    server_factory: ServerFactoryDep,
) -> Callable[[], AsyncSession]:
    """Get database session dependency."""
    return server_factory.session_factory


DBSessionFactoryDep = Annotated[
    Callable[[], AsyncSession], Depends(get_db_session_factory)
]


async def get_queue_service(
    session_factory: DBSessionFactoryDep,
) -> QueueService:
    """Get queue service dependency."""
    return QueueService(
        session_factory=session_factory,
    )


QueueServiceDep = Annotated[QueueService, Depends(get_queue_service)]


async def get_task_status_query_service(
    session_factory: DBSessionFactoryDep,
) -> TaskStatusQueryService:
    """Get task status query service dependency."""
    return TaskStatusQueryService(
        repository=create_task_status_repository(session_factory=session_factory)
    )


TaskStatusQueryServiceDep = Annotated[
    TaskStatusQueryService, Depends(get_task_status_query_service)
]


async def get_git_repository(
    server_factory: ServerFactoryDep,
) -> GitRepoRepository:
    """Get git repository dependency."""
    return server_factory.repo_repository()


GitRepositoryDep = Annotated[GitRepoRepository, Depends(get_git_repository)]


async def get_git_commit_repository(
    server_factory: ServerFactoryDep,
) -> GitCommitRepository:
    """Get git commit repository dependency."""
    return server_factory.git_commit_repository()


GitCommitRepositoryDep = Annotated[
    GitCommitRepository, Depends(get_git_commit_repository)
]


async def get_git_branch_repository(
    server_factory: ServerFactoryDep,
) -> GitBranchRepository:
    """Get git branch repository dependency."""
    return server_factory.git_branch_repository()


GitBranchRepositoryDep = Annotated[
    GitBranchRepository, Depends(get_git_branch_repository)
]


async def get_git_tag_repository(
    server_factory: ServerFactoryDep,
) -> GitTagRepository:
    """Get git tag repository dependency."""
    return server_factory.git_tag_repository()


GitTagRepositoryDep = Annotated[GitTagRepository, Depends(get_git_tag_repository)]


async def get_commit_indexing_app_service(
    server_factory: ServerFactoryDep,
) -> CommitIndexingApplicationService:
    """Get commit indexing application service dependency."""
    return server_factory.commit_indexing_application_service()


CommitIndexingAppServiceDep = Annotated[
    CommitIndexingApplicationService, Depends(get_commit_indexing_app_service)
]


async def get_code_search_app_service(
    server_factory: ServerFactoryDep,
) -> CodeSearchApplicationService:
    """Get code search application service dependency."""
    return server_factory.code_search_application_service()


CodeSearchAppServiceDep = Annotated[
    CodeSearchApplicationService, Depends(get_code_search_app_service)
]


async def get_enrichment_query_service(
    server_factory: ServerFactoryDep,
) -> EnrichmentQueryService:
    """Get enrichment query service dependency."""
    return server_factory.enrichment_query_service()


EnrichmentQueryServiceDep = Annotated[
    EnrichmentQueryService, Depends(get_enrichment_query_service)
]


async def get_repository_query_service(
    server_factory: ServerFactoryDep,
) -> RepositoryQueryService:
    """Get repository query service dependency."""
    return server_factory.repository_query_service()


RepositoryQueryServiceDep = Annotated[
    RepositoryQueryService, Depends(get_repository_query_service)
]


async def get_git_file_repository(
    server_factory: ServerFactoryDep,
) -> GitFileRepository:
    """Get git file repository dependency."""
    return server_factory.git_file_repository()


GitFileRepositoryDep = Annotated[GitFileRepository, Depends(get_git_file_repository)]
