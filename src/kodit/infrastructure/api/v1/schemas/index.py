"""JSON:API schemas for index operations."""

from datetime import datetime

from pydantic import BaseModel, Field


class IndexAttributes(BaseModel):
    """Index attributes for JSON:API responses."""

    created_at: datetime
    updated_at: datetime
    uri: str


class SourceData(BaseModel):
    """Source data for JSON:API relationships."""

    type: str = "source"
    id: str


class SourceRelationship(BaseModel):
    """Source relationship for JSON:API."""

    data: SourceData


class SnippetData(BaseModel):
    """Snippet data for JSON:API relationships."""

    type: str = "snippet"
    id: str


class SnippetsRelationship(BaseModel):
    """Snippets relationship for JSON:API."""

    data: list[SnippetData]


class IndexRelationships(BaseModel):
    """Index relationships for JSON:API responses."""

    source: SourceRelationship
    snippets: SnippetsRelationship | None = None


class IndexData(BaseModel):
    """Index data for JSON:API responses."""

    type: str = "index"
    id: str
    attributes: IndexAttributes


class IndexResponse(BaseModel):
    """JSON:API response for single index."""

    data: IndexData


class IndexListResponse(BaseModel):
    """JSON:API response for index list."""

    data: list[IndexData]


class IndexCreateAttributes(BaseModel):
    """Attributes for creating an index."""

    source_uri: str = Field(..., description="URI of the source to index")


class IndexCreateData(BaseModel):
    """Data for creating an index."""

    type: str = "index"
    attributes: IndexCreateAttributes


class IndexCreateRequest(BaseModel):
    """JSON:API request for creating an index."""

    data: IndexCreateData


class SourceAttributes(BaseModel):
    """Source attributes for JSON:API included resources."""

    created_at: datetime
    updated_at: datetime
    remote_uri: str
    cloned_path: str
    source_type: str


class AuthorData(BaseModel):
    """Author data for JSON:API relationships."""

    type: str = "author"
    id: str


class AuthorsRelationship(BaseModel):
    """Authors relationship for JSON:API."""

    data: list[AuthorData]


class FileRelationships(BaseModel):
    """File relationships for JSON:API."""

    authors: AuthorsRelationship


class FileAttributes(BaseModel):
    """File attributes for JSON:API included resources."""

    uri: str
    sha256: str
    mime_type: str
    created_at: datetime
    updated_at: datetime


class FileData(BaseModel):
    """File data for JSON:API included resources."""

    type: str = "file"
    id: str
    attributes: FileAttributes
    relationships: FileRelationships


class AuthorAttributes(BaseModel):
    """Author attributes for JSON:API included resources."""

    name: str
    email: str


class AuthorIncluded(BaseModel):
    """Author data for JSON:API included resources."""

    type: str = "author"
    id: str
    attributes: AuthorAttributes


class SourceIncluded(BaseModel):
    """Source data for JSON:API included resources."""

    type: str = "source"
    id: str
    attributes: SourceAttributes


class IndexDetailResponse(BaseModel):
    """JSON:API response for index details with included resources."""

    data: IndexData
    included: list[SourceIncluded] | None = None
