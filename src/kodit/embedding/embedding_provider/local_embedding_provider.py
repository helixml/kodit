"""Local embedding service."""

import os

import structlog
import tiktoken
from sentence_transformers import SentenceTransformer
from tqdm import tqdm

from kodit.embedding.embedding_provider.embedding_provider import (
    EmbeddingProvider,
    Vector,
    split_sub_batches,
)

TINY = "tiny"
CODE = "code"
TEST = "test"

COMMON_EMBEDDING_MODELS = {
    TINY: "ibm-granite/granite-embedding-30m-english",
    CODE: "flax-sentence-embeddings/st-codesearch-distilroberta-base",
    TEST: "minishlab/potion-base-4M",
}


class LocalEmbeddingProvider(EmbeddingProvider):
    """Local embedder."""

    def __init__(self, model_name: str) -> None:
        """Initialize the local embedder."""
        self.log = structlog.get_logger(__name__)
        self.model_name = COMMON_EMBEDDING_MODELS.get(model_name, model_name)
        self.embedding_model = None
        self.encoding = tiktoken.encoding_for_model("text-embedding-3-small")

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

    async def embed(self, data: list[str]) -> list[Vector]:
        """Embed a list of strings."""
        model = self._model()

        batched_data = split_sub_batches(self.encoding, data)

        results: list[Vector] = []
        for batch in tqdm(batched_data, total=len(batched_data), leave=False):
            embeddings = model.encode(batch, show_progress_bar=False, batch_size=4)
            results.extend([[float(x) for x in embedding] for embedding in embeddings])
        return results
