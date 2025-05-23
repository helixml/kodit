from kodit.embedding.embedding import TEST, EmbeddingService


def test_embed() -> None:
    """Test the embed method."""
    embedding_service = EmbeddingService(model_name=TEST)
    embeddings = list(embedding_service.embed(["Hello, world!"]))
    assert len(embeddings) == 1
    assert len(embeddings[0]) > 100  # Just check that the dimensions are reasonable
