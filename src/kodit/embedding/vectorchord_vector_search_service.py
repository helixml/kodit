"""Vectorchord vector search."""

from collections.abc import AsyncGenerator
from typing import Any, Literal

import structlog
from sqlalchemy import Result, TextClause, text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.embedding.embedding_models import EmbeddingType
from kodit.embedding.embedding_provider.embedding_provider import (
    EmbeddingProvider,
    EmbeddingRequest,
)
from kodit.embedding.vector_search_service import (
    IndexResult,
    VectorSearchRequest,
    VectorSearchResponse,
    VectorSearchService,
)

# SQL Queries
CREATE_VCHORD_EXTENSION = """
CREATE EXTENSION IF NOT EXISTS vchord CASCADE;
"""

CHECK_VCHORD_EMBEDDING_DIMENSION = """
SELECT a.atttypmod as dimension
FROM pg_attribute a
JOIN pg_class c ON a.attrelid = c.oid
WHERE c.relname = '{TABLE_NAME}'
AND a.attname = 'embedding';
"""

CREATE_VCHORD_INDEX = """
CREATE INDEX IF NOT EXISTS {INDEX_NAME}
ON {TABLE_NAME}
USING vchordrq (embedding vector_l2_ops) WITH (options = $$
residual_quantization = true
[build.internal]
lists = []
$$);
"""

INSERT_QUERY = """
INSERT INTO {TABLE_NAME} (snippet_id, embedding)
VALUES (:snippet_id, :embedding)
ON CONFLICT (snippet_id) DO UPDATE
SET embedding = EXCLUDED.embedding
"""

# Note that <=> in vectorchord is cosine distance
# So scores go from 0 (similar) to 2 (opposite)
SEARCH_QUERY = """
SELECT snippet_id, embedding <=> :query as score
FROM {TABLE_NAME}
ORDER BY score ASC
LIMIT :top_k;
"""

CHECK_VCHORD_EMBEDDING_EXISTS = """
SELECT EXISTS(SELECT 1 FROM {TABLE_NAME} WHERE snippet_id = :snippet_id)
"""

TaskName = Literal["code", "text"]


class VectorChordVectorSearchService(VectorSearchService):
    """VectorChord vector search."""

    def __init__(
        self,
        task_name: TaskName,
        session: AsyncSession,
        embedding_provider: EmbeddingProvider,
    ) -> None:
        """Initialize the VectorChord BM25."""
        self.embedding_provider = embedding_provider
        self._session = session
        self._initialized = False
        self.table_name = f"vectorchord_{task_name}_embeddings"
        self.index_name = f"{self.table_name}_idx"
        self.log = structlog.get_logger(__name__)

    async def _initialize(self) -> None:
        """Initialize the VectorChord environment."""
        try:
            await self._create_extensions()
            await self._create_tables()
            self._initialized = True
        except Exception as e:
            msg = f"Failed to initialize VectorChord repository: {e}"
            raise RuntimeError(msg) from e

    async def _create_extensions(self) -> None:
        """Create the necessary extensions."""
        await self._session.execute(text(CREATE_VCHORD_EXTENSION))
        await self._commit()

    async def _create_tables(self) -> None:
        """Create the necessary tables."""
        req = EmbeddingRequest(id=0, text="dimension")
        vector_dim: list[float] | None = None
        async for batch in self.embedding_provider.embed([req]):
            if batch:
                vector_dim = batch[0].embedding
                break
        if vector_dim is None:
            msg = "Failed to obtain embedding dimension from provider"
            raise RuntimeError(msg)
        await self._session.execute(
            text(
                f"""CREATE TABLE IF NOT EXISTS {self.table_name} (
                    id SERIAL PRIMARY KEY,
                    snippet_id INT NOT NULL UNIQUE,
                    embedding VECTOR({len(vector_dim)}) NOT NULL
                );"""
            )
        )
        await self._session.execute(
            text(
                CREATE_VCHORD_INDEX.format(
                    TABLE_NAME=self.table_name, INDEX_NAME=self.index_name
                )
            )
        )
        result = await self._session.execute(
            text(CHECK_VCHORD_EMBEDDING_DIMENSION.format(TABLE_NAME=self.table_name))
        )
        vector_dim_from_db = result.scalar_one()
        if vector_dim_from_db != len(vector_dim):
            msg = (
                f"Embedding vector dimension does not match database, "
                f"please delete your index: {vector_dim_from_db} != {len(vector_dim)}"
            )
            raise ValueError(msg)
        await self._commit()

    async def _execute(
        self, query: TextClause, param_list: list[Any] | dict[str, Any] | None = None
    ) -> Result:
        """Execute a query."""
        if not self._initialized:
            await self._initialize()
        return await self._session.execute(query, param_list)

    async def _commit(self) -> None:
        """Commit the session."""
        await self._session.commit()

    async def index(
        self, data: list[VectorSearchRequest]
    ) -> AsyncGenerator[list[IndexResult], None]:
        """Embed a list of documents."""
        if not data or len(data) == 0:
            self.log.warning("Embedding data is empty, skipping embedding")
            return

        requests = [EmbeddingRequest(id=doc.snippet_id, text=doc.text) for doc in data]

        async for batch in self.embedding_provider.embed(requests):
            await self._execute(
                text(INSERT_QUERY.format(TABLE_NAME=self.table_name)),
                [
                    {
                        "snippet_id": result.id,
                        "embedding": str(result.embedding),
                    }
                    for result in batch
                ],
            )
            await self._commit()
            yield [IndexResult(snippet_id=result.id) for result in batch]

    async def retrieve(self, query: str, top_k: int = 10) -> list[VectorSearchResponse]:
        """Query the embedding model."""
        from kodit.embedding.embedding_provider.embedding_provider import (
            EmbeddingRequest,
        )

        req = EmbeddingRequest(id=0, text=query)
        embedding_vec: list[float] | None = None
        async for batch in self.embedding_provider.embed([req]):
            if batch:
                embedding_vec = batch[0].embedding
                break

        if not embedding_vec:
            return []
        result = await self._execute(
            text(SEARCH_QUERY.format(TABLE_NAME=self.table_name)),
            {"query": str(embedding_vec), "top_k": top_k},
        )
        rows = result.mappings().all()

        return [
            VectorSearchResponse(snippet_id=row["snippet_id"], score=row["score"])
            for row in rows
        ]

    async def has_embedding(
        self,
        snippet_id: int,
        embedding_type: EmbeddingType,  # noqa: ARG002
    ) -> bool:
        """Check if a snippet has an embedding."""
        result = await self._execute(
            text(CHECK_VCHORD_EMBEDDING_EXISTS.format(TABLE_NAME=self.table_name)),
            {"snippet_id": snippet_id},
        )
        return result.scalar_one()
