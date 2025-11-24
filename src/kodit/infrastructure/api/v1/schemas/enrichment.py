"""Enrichment JSON-API schemas."""

from datetime import datetime

from pydantic import BaseModel, Field


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


class EnrichmentLinks(BaseModel):
    """Links following JSON-API spec."""

    self_link: str = Field(..., alias="self")

    model_config = {"populate_by_name": True}


class RelationshipData(BaseModel):
    """Data for a single relationship."""

    type: str
    id: str


class Relationship(BaseModel):
    """A JSON:API relationship."""

    links: EnrichmentLinks | None = None
    data: RelationshipData | list[RelationshipData] | None = None


class EnrichmentRelationships(BaseModel):
    """Enrichment relationships for JSON-API spec."""

    associations: list[EnrichmentAssociationData] | None = None
    commit: Relationship | None = None


class EnrichmentData(BaseModel):
    """Enrichment data following JSON-API spec."""

    type: str = "enrichment"
    id: str
    attributes: EnrichmentAttributes
    relationships: EnrichmentRelationships | None = None
    links: EnrichmentLinks | None = None


class EnrichmentListResponse(BaseModel):
    """Enrichment list response following JSON-API spec."""

    data: list[EnrichmentData]


class EnrichmentResponse(BaseModel):
    """Single enrichment response following JSON-API spec."""

    data: EnrichmentData


class EnrichmentUpdateAttributes(BaseModel):
    """Attributes for updating an enrichment."""

    content: str


class EnrichmentUpdateData(BaseModel):
    """Data for updating an enrichment."""

    type: str = "enrichment"
    attributes: EnrichmentUpdateAttributes


class EnrichmentUpdateRequest(BaseModel):
    """Request to update an enrichment."""

    data: EnrichmentUpdateData
