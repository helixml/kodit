"""Locally hosted BM25 service primarily for use with SQLite."""

from __future__ import annotations

import json
from pathlib import Path
from typing import TYPE_CHECKING

import aiofiles
import Stemmer
import structlog

from kodit.bm25.keyword_search_service import (
    BM25Document,
    BM25Result,
    KeywordSearchProvider,
)

if TYPE_CHECKING:
    import bm25s
    from bm25s.tokenization import Tokenized


SNIPPET_IDS_FILE = "snippet_ids.jsonl"


class BM25Service(KeywordSearchProvider):
    """LocalBM25 service."""

    def __init__(self, data_dir: Path) -> None:
        """Initialize the BM25 service."""
        self.log = structlog.get_logger(__name__)
        self.index_path = data_dir / "bm25s_index"
        self.snippet_ids: list[int] = []
        self.stemmer = Stemmer.Stemmer("english")
        self.__retriever: bm25s.BM25 | None = None

    def _retriever(self) -> bm25s.BM25:
        """Get the BM25 retriever."""
        if self.__retriever is None:
            import bm25s

            try:
                self.log.debug("Loading BM25 index")
                self.__retriever = bm25s.BM25.load(self.index_path, mmap=True)
                with Path(self.index_path / SNIPPET_IDS_FILE).open() as f:
                    self.snippet_ids = json.load(f)
            except FileNotFoundError:
                self.log.debug("BM25 index not found, creating new index")
                self.__retriever = bm25s.BM25()
        return self.__retriever

    def _tokenize(self, corpus: list[str]) -> list[list[str]] | Tokenized:
        from bm25s import tokenize

        return tokenize(
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
        self._retriever().index(vocab, show_progress=False)
        self._retriever().save(self.index_path)
        self.snippet_ids = self.snippet_ids + [doc.snippet_id for doc in corpus]
        async with aiofiles.open(self.index_path / SNIPPET_IDS_FILE, "w") as f:
            await f.write(json.dumps(self.snippet_ids))

    async def retrieve(self, query: str, top_k: int = 2) -> list[BM25Result]:
        """Retrieve from the index."""
        if top_k == 0:
            self.log.warning("Top k is 0, returning empty list")
            return []

        # Check that the index has data
        if not hasattr(self._retriever(), "scores"):
            return []

        # Get the number of documents in the index
        num_docs = self._retriever().scores["num_docs"]
        if num_docs == 0:
            return []

        # Adjust top_k to not exceed corpus size
        top_k = min(top_k, num_docs)
        self.log.debug(
            "Retrieving from index",
            query=query,
            top_k=top_k,
        )

        query_tokens = self._tokenize([query])

        self.log.debug("Query tokens", query_tokens=query_tokens)

        results, scores = self._retriever().retrieve(
            query_tokens=query_tokens,
            corpus=self.snippet_ids,
            k=top_k,
        )
        self.log.debug("Raw results", results=results, scores=scores)
        return [
            BM25Result(snippet_id=int(result), score=float(score))
            for result, score in zip(results[0], scores[0], strict=False)
            if score > 0.0
        ]

    async def delete(self, snippet_ids: list[int]) -> None:  # noqa: ARG002
        """Delete documents from the index."""
        self.log.warning("Deletion not supported for local BM25 index")
