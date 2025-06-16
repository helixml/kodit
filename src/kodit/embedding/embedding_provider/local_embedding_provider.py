"""Local embedding service."""

from __future__ import annotations

import os
from time import time
from typing import TYPE_CHECKING

import structlog

from kodit.embedding.embedding_provider.embedding_provider import (
    EmbeddingProvider,
    EmbeddingRequest,
    EmbeddingResponse,
    split_sub_batches,
)

if TYPE_CHECKING:
    from collections.abc import AsyncGenerator

    from sentence_transformers import SentenceTransformer
    from tiktoken import Encoding


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
        self.encoding_name = "text-embedding-3-small"
        self.embedding_model = None
        self.encoding = None

    def _encoding(self) -> Encoding:
        if self.encoding is None:
            from tiktoken import encoding_for_model

            start_time = time()
            self.encoding = encoding_for_model(self.encoding_name)
            self.log.debug(
                "Encoding loaded",
                model_name=self.encoding_name,
                duration=time() - start_time,
            )
        return self.encoding

    def _model(self) -> SentenceTransformer:
        """Get the embedding model."""
        if self.embedding_model is None:
            os.environ["TOKENIZERS_PARALLELISM"] = "false"  # Avoid warnings
            from sentence_transformers import SentenceTransformer

            start_time = time()
            self.embedding_model = SentenceTransformer(
                self.model_name,
                trust_remote_code=True,
            )
            self.log.debug(
                "Model loaded",
                model_name=self.model_name,
                duration=time() - start_time,
            )
        return self.embedding_model

    async def embed(
        self, data: list[EmbeddingRequest]
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed a list of strings."""
        model = self._model()

        batched_data = split_sub_batches(self._encoding(), data)

        for batch in batched_data:
            embeddings = model.encode(
                [i.text for i in batch], show_progress_bar=False, batch_size=4
            )
            yield [
                EmbeddingResponse(
                    id=item.id,
                    embedding=[float(x) for x in embedding],
                )
                for item, embedding in zip(batch, embeddings, strict=True)
            ]
