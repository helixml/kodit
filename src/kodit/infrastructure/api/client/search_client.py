"""Search operations API client for Kodit server."""

from datetime import datetime

from kodit.infrastructure.api.v1.schemas.search import (
    SearchAttributes,
    SearchData,
    SearchFilters,
    SearchRequest,
    SearchResponse,
    SnippetData,
)

from .base import BaseAPIClient
from .generated_endpoints import APIEndpoints


class SearchClient(BaseAPIClient):
    """API client for search operations."""

    async def search(  # noqa: PLR0913
        self,
        keywords: list[str] | None = None,
        code_query: str | None = None,
        text_query: str | None = None,
        limit: int = 10,
        languages: list[str] | None = None,
        authors: list[str] | None = None,
        start_date: datetime | None = None,
        end_date: datetime | None = None,
        sources: list[str] | None = None,
        file_patterns: list[str] | None = None,
    ) -> list[SnippetData]:
        """Search for code snippets.

        Args:
            keywords: Keywords to search for
            code_query: Code search query
            text_query: Text search query
            limit: Maximum number of results
            languages: Programming languages to filter by
            authors: Authors to filter by
            start_date: Filter snippets created after this date
            end_date: Filter snippets created before this date
            sources: Source repositories to filter by
            file_patterns: File path patterns to filter by

        Returns:
            List of matching snippets

        Raises:
            KoditAPIError: If the request fails

        """
        filters = None
        if any([languages, authors, start_date, end_date, sources, file_patterns]):
            filters = SearchFilters(
                languages=languages,
                authors=authors,
                start_date=start_date,
                end_date=end_date,
                sources=sources,
                file_patterns=file_patterns,
            )

        request = SearchRequest(
            data=SearchData(
                type="search",
                attributes=SearchAttributes(
                    keywords=keywords,
                    code=code_query,
                    text=text_query,
                    limit=limit,
                    filters=filters,
                ),
            )
        )

        response = await self._request(
            "POST",
            APIEndpoints.API_V1_SEARCH,
            json=request.model_dump(exclude_none=True),
        )

        result = SearchResponse.model_validate_json(response.text)
        return result.data
