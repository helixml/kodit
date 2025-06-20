"""Integration tests for enrichment functionality."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from kodit.application.services.indexing_application_service import (
    IndexingApplicationService,
)
from kodit.domain.entities import Snippet
from kodit.domain.value_objects import (
    EnrichmentIndexRequest,
    EnrichmentRequest,
    EnrichmentResponse,
)
from kodit.domain.services.enrichment_service import (
    EnrichmentDomainService,
    EnrichmentProvider,
)
from kodit.infrastructure.enrichment.null_enrichment_provider import (
    NullEnrichmentProvider,
)


class TestEnrichmentIntegration:
    """Integration tests for enrichment functionality."""

    @pytest.mark.asyncio
    async def test_enrichment_pipeline_with_null_provider(self):
        """Test the complete enrichment pipeline with null provider."""
        # Create a mock indexing domain service
        mock_indexing_domain_service = MagicMock()
        mock_indexing_domain_service.get_snippets_for_index.return_value = [
            {"id": 1, "content": "def hello(): pass"},
            {"id": 2, "content": "def world(): pass"},
        ]
        mock_indexing_domain_service.add_snippet = AsyncMock()
        mock_indexing_domain_service.update_index_timestamp = AsyncMock()

        # Create enrichment domain service with null provider
        enrichment_provider = NullEnrichmentProvider()
        enrichment_domain_service = EnrichmentDomainService(enrichment_provider)

        # Create enrichment request
        enrichment_request = EnrichmentIndexRequest(
            requests=[
                EnrichmentRequest(snippet_id=1, text="def hello(): pass"),
                EnrichmentRequest(snippet_id=2, text="def world(): pass"),
            ]
        )

        # Process enrichment
        results = []
        async for result in enrichment_domain_service.enrich_documents(
            enrichment_request
        ):
            results.append(result)

        # Verify results
        assert len(results) == 2
        assert results[0].snippet_id == 1
        assert results[0].text == ""
        assert results[1].snippet_id == 2
        assert results[1].text == ""

    @pytest.mark.asyncio
    async def test_enrichment_in_indexing_application_service(self):
        """Test enrichment integration in the indexing application service."""
        # Create mock services
        mock_snippet_application_service = MagicMock()
        mock_snippet_application_service.create_snippets_for_index = AsyncMock()

        mock_bm25_service = MagicMock()
        mock_bm25_service.index_documents = AsyncMock()

        mock_code_search_service = MagicMock()
        mock_code_search_service.index_documents = AsyncMock()

        mock_text_search_service = MagicMock()
        mock_text_search_service.index_documents = AsyncMock()

        mock_source_service = MagicMock()

        # Create enrichment service with null provider
        enrichment_provider = NullEnrichmentProvider()
        enrichment_domain_service = EnrichmentDomainService(enrichment_provider)

        # Create indexing domain service mock
        mock_indexing_domain_service = MagicMock()
        mock_index = MagicMock()
        mock_index.id = 1
        mock_indexing_domain_service.get_index = AsyncMock(return_value=mock_index)
        mock_indexing_domain_service.delete_all_snippets = AsyncMock()
        # Create mock Snippet entities
        mock_snippet1 = MagicMock(spec=Snippet)
        mock_snippet1.id = 1
        mock_snippet1.content = "def hello(): pass"
        mock_snippet2 = MagicMock(spec=Snippet)
        mock_snippet2.id = 2
        mock_snippet2.content = "def world(): pass"

        mock_indexing_domain_service.get_snippets_for_index = AsyncMock(
            return_value=[mock_snippet1, mock_snippet2]
        )
        mock_indexing_domain_service.add_snippet = AsyncMock()
        mock_indexing_domain_service.update_snippet_content = AsyncMock()
        mock_indexing_domain_service.update_index_timestamp = AsyncMock()

        # Mock the search methods to return empty results
        async def mock_index_documents(*args, **kwargs):
            yield []

        mock_code_search_service.index_documents = mock_index_documents
        mock_text_search_service.index_documents = mock_index_documents

        # Create the indexing application service
        service = IndexingApplicationService(
            indexing_domain_service=mock_indexing_domain_service,
            snippet_application_service=mock_snippet_application_service,
            bm25_service=mock_bm25_service,
            code_search_service=mock_code_search_service,
            text_search_service=mock_text_search_service,
            enrichment_service=enrichment_domain_service,
            source_service=mock_source_service,
        )

        # Run the index
        await service.run_index(1)

        # Verify that snippets were enriched and updated
        assert mock_indexing_domain_service.update_snippet_content.call_count == 2

        # Verify that the index timestamp was updated
        assert mock_indexing_domain_service.update_index_timestamp.call_count == 1

    @pytest.mark.asyncio
    async def test_enrichment_content_format(self):
        """Test that enrichment content is properly formatted."""

        # Create a mock enrichment provider that returns actual content
        class MockEnrichmentProvider(EnrichmentProvider):
            async def enrich(self, requests):
                for request in requests:
                    yield EnrichmentResponse(
                        snippet_id=request.snippet_id,
                        text=f"This is an explanation of: {request.text}",
                    )

        # Create enrichment domain service
        enrichment_provider = MockEnrichmentProvider()
        enrichment_domain_service = EnrichmentDomainService(enrichment_provider)

        # Create enrichment request
        enrichment_request = EnrichmentIndexRequest(
            requests=[
                EnrichmentRequest(snippet_id=1, text="def hello(): pass"),
            ]
        )

        # Process enrichment
        results = []
        async for result in enrichment_domain_service.enrich_documents(
            enrichment_request
        ):
            results.append(result)

        # Verify the enrichment content
        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == "This is an explanation of: def hello(): pass"

    @pytest.mark.asyncio
    async def test_enrichment_with_empty_requests(self):
        """Test enrichment with empty requests."""
        enrichment_provider = NullEnrichmentProvider()
        enrichment_domain_service = EnrichmentDomainService(enrichment_provider)

        # Create empty enrichment request
        enrichment_request = EnrichmentIndexRequest(requests=[])

        # Process enrichment
        results = []
        async for result in enrichment_domain_service.enrich_documents(
            enrichment_request
        ):
            results.append(result)

        # Should return no results
        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_enrichment_preserves_snippet_ids(self):
        """Test that enrichment preserves snippet IDs correctly."""

        # Create a mock enrichment provider
        class MockEnrichmentProvider(EnrichmentProvider):
            async def enrich(self, requests):
                for request in requests:
                    yield EnrichmentResponse(
                        snippet_id=request.snippet_id,
                        text=f"Enriched content for snippet {request.snippet_id}",
                    )

        enrichment_provider = MockEnrichmentProvider()
        enrichment_domain_service = EnrichmentDomainService(enrichment_provider)

        # Create enrichment request with specific IDs
        enrichment_request = EnrichmentIndexRequest(
            requests=[
                EnrichmentRequest(snippet_id=42, text="def test(): pass"),
                EnrichmentRequest(snippet_id=123, text="def another(): pass"),
            ]
        )

        # Process enrichment
        results = []
        async for result in enrichment_domain_service.enrich_documents(
            enrichment_request
        ):
            results.append(result)

        # Verify snippet IDs are preserved
        assert len(results) == 2
        assert results[0].snippet_id == 42
        assert results[1].snippet_id == 123
        assert results[0].text == "Enriched content for snippet 42"
        assert results[1].text == "Enriched content for snippet 123"
