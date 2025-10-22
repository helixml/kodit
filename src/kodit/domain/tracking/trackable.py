"""Trackable value objects."""

from dataclasses import dataclass
from enum import StrEnum


class TrackableReferenceType(StrEnum):
    """Types of git references that can be tracked."""

    BRANCH = "branch"
    TAG = "tag"
    COMMIT_SHA = "commit_sha"


@dataclass(frozen=True)
class Trackable:
    """Represents a trackable reference point in a git repository."""

    type: TrackableReferenceType
    identifier: str  # e.g., "main", "v1.0.0", "abc123..."
    repo_id: int
