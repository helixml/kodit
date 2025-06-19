"""Factory for creating BM25 repositories."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext
from kodit.domain.services.bm25_service import BM25Repository
from kodit.infrastructure.bm25.local_bm25_repository import LocalBM25Repository
from kodit.infrastructure.bm25.vectorchord_bm25_repository import (
    VectorChordBM25Repository,
)


def bm25_repository_factory(
    app_context: AppContext, session: AsyncSession
) -> BM25Repository:
    """Create a BM25 repository based on configuration.

    Args:
        app_context: Application configuration context
        session: SQLAlchemy async session

    Returns:
        BM25Repository instance

    """
    if app_context.default_search.provider == "vectorchord":
        return VectorChordBM25Repository(session=session)
    return LocalBM25Repository(data_dir=app_context.get_data_dir())
