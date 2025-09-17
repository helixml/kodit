"""VectorChord vector search repository implementation."""

from collections.abc import AsyncGenerator, Callable
from typing import Literal

import structlog
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.services.embedding_service import (
    EmbeddingProvider,
    VectorSearchRepository,
)
from kodit.domain.value_objects import (
    EmbeddingRequest,
    IndexRequest,
    IndexResult,
    SearchRequest,
    SearchResult,
)
from kodit.infrastructure.sqlalchemy.entities import EmbeddingType
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork

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

# Filtered search query with snippet_ids
SEARCH_QUERY_WITH_FILTER = """
SELECT snippet_id, embedding <=> :query as score
FROM {TABLE_NAME}
WHERE snippet_id = ANY(:snippet_ids)
ORDER BY score ASC
LIMIT :top_k;
"""

CHECK_VCHORD_EMBEDDING_EXISTS = """
SELECT EXISTS(SELECT 1 FROM {TABLE_NAME} WHERE snippet_id = :snippet_id)
"""

CHECK_VCHORD_EMBEDDING_EXISTS_MULTIPLE = """
SELECT snippet_id FROM {TABLE_NAME} WHERE snippet_id = ANY(:snippet_ids)
"""

TaskName = Literal["code", "text"]


class VectorChordVectorSearchRepository(VectorSearchRepository):
    """VectorChord vector search repository implementation."""

    def __init__(
        self,
        session_factory: Callable[[], AsyncSession],
        task_name: TaskName,
        embedding_provider: EmbeddingProvider,
    ) -> None:
        """Initialize the VectorChord vector search repository.

        Args:
            task_name: The task name (code or text)
            session: The SQLAlchemy async session
            embedding_provider: The embedding provider for generating embeddings

        """
        self.embedding_provider = embedding_provider
        self.session_factory = session_factory
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
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            await session.execute(text(CREATE_VCHORD_EXTENSION))

    async def _create_tables(self) -> None:
        """Create the necessary tables."""
        req = EmbeddingRequest(snippet_id="0", text="dimension")
        vector_dim: list[float] | None = None
        async for batch in self.embedding_provider.embed([req]):
            if batch:
                vector_dim = batch[0].embedding
                break
        if vector_dim is None:
            msg = "Failed to obtain embedding dimension from provider"
            raise RuntimeError(msg)
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            await session.execute(
                text(
                    f"""CREATE TABLE IF NOT EXISTS {self.table_name} (
                        id SERIAL PRIMARY KEY,
                        snippet_id VARCHAR(255) NOT NULL UNIQUE,
                        embedding VECTOR({len(vector_dim)}) NOT NULL
                    );"""
                )
            )
            await session.execute(
                text(
                    CREATE_VCHORD_INDEX.format(
                        TABLE_NAME=self.table_name, INDEX_NAME=self.index_name
                    )
                )
            )
            result = await session.execute(
                text(
                    CHECK_VCHORD_EMBEDDING_DIMENSION.format(TABLE_NAME=self.table_name)
                )
            )
            vector_dim_from_db = result.scalar_one()
            if vector_dim_from_db != len(vector_dim):
                msg = (
                    f"Embedding vector dimension does not match database, please "
                    f"delete your index: {vector_dim_from_db} != {len(vector_dim)}"
                )
                raise ValueError(msg)

    async def index_documents(
        self, request: IndexRequest
    ) -> AsyncGenerator[list[IndexResult], None]:
        """Index documents for vector search."""
        if not self._initialized:
            await self._initialize()

        if not request.documents:
            yield []

        # Search for existing embeddings
        existing_ids = await self._get_existing_ids(
            [doc.snippet_id for doc in request.documents]
        )
        new_documents = [
            doc for doc in request.documents if doc.snippet_id not in existing_ids
        ]
        if not new_documents:
            self.log.info("No new documents to index")
            return

        # Convert to embedding requests
        embedding_requests = [
            EmbeddingRequest(snippet_id=doc.snippet_id, text=doc.text)
            for doc in new_documents
        ]

        async for batch in self.embedding_provider.embed(embedding_requests):
            async with SqlAlchemyUnitOfWork(self.session_factory) as session:
                await session.execute(
                    text(INSERT_QUERY.format(TABLE_NAME=self.table_name)),
                    [
                        {
                            "snippet_id": result.snippet_id,
                            "embedding": str(result.embedding),
                        }
                        for result in batch
                    ],
                )
                yield [IndexResult(snippet_id=result.snippet_id) for result in batch]

    async def search(self, request: SearchRequest) -> list[SearchResult]:
        """Search documents using vector similarity."""
        if not self._initialized:
            await self._initialize()
        if not request.query or not request.query.strip():
            return []

        req = EmbeddingRequest(snippet_id="0", text=request.query)
        embedding_vec: list[float] | None = None
        async for batch in self.embedding_provider.embed([req]):
            if batch:
                embedding_vec = batch[0].embedding
                break

        if not embedding_vec:
            return []

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Use filtered query if snippet_ids are provided
            if request.snippet_ids is not None:
                result = await session.execute(
                    text(SEARCH_QUERY_WITH_FILTER.format(TABLE_NAME=self.table_name)),
                    {
                        "query": str(embedding_vec),
                        "top_k": request.top_k,
                        "snippet_ids": request.snippet_ids,
                    },
                )
            else:
                result = await session.execute(
                    text(SEARCH_QUERY.format(TABLE_NAME=self.table_name)),
                    {"query": str(embedding_vec), "top_k": request.top_k},
                )

            rows = result.mappings().all()

            return [
                SearchResult(snippet_id=row["snippet_id"], score=row["score"])
                for row in rows
            ]

    async def has_embedding(
        self, snippet_id: int, embedding_type: EmbeddingType
    ) -> bool:
        """Check if a snippet has an embedding."""
        if not self._initialized:
            await self._initialize()
        # For VectorChord, we check if the snippet exists in the table
        # Note: embedding_type is ignored since VectorChord uses separate
        # tables per task
        # ruff: noqa: ARG002
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            result = await session.execute(
                text(CHECK_VCHORD_EMBEDDING_EXISTS.format(TABLE_NAME=self.table_name)),
                {"snippet_id": snippet_id},
            )
            return bool(result.scalar())

    async def _get_existing_ids(self, snippet_ids: list[str]) -> set[str]:
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            result = await session.execute(
                text(
                    CHECK_VCHORD_EMBEDDING_EXISTS_MULTIPLE.format(
                        TABLE_NAME=self.table_name
                    )
                ),
                {"snippet_ids": snippet_ids},
            )
            return {row[0] for row in result.fetchall()}
