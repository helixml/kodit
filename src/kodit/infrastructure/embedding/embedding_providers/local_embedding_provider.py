"""Local embedding provider implementation."""

import os
from collections.abc import AsyncGenerator
from time import time
from typing import TYPE_CHECKING

import structlog

from kodit.domain.services.embedding_service import EmbeddingProvider
from kodit.domain.value_objects import EmbeddingRequest, EmbeddingResponse

if TYPE_CHECKING:
    from sentence_transformers import SentenceTransformer
    from tiktoken import Encoding

# Constants for different embedding models
TINY = "tiny"
CODE = "code"
TEST = "test"

COMMON_EMBEDDING_MODELS = {
    TINY: "ibm-granite/granite-embedding-30m-english",
    CODE: "flax-sentence-embeddings/st-codesearch-distilroberta-base",
    TEST: "minishlab/potion-base-4M",
}


class LocalEmbeddingProvider(EmbeddingProvider):
    """Local embedding provider that uses sentence-transformers."""

    def __init__(self, model_name: str = CODE) -> None:
        """Initialize the local embedding provider.

        Args:
            model_name: The model name to use for embeddings. Can be a preset
                       ('tiny', 'code', 'test') or a full model name.

        """
        self.log = structlog.get_logger(__name__)
        self.model_name = COMMON_EMBEDDING_MODELS.get(model_name, model_name)
        self.encoding_name = "text-embedding-3-small"
        self.embedding_model: SentenceTransformer | None = None
        self.encoding: Encoding | None = None

    def _encoding(self) -> "Encoding":
        """Get the tiktoken encoding."""
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

    def _model(self) -> "SentenceTransformer":
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

    def embed(
        self, data: list[EmbeddingRequest]
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed a list of strings using the local model."""
        if not data:

            async def empty_generator() -> AsyncGenerator[
                list[EmbeddingResponse], None
            ]:
                if False:
                    yield []

            return empty_generator()

        model = self._model()
        encoding = self._encoding()

        # Split into sub-batches based on token limits
        batched_data = self._split_sub_batches(encoding, data)

        async def _embed_batches() -> AsyncGenerator[list[EmbeddingResponse], None]:
            for batch in batched_data:
                try:
                    # Encode the texts using the model
                    embeddings = model.encode(
                        [item.text for item in batch],
                        show_progress_bar=False,
                        batch_size=4,
                    )

                    # Convert to our response format
                    responses = [
                        EmbeddingResponse(
                            snippet_id=item.snippet_id,
                            embedding=[float(x) for x in embedding],
                        )
                        for item, embedding in zip(batch, embeddings, strict=True)
                    ]

                    yield responses

                except Exception as e:
                    self.log.exception("Error generating embeddings", error=str(e))
                    # Return zero embeddings on error
                    responses = [
                        EmbeddingResponse(
                            snippet_id=item.snippet_id,
                            embedding=[0.0] * 1536,  # Default embedding size
                        )
                        for item in batch
                    ]
                    yield responses

        return _embed_batches()

    def _split_sub_batches(
        self, encoding: "Encoding", data: list[EmbeddingRequest]
    ) -> list[list[EmbeddingRequest]]:
        """Split data into sub-batches based on token limits."""
        max_tokens = 8192  # Conservative token limit
        batches = []
        current_batch = []
        current_tokens = 0

        for item in data:
            # Count tokens for this item
            tokens = len(encoding.encode(item.text))

            # If adding this item would exceed the limit, start a new batch
            if current_tokens + tokens > max_tokens and current_batch:
                batches.append(current_batch)
                current_batch = [item]
                current_tokens = tokens
            else:
                current_batch.append(item)
                current_tokens += tokens

        # Add the last batch if it has items
        if current_batch:
            batches.append(current_batch)

        return batches
