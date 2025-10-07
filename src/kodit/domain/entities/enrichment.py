"""Enrichment domain entities."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime

ENRICHMENT_TYPE_ARCHITECTURE = "architecture"
ENRICHMENT_TYPE_DEVELOPMENT = "development"
ENRICHMENT_SUBTYPE_PHYSICAL = "physical"
ENRICHMENT_SUBTYPE_GENERAL = "general"
ENRICHMENT_SUBTYPE_SNIPPET = "snippet"


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

    @property
    @abstractmethod
    def subtype(self) -> str | None:
        """Return the enrichment subtype (optional for hierarchical types)."""

    @abstractmethod
    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""


@dataclass
class SnippetEnrichment(EnrichmentV2):
    """Enrichment specific to code snippets."""

    @property
    def type(self) -> str:
        """Return the enrichment type."""
        return ENRICHMENT_TYPE_DEVELOPMENT

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_SNIPPET

    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""
        return "snippet_v2"


@dataclass
class CommitEnrichment(EnrichmentV2, ABC):
    """Enrichment specific to commits."""

    def entity_type_key(self) -> str:
        """Return the entity type key this enrichment is for."""
        return "git_commit"


@dataclass
class PhysicalArchitectureEnrichment(CommitEnrichment):
    """Enrichment containing physical architecture discovery for a commit."""

    @property
    def type(self) -> str:
        """Return the enrichment type."""
        return ENRICHMENT_TYPE_ARCHITECTURE

    @property
    def subtype(self) -> str | None:
        """Return the enrichment subtype."""
        return ENRICHMENT_SUBTYPE_PHYSICAL
