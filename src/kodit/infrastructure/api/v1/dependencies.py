"""FastAPI dependencies for the REST API."""

from collections.abc import AsyncGenerator
from typing import Annotated, cast

from fastapi import Depends, Request

from kodit.application.factories.code_indexing_factory import (
    create_code_indexing_application_service,
)
from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService,
)
from kodit.application.services.queue_service import QueueService
from kodit.config import AppContext
from kodit.domain.protocols import ReportingService, UnitOfWork
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.reporting.progress import ProgressConfig
from kodit.infrastructure.reporting.reporter import create_server_reporter
from kodit.infrastructure.sqlalchemy.index_repository import SqlAlchemyIndexRepository
from kodit.infrastructure.sqlalchemy.operation_repository import (
    SqlAlchemyOperationRepository,
)
from kodit.infrastructure.sqlalchemy.task_repository import SqlAlchemyTaskRepository
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def get_app_context(request: Request) -> AppContext:
    """Get the app context dependency."""
    app_context = cast("AppContext", request.state.app_context)
    if app_context is None:
        raise RuntimeError("App context not initialized")
    return app_context


AppContextDep = Annotated[AppContext, Depends(get_app_context)]


async def get_unit_of_work(
    app_context: AppContextDep,
) -> AsyncGenerator[UnitOfWork, None]:
    """Get database session dependency."""
    db = await app_context.get_db()
    yield SqlAlchemyUnitOfWork(db.session_factory)


UOWDep = Annotated[UnitOfWork, Depends(get_unit_of_work)]


async def get_index_query_service(
    uow: UOWDep,
) -> IndexQueryService:
    """Get index query service dependency."""
    return IndexQueryService(
        index_repository=SqlAlchemyIndexRepository(uow=uow),
        fusion_service=ReciprocalRankFusionService(),
    )


IndexQueryServiceDep = Annotated[IndexQueryService, Depends(get_index_query_service)]


async def get_indexing_app_service(
    app_context: AppContextDep,
    uow: UOWDep,
) -> CodeIndexingApplicationService:
    """Get indexing application service dependency."""
    return create_code_indexing_application_service(
        app_context=app_context,
        unit_of_work=uow,
        reporter=_create_reporter(uow),
    )


def _create_reporter(unit_of_work: UnitOfWork) -> ReportingService:
    reporter_config = ProgressConfig()
    operation_repository = SqlAlchemyOperationRepository(unit_of_work)
    return create_server_reporter(
        operation_repository=operation_repository, config=reporter_config
    )


IndexingAppServiceDep = Annotated[
    CodeIndexingApplicationService, Depends(get_indexing_app_service)
]


async def get_queue_service(
    unit_of_work: UnitOfWork,
) -> QueueService:
    """Get queue service dependency."""
    task_repository = SqlAlchemyTaskRepository(unit_of_work)
    return QueueService(
        task_repository=task_repository,
    )


QueueServiceDep = Annotated[QueueService, Depends(get_queue_service)]
