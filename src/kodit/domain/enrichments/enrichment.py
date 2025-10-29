"""Enrichment domain entities."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime


@dataclass(frozen=True)
class EnrichmentV2(ABC):
    """Generic enrichment that can be attached to any entity."""

    content: str = ""
    id: int | None = None
    created_at: datetime | None = None
    updated_at: datetime | None = None

    @property
    @abstractmethod
    def type(self) -> str:
        """Return the enrichment type."""

    @property
    @abstractmethod
    def subtype(self) -> str | None:
        """Return the enrichment subtype (optional for hierarchical types)."""

    @abstractmethod
    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""


@dataclass(frozen=True)
class CommitEnrichment(EnrichmentV2, ABC):
    """Enrichment specific to commits."""

    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""
        return "git_commit"


@dataclass(frozen=True)
class EnrichmentAssociation:
    """Association between an enrichment and an entity."""

    enrichment_id: int
    entity_type: str
    entity_id: str
    id: int | None = None


@dataclass
class SnippetSummaryAssociation:
    """Association between a snippet and a summary enrichment."""

    snippet_summary: EnrichmentAssociation
    snippet: EnrichmentAssociation
