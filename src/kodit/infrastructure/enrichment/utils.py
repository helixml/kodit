"""Utility functions for enrichment processing."""

import re


def clean_thinking_tags(content: str) -> str:
    """Remove <think>...</think> tags from content.

    This utility handles thinking tags that may be produced by various AI models,
    including both local and remote models. It safely removes thinking content
    while preserving the actual response.

    Args:
        content: The content that may contain thinking tags.

    Returns:
        The content with thinking tags removed and cleaned up.

    """
    if not content:
        return content

    # Remove thinking tags using regex with DOTALL flag to match across newlines
    cleaned = re.sub(
        r"<think>.*?</think>", "", content, flags=re.DOTALL | re.IGNORECASE
    )

    # Clean up extra whitespace that may be left behind
    cleaned = re.sub(r"\n\s*\n\s*\n", "\n\n", cleaned)
    return cleaned.strip()  # Remove leading/trailing whitespace
