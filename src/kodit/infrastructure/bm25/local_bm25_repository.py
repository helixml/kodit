"""Local BM25 repository implementation."""

from __future__ import annotations

import json
from pathlib import Path
from typing import TYPE_CHECKING

import aiofiles
import Stemmer  # type: ignore[import-not-found]
import structlog

from kodit.domain.services.bm25_service import BM25Repository
from kodit.domain.value_objects import (
    DeleteRequest,
    IndexRequest,
    SearchRequest,
    SearchResult,
)

if TYPE_CHECKING:
    import bm25s  # type: ignore[import-untyped]
    from bm25s.tokenization import Tokenized  # type: ignore[import-untyped]

SNIPPET_IDS_FILE = "snippet_ids.jsonl"


class LocalBM25Repository(BM25Repository):
    """Local BM25 repository implementation."""

    def __init__(self, data_dir: Path) -> None:
        """Initialize the local BM25 repository.

        Args:
            data_dir: Directory to store BM25 index files

        """
        self.log = structlog.get_logger(__name__)
        self.index_path = data_dir / "bm25s_index"
        self.snippet_ids: list[str] = []
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
        """Tokenize text corpus."""
        from bm25s import tokenize

        return tokenize(
            corpus,
            stopwords="en",
            stemmer=self.stemmer,
            return_ids=False,
            show_progress=True,
            lower=True,
        )

    async def index_documents(self, request: IndexRequest) -> None:
        """Index documents for BM25 search."""
        self.log.debug("Indexing corpus")
        if not request.documents:
            self.log.warning("Corpus is empty, skipping bm25 index")
            return

        if not self.snippet_ids and (self.index_path / SNIPPET_IDS_FILE).exists():
            async with aiofiles.open(self.index_path / SNIPPET_IDS_FILE) as f:
                self.snippet_ids = json.loads(await f.read())

        # Filter out documents that have already been indexed
        new_documents = [
            doc for doc in request.documents if doc.snippet_id not in self.snippet_ids
        ]
        if not new_documents:
            self.log.info("No new documents to index")
            return

        vocab = self._tokenize([doc.text for doc in new_documents])
        self._retriever().index(vocab, show_progress=False)
        self._retriever().save(self.index_path)
        # Replace snippet_ids instead of appending, since the BM25 index is rebuilt
        self.snippet_ids = [doc.snippet_id for doc in new_documents]
        async with aiofiles.open(self.index_path / SNIPPET_IDS_FILE, "w") as f:
            await f.write(json.dumps(self.snippet_ids))

    async def search(self, request: SearchRequest) -> list[SearchResult]:
        """Search documents using BM25."""
        if request.top_k == 0:
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
        top_k = min(request.top_k, num_docs)
        self.log.debug(
            "Retrieving from index",
            query=request.query,
            top_k=top_k,
        )

        query_tokens = self._tokenize([request.query])

        self.log.debug("Query tokens", query_tokens=query_tokens)

        results, scores = self._retriever().retrieve(
            query_tokens=query_tokens,
            corpus=self.snippet_ids,
            k=top_k,
        )
        self.log.debug("Raw results", results=results, scores=scores)

        # Filter results by snippet_ids if provided
        filtered_results = []
        for result, score in zip(results[0], scores[0], strict=True):
            snippet_id = result
            if score > 0.0 and (
                request.snippet_ids is None or snippet_id in request.snippet_ids
            ):
                filtered_results.append(
                    SearchResult(snippet_id=snippet_id, score=float(score))
                )

        return filtered_results

    async def delete_documents(self, request: DeleteRequest) -> None:
        """Delete documents from the index."""
        # request parameter is unused as deletion is not supported
        # ruff: noqa: ARG002
        self.log.warning("Deletion not supported for local BM25 index")
