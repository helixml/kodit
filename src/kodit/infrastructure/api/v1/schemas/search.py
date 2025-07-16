"""JSON:API schemas for search operations."""

from datetime import datetime

from pydantic import BaseModel, Field


class SearchFilters(BaseModel):
    """Search filters for JSON:API requests."""

    languages: list[str] | None = Field(
        None, description="Programming languages to filter by"
    )
    authors: list[str] | None = Field(None, description="Authors to filter by")
    start_date: datetime | None = Field(
        None, description="Filter snippets created after this date"
    )
    end_date: datetime | None = Field(
        None, description="Filter snippets created before this date"
    )
    sources: list[str] | None = Field(
        None, description="Source repositories to filter by"
    )
    file_patterns: list[str] | None = Field(
        None, description="File path patterns to filter by"
    )


class SearchAttributes(BaseModel):
    """Search attributes for JSON:API requests."""

    keywords: list[str] | None = Field(None, description="Search keywords")
    code: str | None = Field(None, description="Code search query")
    text: str | None = Field(None, description="Text search query")
    limit: int | None = Field(10, description="Maximum number of results to return")
    filters: SearchFilters | None = Field(None, description="Search filters")


class SearchData(BaseModel):
    """Search data for JSON:API requests."""

    type: str = "search"
    attributes: SearchAttributes


class SearchRequest(BaseModel):
    """JSON:API request for searching snippets."""

    data: SearchData

    @property
    def limit(self) -> int | None:
        """Get the limit from the search request."""
        return self.data.attributes.limit

    @property
    def languages(self) -> list[str] | None:
        """Get the languages from the search request."""
        return (
            self.data.attributes.filters.languages
            if self.data.attributes.filters
            else None
        )

    @property
    def authors(self) -> list[str] | None:
        """Get the authors from the search request."""
        return (
            self.data.attributes.filters.authors
            if self.data.attributes.filters
            else None
        )

    @property
    def start_date(self) -> datetime | None:
        """Get the start date from the search request."""
        return (
            self.data.attributes.filters.start_date
            if self.data.attributes.filters
            else None
        )

    @property
    def end_date(self) -> datetime | None:
        """Get the end date from the search request."""
        return (
            self.data.attributes.filters.end_date
            if self.data.attributes.filters
            else None
        )

    @property
    def sources(self) -> list[str] | None:
        """Get the sources from the search request."""
        return (
            self.data.attributes.filters.sources
            if self.data.attributes.filters
            else None
        )

    @property
    def file_patterns(self) -> list[str] | None:
        """Get the file patterns from the search request."""
        return (
            self.data.attributes.filters.file_patterns
            if self.data.attributes.filters
            else None
        )


class SnippetAttributes(BaseModel):
    """Snippet attributes for JSON:API responses."""

    content: str
    created_at: datetime
    updated_at: datetime
    original_scores: list[float]
    source_uri: str
    relative_path: str
    language: str
    authors: list[str]
    summary: str


class SnippetData(BaseModel):
    """Snippet data for JSON:API responses."""

    type: str = "snippet"
    id: int
    attributes: SnippetAttributes


class SearchResponse(BaseModel):
    """JSON:API response for search results."""

    data: list[SnippetData]


class FileAttributes(BaseModel):
    """File attributes for JSON:API included resources."""

    uri: str
    sha256: str
    mime_type: str
    created_at: datetime
    updated_at: datetime


class AuthorData(BaseModel):
    """Author data for JSON:API relationships."""

    type: str = "author"
    id: int


class AuthorsRelationship(BaseModel):
    """Authors relationship for JSON:API."""

    data: list[AuthorData]


class FileRelationships(BaseModel):
    """File relationships for JSON:API."""

    authors: AuthorsRelationship


class FileDataWithRelationships(BaseModel):
    """File data with relationships for JSON:API included resources."""

    type: str = "file"
    id: int
    attributes: FileAttributes
    relationships: FileRelationships


class AuthorAttributes(BaseModel):
    """Author attributes for JSON:API included resources."""

    name: str
    email: str


class AuthorDataWithAttributes(BaseModel):
    """Author data with attributes for JSON:API included resources."""

    type: str = "author"
    id: int
    attributes: AuthorAttributes


class SearchResponseWithIncluded(BaseModel):
    """JSON:API response for search results with included resources."""

    data: list[SnippetData]
    included: list[FileDataWithRelationships | AuthorDataWithAttributes] | None = None


class SnippetDetailAttributes(BaseModel):
    """Snippet detail attributes for JSON:API responses."""

    created_at: datetime
    updated_at: datetime
    original_content: dict
    summary_content: dict


class SnippetDetailData(BaseModel):
    """Snippet detail data for JSON:API responses."""

    type: str = "snippet"
    id: str
    attributes: SnippetDetailAttributes


class SnippetDetailResponse(BaseModel):
    """JSON:API response for snippet details."""

    data: SnippetDetailData
