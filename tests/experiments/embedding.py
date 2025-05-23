"""Test several embedding models for performace."""

import numpy as np
import psutil
from kodit.embedding.embedding import EmbeddingService

EXAMPLE_CODE = """
from contextlib import asynccontextmanager

from fastapi import FastAPI


def fake_answer_to_everything_ml_model(x: float):
    return x * 42


ml_models = {}


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Load the ML model
    ml_models["answer_to_everything"] = fake_answer_to_everything_ml_model
    yield
    # Clean up the ML models and release the resources
    ml_models.clear()


app = FastAPI(lifespan=lifespan)


@app.get("/predict")
async def predict(x: float):
    result = ml_models["answer_to_everything"](x)
    return {"result": result}
"""


def test_embedding_performance():
    """Test several embedding models for performace."""
    models_under_test = [
        "minishlab/potion-base-4M",  # teeny weeny
        "ibm-granite/granite-embedding-30m-english",
        "jinaai/jina-embeddings-v2-small-en",  # Best smallest "normal" embedding
        "flax-sentence-embeddings/st-codesearch-distilroberta-base",  # Best smallest "code" embedding
        "jinaai/jina-embeddings-v2-base-en",
        "sentence-transformers/all-MiniLM-L6-v2",  # Default sentence-transformers model
        "BAAI/bge-code-v1",
        "nomic-ai/nomic-embed-code",
        "mchochlov/codebert-base-cd-ft",
        "Shuu12121/CodeSearch-ModernBERT-Crow-Plus",
        "nomic-ai/CodeRankEmbed",
        "codesage/codesage-small-v2",
        "codesage/codesage-large-v2",
        "Salesforce/SFR-Embedding-Code-400M_R",
    ]
    # Pre-download the models and print some stats about the model
    print("Downloading models and printing stats...")
    for model in models_under_test:
        embedding_service = EmbeddingService(model)
        sen_model = embedding_service._model()
        dims = sen_model.get_sentence_embedding_dimension()
        total_num = sum(p.numel() for p in sen_model.parameters()) / 1_000_000
        print(f"{model} has {dims} dimensions and {total_num:.2f}M parameters")

    print("Testing embedding performance. Should be HIGH, LOW, ZERO")
    for model in models_under_test:
        embedding_service = EmbeddingService(model)
        embeddings = next(embedding_service.embed([EXAMPLE_CODE]))
        query = [
            "The user wants to add hooks for startup and shutdown in their fastapi application.",
            "The user wants to develop a quicksort algorithm.",
            "Swans are the white rose of Selby.",
        ]
        query_embeddings = embedding_service.query(query)

        # Cosine similarity
        similarities = []
        for query_embedding in query_embeddings:
            similarity = np.dot(embeddings, query_embedding) / (
                np.linalg.norm(embeddings) * np.linalg.norm(query_embedding)
            )
            similarities.append(similarity)

        print(f"{model}, similarities: {', '.join([f'{s:.2f}' for s in similarities])}")


if __name__ == "__main__":
    test_embedding_performance()
