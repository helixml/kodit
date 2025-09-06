"""FastAPI dependencies for the REST API."""

from collections.abc import AsyncGenerator, Callable
from typing import Annotated, cast

from fastapi import Depends, Request
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.code_indexing_factory import (
    create_server_code_indexing_application_service,
)
from kodit.application.factories.code_search_factory import (
    create_server_code_search_application_service,
)
from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService,
)
from kodit.application.services.code_search_application_service import (
    CodeSearchApplicationService,
)
from kodit.application.services.queue_service import QueueService
from kodit.config import AppContext
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.services.task_status_query_service import TaskStatusQueryService
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.sqlalchemy.index_repository import create_index_repository
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


async def get_db_session(
    app_context: AppContextDep,
) -> AsyncGenerator[AsyncSession, None]:
    """Get database session dependency."""
    db = await app_context.get_db()
    async with db.session_factory() as session:
        yield session


DBSessionDep = Annotated[AsyncSession, Depends(get_db_session)]


async def get_db_session_factory(
    app_context: AppContextDep,
) -> AsyncGenerator[Callable[[], AsyncSession], None]:
    """Get database session dependency."""
    db = await app_context.get_db()
    yield db.session_factory


DBSessionFactoryDep = Annotated[
    Callable[[], AsyncSession], Depends(get_db_session_factory)
]


async def get_index_query_service(
    session_factory: DBSessionFactoryDep,
) -> IndexQueryService:
    """Get index query service dependency."""
    return IndexQueryService(
        index_repository=create_index_repository(session_factory=session_factory),
        fusion_service=ReciprocalRankFusionService(),
    )


IndexQueryServiceDep = Annotated[IndexQueryService, Depends(get_index_query_service)]


async def get_indexing_app_service(
    app_context: AppContextDep,
    session_factory: DBSessionFactoryDep,
) -> CodeIndexingApplicationService:
    """Get indexing application service dependency."""
    return create_server_code_indexing_application_service(app_context, session_factory)


IndexingAppServiceDep = Annotated[
    CodeIndexingApplicationService, Depends(get_indexing_app_service)
]


async def get_search_app_service(
    app_context: AppContextDep,
    session_factory: DBSessionFactoryDep,
) -> CodeSearchApplicationService:
    """Get search application service dependency."""
    return create_server_code_search_application_service(app_context, session_factory)


SearchAppServiceDep = Annotated[
    CodeSearchApplicationService, Depends(get_search_app_service)
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
