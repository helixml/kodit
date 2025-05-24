"""Embedding service."""

import os
from collections.abc import Generator

import structlog
from sentence_transformers import SentenceTransformer

TINY = "tiny"
CODE = "code"
TEST = "test"

COMMON_EMBEDDING_MODELS = {
    TINY: "ibm-granite/granite-embedding-30m-english",
    CODE: "flax-sentence-embeddings/st-codesearch-distilroberta-base",
    TEST: "minishlab/potion-base-4M",
}


class EmbeddingService:
    """Service for embeddings."""

    def __init__(self, model_name: str) -> None:
        """Initialize the embedding service."""
        self.log = structlog.get_logger(__name__)
        self.model_name = COMMON_EMBEDDING_MODELS.get(model_name, model_name)
        self.embedding_model = None

    def _model(self) -> SentenceTransformer:
        """Get the embedding model."""
        if self.embedding_model is None:
            os.environ["TOKENIZERS_PARALLELISM"] = "false"  # Avoid warnings
            self.embedding_model = SentenceTransformer(
                self.model_name,
                trust_remote_code=True,
                device="cpu",  # Force CPU so we don't have to install accelerate, etc.
            )
        return self.embedding_model

    def embed(self, snippets: list[str]) -> Generator[list[float], None, None]:
        """Embed a list of documents."""
        model = self._model()
        embeddings = model.encode(snippets, show_progress_bar=False, batch_size=4)
        for embedding in embeddings:
            yield [float(x) for x in embedding]

    def query(self, query: list[str]) -> Generator[list[float], None, None]:
        """Query the embedding model."""
        model = self._model()
        embeddings = model.encode(query, show_progress_bar=False, batch_size=4)
        for embedding in embeddings:
            yield [float(x) for x in embedding]
