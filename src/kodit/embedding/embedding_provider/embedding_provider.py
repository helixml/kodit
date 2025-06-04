"""Embedding provider."""

from abc import ABC, abstractmethod

import structlog
import tiktoken

OPENAI_MAX_EMBEDDING_SIZE = 8192

Vector = list[float]


class EmbeddingProvider(ABC):
    """Embedding provider."""

    @abstractmethod
    async def embed(self, data: list[str]) -> list[Vector]:
        """Embed a list of strings.

        The embedding provider is responsible for embedding a list of strings into a
        list of vectors. The embedding provider is responsible for splitting the list of
        strings into smaller sub-batches and embedding them in parallel.
        """


def split_sub_batches(encoding: tiktoken.Encoding, data: list[str]) -> list[list[str]]:
    """Split a list of strings into smaller sub-batches."""
    log = structlog.get_logger(__name__)
    result = []
    data_to_process = [s for s in data if s.strip()]  # Filter out empty strings

    while data_to_process:
        next_batch = []
        current_tokens = 0

        while data_to_process:
            next_item = data_to_process[0]
            item_tokens = len(encoding.encode(next_item))

            if item_tokens > OPENAI_MAX_EMBEDDING_SIZE:
                # Loop around trying to truncate the snippet until it fits in the max
                # embedding size
                while item_tokens > OPENAI_MAX_EMBEDDING_SIZE:
                    next_item = next_item[:-1]
                    item_tokens = len(encoding.encode(next_item))

                data_to_process[0] = next_item

                log.warning("Truncated snippet", snippet=next_item)

            if current_tokens + item_tokens > OPENAI_MAX_EMBEDDING_SIZE:
                break

            next_batch.append(data_to_process.pop(0))
            current_tokens += item_tokens

        if next_batch:
            result.append(next_batch)

    return result
