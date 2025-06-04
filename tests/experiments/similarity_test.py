"""Simple script to test similarity between embeddings using openai."""

import os
from typing import List
import numpy as np
from openai import AsyncOpenAI

# Create openai client
client = AsyncOpenAI(api_key=os.getenv("OPENAI_API_KEY"))


async def get_embedding(text: str) -> List[float]:
    """Get embedding for a text using OpenAI's API."""
    response = await client.embeddings.create(
        model="text-embedding-3-small",
        input=text,
    )
    return response.data[0].embedding


def cosine_distance(a: List[float], b: List[float]) -> float:
    """Calculate cosine distance between two vectors.

    Cosine distance is 1 - cosine similarity.
    Returns a value between 0 (identical) and 2 (completely opposite).
    """
    similarity = np.dot(a, b) / (np.linalg.norm(a) * np.linalg.norm(b))
    return 1 - similarity


async def main():
    source_text = """
    import os
from typing import List
def helper_function(x: List[str]) -> str:
    return " ".join(x)
"""
    source_embedding = await get_embedding(source_text)

    # Test texts
    texts = [
        "def OpenAIEmbedding()",
    ]

    # Get embeddings for all texts
    embeddings = []
    for text in texts:
        embedding = await get_embedding(text)
        embeddings.append(embedding)

    # Calculate cosine distance between source embedding and all other embeddings
    print("\nCosine Distances (0 = identical, 2 = completely opposite):")
    for i, embedding in enumerate(embeddings):
        distance = cosine_distance(source_embedding, embedding)
        print(f"\nText {i + 1}: {texts[i]}")
        print(f"Distance: {distance:.4f}")


if __name__ == "__main__":
    import asyncio

    asyncio.run(main())
