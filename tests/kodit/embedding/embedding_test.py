from kodit.embedding.embedding import TINY, EmbeddingService
from sqlalchemy import (
    String,
    create_engine,
    Column,
    Integer,
    JSON,
    Float,
    select,
    func,
    cast,
    literal,
)
from sqlalchemy.orm import declarative_base, Session
import math, json


def test_embed() -> None:
    """Test the embed method."""
    embedding_service = EmbeddingService(model_name=TINY)
    embeddings = list(embedding_service.embed(["Hello, world!"]))
    assert len(embeddings) == 1
    assert len(embeddings[0]) == 384


def test_sqlite_embed() -> None:
    engine = create_engine("sqlite:///:memory:", echo=False)
    Base = declarative_base()

    class Embedding(Base):
        __tablename__ = "embeddings"
        id = Column(Integer, primary_key=True)
        text = Column(String)
        vec = Column(JSON)  # stored as TEXT in SQLite

    Base.metadata.create_all(engine)

    texts = ["Hello, world!", "People, be nice."]

    embedding_service = EmbeddingService(model_name=TINY)
    embeddings = list(embedding_service.embed(texts))

    # Insert some dummy data
    with Session(engine) as s:
        for text, embedding in zip(texts, embeddings):
            s.add(Embedding(text=text, vec=embedding))
        s.commit()

    # ---------------------------------------------- run a query
    query_vector = list(embedding_service.embed(["hello"]))[0]
    cos_dist = cosine_distance_json(Embedding.vec, query_vector).label("cos_dist")

    stmt = select(Embedding.id, Embedding.text, cos_dist).order_by(cos_dist)

    with Session(engine) as s:
        results = s.execute(stmt).all()

    assert len(results) == 2
    assert results[0].text == texts[0]
    assert results[1].text == texts[1]


def cosine_distance_json(col, query_vec):
    """
    Build a SQLAlchemy scalar expression that returns
    1 – cosine_similarity(json_array, query_vec).
    Works for a *fixed-length* vector.
    """
    q_norm = math.sqrt(sum(x * x for x in query_vec))

    dot = sum(
        cast(func.json_extract(col, f"$[{i}]"), Float) * literal(float(q))
        for i, q in enumerate(query_vec)
    )
    row_norm = func.sqrt(
        sum(
            cast(func.json_extract(col, f"$[{i}]"), Float)
            * cast(func.json_extract(col, f"$[{i}]"), Float)
            for i in range(len(query_vec))
        )
    )

    return 1 - dot / (row_norm * literal(q_norm))
