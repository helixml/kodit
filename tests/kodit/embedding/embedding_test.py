from kodit.embedding.embedding import TEST, EmbeddingInput, LocalEmbedder


async def test_embed():
    """Test the embed method."""
    embedding_service = LocalEmbedder(model_name=TEST)
    embedding = await anext(
        embedding_service.embed([EmbeddingInput(0, "Hello, world!")])
    )
    assert embedding.id == 0
    assert (
        len(embedding.embedding) > 100
    )  # Just check that the dimensions are reasonable
