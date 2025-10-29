"""Enrichment JSON-API schemas."""

from datetime import datetime

from pydantic import BaseModel


class EnrichmentAttributes(BaseModel):
    """Enrichment attributes following JSON-API spec."""

    type: str
    subtype: str | None
    content: str
    created_at: datetime | None
    updated_at: datetime | None


class EnrichmentAssociationData(BaseModel):
    """Enrichment association data for JSON-API spec."""

    id: str
    type: str


class EnrichmentRelationships(BaseModel):
    """Enrichment relationships for JSON-API spec."""

    associations: list[EnrichmentAssociationData] | None = None


class EnrichmentData(BaseModel):
    """Enrichment data following JSON-API spec."""

    type: str = "enrichment"
    id: str
    attributes: EnrichmentAttributes
    relationships: EnrichmentRelationships | None = None


class EnrichmentListResponse(BaseModel):
    """Enrichment list response following JSON-API spec."""

    data: list[EnrichmentData]
