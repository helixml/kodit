from collections.abc import Generator

import structlog
from fastembed import TextEmbedding

TINY = "tiny"
CODE = "code"

COMMON_EMBEDDING_MODELS = {
    TINY: "BAAI/bge-small-en-v1.5",
    CODE: "jinaai/jina-embeddings-v2-base-code",
}


class EmbeddingService:
    """Service for embeddings."""

    def __init__(self, model_name: str = CODE) -> None:
        """Initialize the embedding service."""
        self.log = structlog.get_logger(__name__)
        self.model_name = COMMON_EMBEDDING_MODELS.get(model_name, model_name)
        self.embedding_model = None  # Lazy load the model

    def _model(self) -> TextEmbedding:
        """Get the embedding model."""
        if self.embedding_model is None:
            self.embedding_model = TextEmbedding(model_name=self.model_name)
        return self.embedding_model

    def embed(self, snippets: list[str]) -> Generator[list[float], None, None]:
        """Embed a list of documents."""
        model = self._model()
        embeddings = model.embed(snippets)
        for embedding in embeddings:
            # Convert the numpy array to floats
            yield [float(x) for x in embedding]
