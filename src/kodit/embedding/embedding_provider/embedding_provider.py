"""Embedding provider."""

from abc import ABC, abstractmethod
from collections.abc import AsyncGenerator
from dataclasses import dataclass

import structlog
import tiktoken

OPENAI_MAX_EMBEDDING_SIZE = 8192

Vector = list[float]


@dataclass
class EmbeddingRequest:
    """Embedding request."""

    id: int
    text: str


@dataclass
class EmbeddingResponse:
    """Embedding response."""

    id: int
    embedding: Vector


class EmbeddingProvider(ABC):
    """Embedding provider."""

    @abstractmethod
    def embed(
        self, data: list[EmbeddingRequest]
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed a list of strings.

        The embedding provider is responsible for embedding a list of strings into a
        list of vectors. The embedding provider is responsible for splitting the list of
        strings into smaller sub-batches and embedding them in parallel.
        """


def split_sub_batches(
    encoding: tiktoken.Encoding,
    data: list[EmbeddingRequest],
    max_context_window: int = OPENAI_MAX_EMBEDDING_SIZE,
) -> list[list[EmbeddingRequest]]:
    """Split a list of strings into smaller sub-batches."""
    log = structlog.get_logger(__name__)
    result = []
    data_to_process = [s for s in data if s.text.strip()]  # Filter out empty strings

    while data_to_process:
        next_batch = []
        current_tokens = 0

        while data_to_process:
            next_item = data_to_process[0]
            item_tokens = len(encoding.encode(next_item.text, disallowed_special=()))

            if item_tokens > max_context_window:
                # Optimise truncation by operating on tokens directly instead of
                # removing one character at a time and repeatedly re-encoding.
                tokens = encoding.encode(next_item.text, disallowed_special=())
                if len(tokens) > max_context_window:
                    # Keep only the first *max_context_window* tokens.
                    tokens = tokens[:max_context_window]
                    # Convert back to text. This requires only one decode call and
                    # guarantees that the resulting string fits the token budget.
                    next_item.text = encoding.decode(tokens)
                    item_tokens = max_context_window  # We know the exact size now

                    data_to_process[0] = next_item

                    log.warning(
                        "Truncated snippet because it was too long to embed",
                        snippet=next_item.text[:100] + "...",
                    )

            if current_tokens + item_tokens > max_context_window:
                break

            next_batch.append(data_to_process.pop(0))
            current_tokens += item_tokens

        if next_batch:
            result.append(next_batch)

    return result
