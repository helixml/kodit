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


class EnrichmentData(BaseModel):
    """Enrichment data following JSON-API spec."""

    type: str = "enrichment"
    id: str
    attributes: EnrichmentAttributes


class EnrichmentListResponse(BaseModel):
    """Enrichment list response following JSON-API spec."""

    data: list[EnrichmentData]
