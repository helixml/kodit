"""Fusion functions for combining search results."""

from collections import defaultdict
from dataclasses import dataclass


@dataclass
class FusionResult:
    """Result of a fusion operation."""

    id: int
    score: float
    original_scores: list[float]


@dataclass
class FusionRequest:
    """Result of a RRF operation."""

    id: int
    score: float


def reciprocal_rank_fusion(
    rankings: list[list[FusionRequest]], k: float = 60
) -> list[FusionResult]:
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
            scores[rank.id] = float(0)

    for ranker in rankings:
        for i, rank in enumerate(ranker):
            scores[rank.id] += 1.0 / (k + i)

    # Create a list of tuples of ids and their scores
    results = [(rank, scores[rank]) for rank in scores]

    # Sort results by score
    results.sort(key=lambda x: x[1], reverse=True)

    # Create a map of original scores to ids
    original_scores_to_ids = defaultdict(list)
    for ranker in rankings:
        for rank in ranker:
            original_scores_to_ids[rank.id].append(rank.score)

    # Rebuild a list of final results with their original scores
    return [
        FusionResult(
            id=result[0],
            score=result[1],
            original_scores=original_scores_to_ids[result[0]],
        )
        for result in results
    ]
