import pytest

from kodit.config import AppContext
from kodit.embedding.embedding_factory import embedding_factory


@pytest.mark.asyncio
async def test_embedding_factory_openai(app_context: AppContext):
    assert False
