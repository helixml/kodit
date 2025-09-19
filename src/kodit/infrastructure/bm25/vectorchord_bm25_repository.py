"""VectorChord BM25 repository implementation."""

from collections.abc import Callable

import structlog
from sqlalchemy import bindparam, text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.services.bm25_service import BM25Repository
from kodit.domain.value_objects import (
    DeleteRequest,
    IndexRequest,
    SearchRequest,
    SearchResult,
)
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork

TABLE_NAME = "vectorchord_bm25_documents"
INDEX_NAME = f"{TABLE_NAME}_idx"
TOKENIZER_NAME = "bert"

# SQL statements
CREATE_VCHORD_EXTENSION = "CREATE EXTENSION IF NOT EXISTS vchord CASCADE;"
CREATE_PG_TOKENIZER = "CREATE EXTENSION IF NOT EXISTS pg_tokenizer CASCADE;"
CREATE_VCHORD_BM25 = "CREATE EXTENSION IF NOT EXISTS vchord_bm25 CASCADE;"
SET_SEARCH_PATH = """
SET search_path TO
    "$user", public, bm25_catalog, pg_catalog, information_schema, tokenizer_catalog;
"""
CREATE_BM25_TABLE = f"""
CREATE TABLE IF NOT EXISTS {TABLE_NAME} (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL,
    passage TEXT NOT NULL,
    embedding bm25vector,
    UNIQUE(snippet_id)
)
"""
CHECK_EXISTING_IDS = f"""
    SELECT snippet_id
    FROM {TABLE_NAME}
    WHERE snippet_id = ANY(:snippet_ids)
"""  # noqa: S608
CREATE_BM25_INDEX = f"""
CREATE INDEX IF NOT EXISTS {INDEX_NAME}
ON {TABLE_NAME}
USING bm25 (embedding bm25_ops)
"""
TOKENIZER_NAME_CHECK_QUERY = (
    f"SELECT 1 FROM tokenizer_catalog.tokenizer WHERE name = '{TOKENIZER_NAME}'"  # noqa: S608
)
LOAD_TOKENIZER = """
SELECT create_tokenizer('bert', $$
model = "llmlingua2"
pre_tokenizer = "unicode_segmentation"  # Unicode Standard Annex #29
[[character_filters]]
to_lowercase = {}                       # convert all characters to lowercase
[[character_filters]]
unicode_normalization = "nfkd"          # Unicode Normalization Form KD
[[token_filters]]
skip_non_alphanumeric = {}              # remove non-alphanumeric tokens
[[token_filters]]
stopwords = "nltk_english"              # remove stopwords using the nltk dictionary
[[token_filters]]
stemmer = "english_porter2"             # stem tokens using the English Porter2 stemmer
$$)
"""
INSERT_QUERY = f"""
INSERT INTO {TABLE_NAME} (snippet_id, passage)
VALUES (:snippet_id, :passage)
ON CONFLICT (snippet_id) DO UPDATE
SET passage = EXCLUDED.passage
"""  # noqa: S608
UPDATE_QUERY = f"""
UPDATE {TABLE_NAME}
SET embedding = tokenize(passage, '{TOKENIZER_NAME}')
"""  # noqa: S608
# https://github.com/tensorchord/VectorChord-bm25:
# We intentionally make it negative so that you can use the
# default order by to get the most relevant documents first.
SEARCH_QUERY = f"""
    SELECT
        snippet_id,
        embedding <&>
            to_bm25query('{INDEX_NAME}', tokenize(:query_text, '{TOKENIZER_NAME}'))
    AS bm25_score
    FROM {TABLE_NAME}
    ORDER BY bm25_score
    LIMIT :limit
"""  # noqa: S608
SEARCH_QUERY_WITH_FILTER = f"""
    SELECT
        snippet_id,
        embedding <&>
            to_bm25query('{INDEX_NAME}', tokenize(:query_text, '{TOKENIZER_NAME}'))
    AS bm25_score
    FROM {TABLE_NAME}
    WHERE snippet_id = ANY(:snippet_ids)
    ORDER BY bm25_score
    LIMIT :limit
"""  # noqa: S608
DELETE_QUERY = f"""
DELETE FROM {TABLE_NAME}
WHERE snippet_id IN :snippet_ids
"""  # noqa: S608


