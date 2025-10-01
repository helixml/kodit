"""Enrichment domain entities."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime


@dataclass
class EnrichmentV2(ABC):
    """Generic enrichment that can be attached to any entity."""

    entity_id: str
    content: str = ""
    id: int | None = None
    created_at: datetime | None = None
    updated_at: datetime | None = None

    @property
    @abstractmethod
    def type(self) -> str:
        """Return the enrichment type."""

    @abstractmethod
    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""


@dataclass
class SnippetEnrichment(EnrichmentV2):
    """Enrichment specific to code snippets."""

    @property
    def type(self) -> str:
        """Return the enrichment type."""
        return "snippet"

    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""
        return "snippet_v2"


@dataclass
class CommitEnrichment(EnrichmentV2):
    """Enrichment specific to commits."""

    @property
    def type(self) -> str:
        """Return the enrichment type."""
        return "commit"

    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""
        return "git_commit"


@dataclass
class ArchitectureEnrichment(EnrichmentV2):
    """Enrichment containing physical architecture discovery for a commit."""

    @property
    def type(self) -> str:
        """Return the enrichment type."""
        return "architecture"

    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""
        return "git_commit"
