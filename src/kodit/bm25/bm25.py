"""BM25 service."""

from pathlib import Path

import bm25s
import Stemmer
import structlog
from bm25s.tokenization import Tokenized

from kodit.bm25.keyword_search_service import (
    BM25Document,
    BM25Result,
    KeywordSearchProvider,
)


class BM25Service(KeywordSearchProvider):
    """Service for BM25."""

    def __init__(self, data_dir: Path) -> None:
        """Initialize the BM25 service."""
        self.log = structlog.get_logger(__name__)
        self.index_path = data_dir / "bm25s_index"
        try:
            self.log.debug("Loading BM25 index")
            self.retriever = bm25s.BM25.load(self.index_path, mmap=True)
        except FileNotFoundError:
            self.log.debug("BM25 index not found, creating new index")
            self.retriever = bm25s.BM25()

        self.stemmer = Stemmer.Stemmer("english")

    def _tokenize(self, corpus: list[str]) -> list[list[str]] | Tokenized:
        return bm25s.tokenize(
            corpus,
            stopwords="en",
            stemmer=self.stemmer,
            return_ids=False,
            show_progress=True,
        )

    async def index(self, corpus: list[BM25Document]) -> None:
        """Index a new corpus."""
        self.log.debug("Indexing corpus")
        vocab = self._tokenize([doc.text for doc in corpus])
        self.retriever = bm25s.BM25()
        self.retriever.index(vocab, show_progress=False)
        self.retriever.save(self.index_path)

    async def retrieve(self, query: str, top_k: int = 2) -> list[BM25Result]:
        """Retrieve from the index."""
        if top_k == 0:
            self.log.warning("Top k is 0, returning empty list")
            return []

        top_k = min(top_k, len(self.retriever.scores))
        self.log.debug(
            "Retrieving from index",
            query=query,
            top_k=top_k,
        )

        query_tokens = self._tokenize([query])

        self.log.debug("Query tokens", query_tokens=query_tokens)

        results, scores = self.retriever.retrieve(
            query_tokens=query_tokens,
            corpus=list(range(len(self.retriever.scores))),
            k=top_k,
        )
        self.log.debug("Raw results", results=results, scores=scores)
        return [
            BM25Result(snippet_id=int(result), score=float(score))
            for result, score in zip(results[0], scores[0], strict=False)
        ]
