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
    source_text = """This code snippet is part of a Python class definition for a service that uses vector embeddings for searching, specifically tailored for a task related to "VectorChord". 

Here's a breakdown of its components:

- **Imports**: The snippet imports various types and modules from SQLAlchemy (for asynchronous operations with databases) and a custom embedding provider for handling embeddings.

- **Class Definition**: The class `VectorChordVectorSearchService` inherits from another class called `VectorSearchService`. This implies that it is extending or specializing functionality from its superclass.

- **Constructor (`__init__` method)**: The constructor initializes the object with the following attributes:
  - `self.embedding_provider`: Stores an embedding provider instance, which is likely responsible for generating or managing embeddings.
  - `self._session`: Holds a reference to an asynchronous database session for executing database queries.
  - `self._initialized`: A boolean flag indicating whether the service has been initialized.
  - `self.table_name`: Constructs a table name using a task identifier, which likely pertains to a specific dataset or operation.
  - `self.index_name`: Creates an index name associated with the table for efficient searching.

Overall, this code appears to set up a service that manages embeddings and associated database interactions for a task called "VectorChord."
"""
    source_embedding = await get_embedding(source_text)

    # Test texts
    texts = [
        "vectorchord semantic search implementation",
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
