"""Factory for creating BM25 repositories."""

from kodit.config import AppContext
from kodit.domain.protocols import UnitOfWork
from kodit.domain.services.bm25_service import BM25Repository
from kodit.infrastructure.bm25.local_bm25_repository import LocalBM25Repository
from kodit.infrastructure.bm25.vectorchord_bm25_repository import (
    VectorChordBM25Repository,
)


def bm25_repository_factory(
    app_context: AppContext, unit_of_work: UnitOfWork
) -> BM25Repository:
    """Create a BM25 repository based on configuration.

    Args:
        app_context: Application configuration context
        session: SQLAlchemy async session

    Returns:
        BM25Repository instance

    """
    if app_context.default_search.provider == "vectorchord":
        return VectorChordBM25Repository(unit_of_work)
    return LocalBM25Repository(data_dir=app_context.get_data_dir())
