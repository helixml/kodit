"""Tag JSON-API schemas."""

from pydantic import BaseModel


class TagAttributes(BaseModel):
    """Tag attributes following JSON-API spec."""

    name: str
    target_commit_sha: str
    is_version_tag: bool


class TagData(BaseModel):
    """Tag data following JSON-API spec."""

    type: str = "tag"
    id: str  # The tag name
    attributes: TagAttributes


class TagResponse(BaseModel):
    """Single tag response following JSON-API spec."""

    data: TagData


class TagListResponse(BaseModel):
    """Tag list response following JSON-API spec."""

    data: list[TagData]
