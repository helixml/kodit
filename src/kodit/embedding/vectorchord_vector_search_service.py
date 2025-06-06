"""Vectorchord vector search."""

from typing import Any

from sqlalchemy import Result, TextClause, text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.embedding.embedding_provider.embedding_provider import EmbeddingProvider
from kodit.embedding.vector_search_service import (
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


class VectorChordVectorSearchService(VectorSearchService):
    """VectorChord vector search."""

    def __init__(
        self,
        task_name: str,
        session: AsyncSession,
        embedding_provider: EmbeddingProvider,
    ) -> None:
        """Initialize the VectorChord BM25."""
        self.embedding_provider = embedding_provider
        self._session = session
        self._initialized = False
        self.table_name = f"vectorchord_{task_name}_embeddings"
        self.index_name = f"{self.table_name}_idx"

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
        vector_dim = (await self.embedding_provider.embed(["dimension"]))[0]
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

    async def index(self, data: list[VectorSearchRequest]) -> None:
        """Embed a list of documents."""
        embeddings = await self.embedding_provider.embed([doc.text for doc in data])
        # Execute inserts
        await self._execute(
            text(INSERT_QUERY.format(TABLE_NAME=self.table_name)),
            [
                {"snippet_id": doc.snippet_id, "embedding": str(embedding)}
                for doc, embedding in zip(data, embeddings, strict=True)
            ],
        )
        await self._commit()

    async def retrieve(self, query: str, top_k: int = 10) -> list[VectorSearchResponse]:
        """Query the embedding model."""
        embedding = await self.embedding_provider.embed([query])
        if len(embedding) == 0 or len(embedding[0]) == 0:
            return []
        result = await self._execute(
            text(SEARCH_QUERY.format(TABLE_NAME=self.table_name)),
            {"query": str(embedding[0]), "top_k": top_k},
        )
        rows = result.mappings().all()

        return [
            VectorSearchResponse(snippet_id=row["snippet_id"], score=row["score"])
            for row in rows
        ]
