"""VectorChord repository for document operations."""

from typing import Any

from sqlalchemy import Result, TextClause, bindparam, text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.bm25.keyword_search_service import (
    BM25Document,
    BM25Result,
    KeywordSearchProvider,
)

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
    snippet_id BIGINT NOT NULL,
    passage TEXT NOT NULL,
    embedding bm25vector,
    UNIQUE(snippet_id)
)
"""

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
DELETE_QUERY = f"""
DELETE FROM {TABLE_NAME}
WHERE snippet_id IN :snippet_ids
"""  # noqa: S608


class VectorChordBM25(KeywordSearchProvider):
    """BM25 using VectorChord."""

    def __init__(
        self,
        session: AsyncSession,
    ) -> None:
        """Initialize the VectorChord BM25."""
        self.__session = session
        self._initialized = False

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
        await self.__session.execute(text(CREATE_VCHORD_EXTENSION))
        await self.__session.execute(text(CREATE_PG_TOKENIZER))
        await self.__session.execute(text(CREATE_VCHORD_BM25))
        await self.__session.execute(text(SET_SEARCH_PATH))
        await self._commit()

    async def _create_tokenizer_if_not_exists(self) -> None:
        """Create the tokenizer if it doesn't exist."""
        # Check if tokenizer exists in the catalog
        result = await self.__session.execute(text(TOKENIZER_NAME_CHECK_QUERY))
        if result.scalar_one_or_none() is None:
            # Tokenizer doesn't exist, create it
            await self.__session.execute(text(LOAD_TOKENIZER))
            await self._commit()

    async def _create_tables(self) -> None:
        """Create the necessary tables in the correct order."""
        await self.__session.execute(text(CREATE_BM25_TABLE))
        await self.__session.execute(text(CREATE_BM25_INDEX))
        await self._commit()

    async def _execute(
        self, query: TextClause, param_list: list[Any] | dict[str, Any] | None = None
    ) -> Result:
        """Execute a query."""
        if not self._initialized:
            await self._initialize()
        return await self.__session.execute(query, param_list)

    async def _commit(self) -> None:
        """Commit the session."""
        await self.__session.commit()

    async def index(self, corpus: list[BM25Document]) -> None:
        """Index a new corpus."""
        # Filter out any documents that don't have a snippet_id or text
        corpus = [
            doc
            for doc in corpus
            if doc.snippet_id is not None and doc.text is not None and doc.text != ""
        ]

        if not corpus:
            return

        # Execute inserts
        await self._execute(
            text(INSERT_QUERY),
            [{"snippet_id": doc.snippet_id, "passage": doc.text} for doc in corpus],
        )

        # Tokenize the new documents with schema qualification
        await self._execute(text(UPDATE_QUERY))
        await self._commit()

    async def delete(self, snippet_ids: list[int]) -> None:
        """Delete documents from the index."""
        await self._execute(
            text(DELETE_QUERY).bindparams(bindparam("snippet_ids", expanding=True)),
            {"snippet_ids": snippet_ids},
        )
        await self._commit()

    async def retrieve(
        self,
        query: str,
        top_k: int = 10,
    ) -> list[BM25Result]:
        """Search documents using BM25 similarity."""
        if not query or query == "":
            return []

        sql = text(SEARCH_QUERY).bindparams(query_text=query, limit=top_k)
        try:
            result = await self._execute(sql)
            rows = result.mappings().all()

            return [
                BM25Result(snippet_id=row["snippet_id"], score=row["bm25_score"])
                for row in rows
            ]
        except Exception as e:
            msg = f"Error during BM25 search: {e}"
            raise RuntimeError(msg) from e