class VectorChordBM25Repository(BM25Repository):
    """VectorChord BM25 repository implementation."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the VectorChord BM25 repository.

        Args:
            session: The SQLAlchemy async session to use for database operations

        """
        self.session_factory = session_factory
        self._initialized = False
        self.log = structlog.get_logger(__name__)

    async def _initialize(self) -> None:
        """Initialize the VectorChord environment."""
        try:
            await self._create_extensions()
            await self._create_tokenizer_if_not_exists()
            await self._create_tables()
            self._initialized = True
        except Exception as e:
            msg = f"Failed to initialize VectorChord repository: {e}"
            raise RuntimeError(msg) from e

    async def _create_extensions(self) -> None:
        """Create the necessary extensions."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            await session.execute(text(CREATE_VCHORD_EXTENSION))
            await session.execute(text(CREATE_PG_TOKENIZER))
            await session.execute(text(CREATE_VCHORD_BM25))
            await session.execute(text(SET_SEARCH_PATH))

    async def _create_tokenizer_if_not_exists(self) -> None:
        """Create the tokenizer if it doesn't exist."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Check if tokenizer exists in the catalog
            result = await session.execute(text(TOKENIZER_NAME_CHECK_QUERY))
            if result.scalar_one_or_none() is None:
                # Tokenizer doesn't exist, create it
                await session.execute(text(LOAD_TOKENIZER))

    async def _create_tables(self) -> None:
        """Create the necessary tables in the correct order."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            await session.execute(text(CREATE_BM25_TABLE))
            await session.execute(text(CREATE_BM25_INDEX))

    async def _get_existing_ids(self, snippet_ids: list[str]) -> set[int]:
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            result = await session.execute(
                text(CHECK_EXISTING_IDS), {"snippet_ids": snippet_ids}
            )
            return {row[0] for row in result.fetchall()}

    async def index_documents(self, request: IndexRequest) -> None:
        """Index documents for BM25 search."""
        if not self._initialized:
            await self._initialize()

        # Filter out any documents that don't have a snippet_id or text
        valid_documents = [
            doc
            for doc in request.documents
            if doc.snippet_id is not None and doc.text is not None and doc.text != ""
        ]

        if not valid_documents:
            self.log.warning("Corpus is empty, skipping bm25 index")
            return

        # Filter out documents that have already been indexed
        existing_ids = await self._get_existing_ids(
            [doc.snippet_id for doc in valid_documents]
        )
        valid_documents = [
            doc for doc in valid_documents if doc.snippet_id not in existing_ids
        ]

        if not valid_documents:
            self.log.info("No new documents to index")
            return

        # Execute inserts
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            await session.execute(
                text(INSERT_QUERY),
                [
                    {"snippet_id": doc.snippet_id, "passage": doc.text}
                    for doc in valid_documents
                ],
            )

            # Tokenize the new documents with schema qualification
            await session.execute(text(UPDATE_QUERY))

    async def search(self, request: SearchRequest) -> list[SearchResult]:
        """Search documents using BM25."""
        if not self._initialized:
            await self._initialize()
        if not request.query or request.query.strip() == "":
            return []

        if request.snippet_ids is not None:
            sql = text(SEARCH_QUERY_WITH_FILTER).bindparams(
                query_text=request.query,
                limit=request.top_k,
                snippet_ids=request.snippet_ids,
            )
        else:
            sql = text(SEARCH_QUERY).bindparams(
                query_text=request.query,
                limit=request.top_k,
            )

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            result = await session.execute(sql)
            rows = result.mappings().all()

            return [
                SearchResult(snippet_id=row["snippet_id"], score=row["bm25_score"])
                for row in rows
            ]

    async def delete_documents(self, request: DeleteRequest) -> None:
        """Delete documents from the index."""
        if not self._initialized:
            await self._initialize()
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            await session.execute(
                text(DELETE_QUERY).bindparams(bindparam("snippet_ids", expanding=True)),
                {"snippet_ids": request.snippet_ids},
            )
