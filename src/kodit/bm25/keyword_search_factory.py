"""Factory for creating keyword search providers."""

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.bm25.keyword_search_service import KeywordSearchProvider
from kodit.bm25.local_bm25 import BM25Service
from kodit.bm25.vectorchord_bm25 import VectorChordBM25
from kodit.config import AppContext


def keyword_search_factory(
    app_context: AppContext, session: AsyncSession
) -> KeywordSearchProvider:
    """Create a keyword search provider."""
    if app_context.default_search.provider == "vectorchord":
        return VectorChordBM25(session=session)
    return BM25Service(data_dir=app_context.get_data_dir())
