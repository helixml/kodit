"""Utilities for batching embedding requests based on token counts and batch size.

This module centralises the logic for splitting a list of ``EmbeddingRequest``
into smaller sub-batches that respect token limits (and optionally a maximum
number of items per batch).  Both the OpenAI and Local embedding providers use
this functionality.
"""

from tiktoken import Encoding

from kodit.domain.value_objects import EmbeddingRequest

__all__ = [
    "split_sub_batches",
]


DEFAULT_MAX_TOKENS = 8192  # A conservative upper-bound for most embedding models


def split_sub_batches(
    encoding: Encoding,
    data: list[EmbeddingRequest],
    *,
    max_tokens: int = DEFAULT_MAX_TOKENS,
    batch_size: int | None = None,
) -> list[list[EmbeddingRequest]]:
    """Split *data* into sub-batches constrained by tokens and size.

    Parameters
    ----------
    encoding
        A *tiktoken* ``Encoding`` instance capable of counting tokens.
    data
        List of :class:`kodit.domain.value_objects.EmbeddingRequest` objects.
    max_tokens
        Maximum number of tokens allowed in a single batch.  Defaults to
        ``DEFAULT_MAX_TOKENS``.
    batch_size
        Optional maximum number of items per batch.  If *None*, no explicit
        size constraint is applied (token limit still applies).

    Returns
    -------
    list[list[EmbeddingRequest]]
        A list of batches where each batch is a list of ``EmbeddingRequest``s.

    """
    batches: list[list[EmbeddingRequest]] = []
    current_batch: list[EmbeddingRequest] = []
    current_tokens = 0

    for original_item in data:
        # ------------------------------------------------------------------
        # Ensure **individual** requests never exceed the token limit.
        # If they do, we *truncate* them rather than sending an oversized
        # request to the embedding model (which would raise a 400 error).
        # ------------------------------------------------------------------

        token_ids = encoding.encode(original_item.text, disallowed_special=())
        if len(token_ids) > max_tokens:
            # Keep only the first *max_tokens* tokens and decode back to text.
            token_ids = token_ids[:max_tokens]
            truncated_text = encoding.decode(token_ids)

            # Create a *new* EmbeddingRequest to avoid mutating the caller's
            # objects (side-effects can be surprising).
            item = EmbeddingRequest(
                snippet_id=original_item.snippet_id,
                text=truncated_text,
            )
        else:
            item = original_item

        item_tokens = len(token_ids)

        # Determine whether adding the item would violate limits for the
        # *current* batch. Note: size constraint is optional.
        token_overflow = current_tokens + item_tokens > max_tokens
        size_overflow = batch_size is not None and len(current_batch) >= batch_size

        if (token_overflow or size_overflow) and current_batch:
            batches.append(current_batch)
            current_batch = [item]
            current_tokens = item_tokens
        else:
            current_batch.append(item)
            current_tokens += item_tokens

    if current_batch:
        batches.append(current_batch)

    return batches
