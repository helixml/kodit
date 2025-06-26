"""Batching tests."""

import math

import tiktoken

from kodit.domain.value_objects import EmbeddingRequest
from kodit.infrastructure.embedding.embedding_providers.batching import (
    split_sub_batches,
)


def test_split_sub_batches_handles_endoftext_token() -> None:
    """Ensure the special ``<|endoftext|>`` token is batched without errors."""
    # Use the same encoding as the OpenAI embedding models
    encoding = tiktoken.encoding_for_model("text-embedding-3-small")

    # Single request that is just the special token
    data = [EmbeddingRequest(snippet_id=1, text="<|endoftext|>")]

    # Perform batching - any reasonable token limit should keep this in one batch
    batches = split_sub_batches(encoding, data, max_tokens=10)

    assert len(batches) == 1, "Expected a single batch for one short request"
    assert len(batches[0]) == 1

    # Ensure the token is recognised and tokenised (should yield at least one token).
    assert len(encoding.encode("<|endoftext|>", disallowed_special=())) >= 1


def test_split_sub_batches_respects_token_limit() -> None:
    """Verify that batches never exceed the *max_tokens* constraint."""
    encoding = tiktoken.encoding_for_model("text-embedding-3-small")

    sample_text = "hello world"
    tokens_per_item = len(encoding.encode(sample_text, disallowed_special=()))
    assert tokens_per_item > 0, "Tokenization should yield at least one token"

    # Allow a maximum of *two* items worth of tokens per batch
    max_tokens = tokens_per_item * 2

    num_items = 5  # Deliberately more than can fit in a single batch
    data = [EmbeddingRequest(snippet_id=i, text=sample_text) for i in range(num_items)]

    batches = split_sub_batches(encoding, data, max_tokens=max_tokens)

    # Expect ceil(num_items / 2) batches because only two items fit per batch
    expected_batches = math.ceil(num_items / 2)
    assert len(batches) == expected_batches

    # Ensure no batch exceeds the token limit
    for batch in batches:
        batch_tokens = sum(
            len(encoding.encode(item.text, disallowed_special=())) for item in batch
        )
        assert batch_tokens <= max_tokens, (
            "A batch exceeded the specified max_token limit"
        )


def test_split_sub_batches_truncates_long_items() -> None:
    """Items exceeding *max_tokens* should be truncated to the limit."""
    encoding = tiktoken.encoding_for_model("text-embedding-3-small")

    max_tokens = 50  # small for test purposes

    # Build a string that **definitely** exceeds *max_tokens* tokens, without
    # relying on any assumptions about the encoding's tokenisation granularity.
    base_chunk = "Lorem ipsum dolor sit amet "  # 6-7 tokens depending on model
    long_text = base_chunk
    while len(encoding.encode(long_text, disallowed_special=())) <= max_tokens:
        long_text += base_chunk

    data = [EmbeddingRequest(snippet_id=1, text=long_text)]

    batches = split_sub_batches(encoding, data, max_tokens=max_tokens)

    # We still expect a single batch containing the truncated item
    assert len(batches) == 1
    assert len(batches[0]) == 1

    truncated_item = batches[0][0]
    truncated_tokens = len(encoding.encode(truncated_item.text, disallowed_special=()))
    assert truncated_tokens == max_tokens, (
        "Item was not truncated to the max_tokens limit"
    )
