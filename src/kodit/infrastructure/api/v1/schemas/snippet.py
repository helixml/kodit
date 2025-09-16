"""Snippet JSON-API schemas."""

from datetime import datetime

from pydantic import BaseModel


class SnippetContentSchema(BaseModel):
    """Snippet content schema following JSON-API spec."""

    value: str
    language: str


class GitFileSchema(BaseModel):
    """Git file schema following JSON-API spec."""

    blob_sha: str
    path: str
    mime_type: str
    size: int


class EnrichmentSchema(BaseModel):
    """Enrichment schema following JSON-API spec."""

    type: str
    content: str


class SnippetAttributes(BaseModel):
    """Snippet attributes following JSON-API spec."""

    created_at: datetime | None = None
    updated_at: datetime | None = None
    derives_from: list[GitFileSchema]
    content: SnippetContentSchema
    enrichments: list[EnrichmentSchema]


class SnippetData(BaseModel):
    """Snippet data following JSON-API spec."""

    type: str = "snippet"
    id: str
    attributes: SnippetAttributes


class SnippetResponse(BaseModel):
    """Single snippet response following JSON-API spec."""

    data: SnippetData


class SnippetListResponse(BaseModel):
    """Snippet list response following JSON-API spec."""

    data: list[SnippetData]
