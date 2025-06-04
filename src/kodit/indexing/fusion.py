"""Fusion functions for combining search results."""


def reciprocal_rank_fusion(
    rankings: list[list[int]], k: float = 60
) -> list[tuple[int, float]]:
    """RRF prioritises results that are present in all results.

    Args:
        rankings: List of rankers, each containing a list of document ids. Top of the
        list is considered to be the best result.
        k: Parameter for RRF.

    Returns:
        Dictionary of ids and their scores.

    """
    scores = {}
    for ranker in rankings:
        for rank in ranker:
            scores[rank] = float(0)

    for ranker in rankings:
        for i, rank in enumerate(ranker):
            scores[rank] += 1.0 / (k + i)

    # Create a list of tuples of ids and their scores
    results = [(rank, scores[rank]) for rank in scores]

    # Sort results by score
    results.sort(key=lambda x: x[1], reverse=True)

    return results
