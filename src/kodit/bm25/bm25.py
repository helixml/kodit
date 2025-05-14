"""BM25 service."""

import bm25s
import Stemmer
import structlog

from kodit.config import DATA_DIR

BM25_INDEX_PATH = DATA_DIR / "bm25s_index"


class BM25Service:
    """Service for BM25."""

    def __init__(self) -> None:
        """Initialize the BM25 service."""
        self.log = structlog.get_logger(__name__)

        try:
            self.log.debug("Loading BM25 index")
            self.retriever = bm25s.BM25.load(BM25_INDEX_PATH, mmap=True)
        except FileNotFoundError:
            self.log.debug("BM25 index not found, creating new index")
            self.retriever = bm25s.BM25()

        self.stemmer = Stemmer.Stemmer("english")

    def index(self, corpus: list[str]) -> None:
        """Index a new corpus."""
        self.log.debug("Indexing corpus")
        vocab = bm25s.tokenize(
            corpus,
            stopwords="en",
            stemmer=self.stemmer,
            return_ids=False,
            show_progress=True,
        )
        self.retriever = bm25s.BM25()
        self.retriever.index(vocab)
        self.retriever.save(BM25_INDEX_PATH)

    def retrieve(self, doc_ids: list[int], query: str, top_k: int = 2) -> list[int]:
        """Retrieve from the index."""
        top_k = min(top_k, len(doc_ids))

        query_tokens = bm25s.tokenize(query, stemmer=self.stemmer)

        results, scores = self.retriever.retrieve(
            query_tokens=query_tokens, corpus=doc_ids, k=top_k
        )
        scores = f"Scores: {scores[0]}"
        self.log.debug(scores)

        return [int(result) for result in results[0]]
