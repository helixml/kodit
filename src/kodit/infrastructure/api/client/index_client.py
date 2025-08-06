"""Index operations API client for Kodit server."""

from kodit.infrastructure.api.v1.schemas.index import (
    IndexCreateAttributes,
    IndexCreateData,
    IndexCreateRequest,
    IndexData,
    IndexListResponse,
    IndexResponse,
)

from .base import BaseAPIClient
from .exceptions import KoditAPIError
from .generated_endpoints import APIEndpoints


class IndexClient(BaseAPIClient):
    """API client for index operations."""

    async def list_indexes(self) -> list[IndexData]:
        """List all indexes."""
        response = await self._request("GET", APIEndpoints.API_V1_INDEXES)
        data = IndexListResponse.model_validate_json(response.text)
        return data.data

    async def create_index(self, uri: str) -> IndexData:
        """Create a new index."""
        request = IndexCreateRequest(
            data=IndexCreateData(
                type="index", attributes=IndexCreateAttributes(uri=uri)
            )
        )
        response = await self._request(
            "POST", APIEndpoints.API_V1_INDEXES, json=request.model_dump()
        )
        result = IndexResponse.model_validate_json(response.text)
        return result.data

    async def get_index(self, index_id: str) -> IndexData | None:
        """Get index by ID."""
        try:
            response = await self._request(
                "GET", APIEndpoints.API_V1_INDEXES_INDEX_ID.format(index_id=index_id)
            )
            result = IndexResponse.model_validate_json(response.text)
        except KoditAPIError as e:
            if "404" in str(e):
                return None
            raise
        else:
            return result.data

    async def delete_index(self, index_id: str) -> None:
        """Delete an index."""
        await self._request(
            "DELETE", APIEndpoints.API_V1_INDEXES_INDEX_ID.format(index_id=index_id)
        )
