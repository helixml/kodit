"""VectorChord repository for document operations."""

import logging

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.bm25.keyword_search_service import (
    BM25Document,
    BM25Result,
    KeywordSearchProvider,
)

logger = logging.getLogger(__name__)

# SQL statements
CREATE_VCHORD_EXTENSION = "CREATE EXTENSION IF NOT EXISTS vchord CASCADE;"
CREATE_PG_TOKENIZER = "CREATE EXTENSION IF NOT EXISTS pg_tokenizer CASCADE;"
CREATE_VCHORD_BM25 = "CREATE EXTENSION IF NOT EXISTS vchord_bm25 CASCADE;"
SET_SEARCH_PATH = 'SET search_path TO "$user", public, bm25_catalog, pg_catalog, information_schema, tokenizer_catalog;'

CREATE_BM25_TABLE = """
CREATE TABLE IF NOT EXISTS {table_name} (
    id SERIAL PRIMARY KEY,
    snippet_id BIGINT NOT NULL REFERENCES snippets(id),
    passage TEXT,
    embedding bm25vector,
    UNIQUE(snippet_id)
)
"""

CREATE_BM25_INDEX = """
CREATE INDEX IF NOT EXISTS {index_name}
ON {table_name}
USING bm25 (embedding bm25_ops)
"""

LOAD_TOKENIZER = """
SELECT create_tokenizer('bert', $$
model = "llmlingua2"
pre_tokenizer = "unicode_segmentation"  # split texts according to the Unicode Standard Annex #29
[[character_filters]]
to_lowercase = {}                       # convert all characters to lowercase
[[character_filters]]
unicode_normalization = "nfkd"          # normalize the text to Unicode Normalization Form KD
[[token_filters]]
skip_non_alphanumeric = {}              # skip tokens that all characters are not alphanumeric
[[token_filters]]
stopwords = "nltk_english"              # remove stopwords using the nltk dictionary
[[token_filters]]
stemmer = "english_porter2"             # stem tokens using the English Porter2 stemmer
$$)
"""


class VectorChordRepository(KeywordSearchProvider):
    """Repository for VectorChord document operations."""

    def __init__(
        self,
        session: AsyncSession,
    ) -> None:
        """Initialize the VectorChord repository."""
        self.session = session
        self._initialized = False
        self._bm25_table_name = "vectorchord_bm25_documents"
        self._bm25_index_name = f"{self._bm25_table_name}_idx"

    async def _initialize(self) -> None:
        """Initialize the VectorChord environment."""
        try:
            # Execute each command separately with verification
            await self.session.execute(text(CREATE_VCHORD_EXTENSION))
            await self.session.execute(text(CREATE_PG_TOKENIZER))
            await self.session.execute(text(CREATE_VCHORD_BM25))
            await self.session.execute(text(SET_SEARCH_PATH))
            await self.session.commit()

            # Create the tokenizer
            await self.session.execute(text(LOAD_TOKENIZER))
            await self.session.commit()

            # Create tables with proper dependency order
            await self._create_tables()
            self._initialized = True
        except Exception as e:
            msg = f"Failed to initialize VectorChord repository: {e}"
            raise RuntimeError(msg) from e

    async def _create_tables(self) -> None:
        """Create the necessary tables in the correct order."""
        try:
            await self.session.execute(
                text(CREATE_BM25_TABLE.format(table_name=self._bm25_table_name))
            )
            await self.session.execute(
                text(
                    CREATE_BM25_INDEX.format(
                        table_name=self._bm25_table_name,
                        index_name=self._bm25_index_name,
                    )
                )
            )
            await self.session.commit()
        except Exception as e:
            logger.error(f"Error creating tables: {e}")
            raise

    async def index(self, corpus: list[BM25Document]) -> None:
        """Index a new corpus."""
        if not self._initialized:
            await self._initialize()

        if not corpus:
            return

        # Filter out any documents that don't have a snippet_id or text
        corpus = [
            doc
            for doc in corpus
            if doc.snippet_id is not None and doc.text is not None and doc.text != ""
        ]

        # Write the documents to the bm25 database in one big batch
        query = text(
            f"INSERT INTO {self._bm25_table_name} (snippet_id, passage) VALUES (:snippet_id, :passage)"
        )

        for doc in corpus:
            await self.session.execute(
                query, {"snippet_id": doc.snippet_id, "passage": doc.text}
            )
        await self.session.commit()

        # Tokenize the new documents with schema qualification
        query = text(
            f"UPDATE {self._bm25_table_name} SET embedding = tokenize(passage, 'bert')"
        )
        await self.session.execute(query)
        await self.session.commit()

    async def retrieve(
        self,
        query: str,
        top_k: int = 10,
    ) -> list[BM25Result]:
        """Search documents using BM25 similarity."""
        if not self._initialized:
            await self._initialize()

        if not query or query == "":
            return []

        table_name = self._bm25_table_name
        index_name = self._bm25_index_name

        sql = text(f"""
            SELECT 
                   snippet_id, 
                   embedding <&> to_bm25query('{index_name}', tokenize(:query_text,
                   'bert')) AS bm25_score 
                FROM {table_name} 
                ORDER BY bm25_score
                LIMIT :limit
        """)

        # Bind parameters separately to avoid SQL injection
        params = {"query_text": query, "limit": top_k}

        try:
            result = await self.session.execute(sql, params)
            rows = result.fetchall()

            # Convert to dictionary format
            return [BM25Result(snippet_id=row[0], score=row[1]) for row in rows]
        except Exception as e:
            logger.error(f"Error during BM25 search: {e}")
            raise
