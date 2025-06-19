"""Index service for managing code indexes.

This module provides the IndexService class which handles the business logic for
creating, listing, and running code indexes. It orchestrates the interaction between the
file system, database operations (via IndexRepository), and provides a clean API for
index management.
"""

from datetime import datetime

import pydantic
import structlog
from tqdm.asyncio import tqdm

from kodit.application.commands.snippet_commands import CreateIndexSnippetsCommand
from kodit.application.services.snippet_application_service import (
    SnippetApplicationService,
)
from kodit.domain.models import (
    BM25Document,
    BM25SearchResult,
    EnrichmentIndexRequest,
    EnrichmentRequest,
    SnippetExtractionStrategy,
    VectorIndexRequest,
    VectorSearchQueryRequest,
    VectorSearchRequest,
)
from kodit.domain.services.bm25_service import (
    BM25DomainService,
    BM25IndexRequest,
    BM25SearchRequest,
)
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.source_service import SourceService
from kodit.indexing.fusion import FusionRequest, reciprocal_rank_fusion
from kodit.indexing.indexing_repository import IndexRepository
from kodit.log import log_event
from kodit.util.spinner import Spinner

# List of MIME types that are blacklisted from being indexed
MIME_BLACKLIST = ["unknown/unknown"]


class IndexView(pydantic.BaseModel):
    """Data transfer object for index information.

    This model represents the public interface for index data, providing a clean
    view of index information without exposing internal implementation details.
    """

    id: int
    created_at: datetime
    updated_at: datetime | None = None
    source: str | None = None
    num_snippets: int


class SearchRequest(pydantic.BaseModel):
    """Request for a search."""

    text_query: str | None = None
    code_query: str | None = None
    keywords: list[str] | None = None
    top_k: int = 10


class SearchResult(pydantic.BaseModel):
    """Data transfer object for search results.

    This model represents a single search result, containing both the file path
    and the matching snippet content.
    """

    id: int
    uri: str
    content: str
    original_scores: list[float]


class IndexService:
    """Service for managing code indexes.

    This service handles the business logic for creating, listing, and running code
    indexes. It coordinates between file system operations, database operations (via
    IndexRepository), and provides a clean API for index management.
    """

    def __init__(  # noqa: PLR0913
        self,
        repository: IndexRepository,
        source_service: SourceService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        enrichment_service: EnrichmentDomainService,
        snippet_application_service: SnippetApplicationService,
    ) -> None:
        """Initialize the index service.

        Args:
            repository: The repository instance to use for database operations.
            source_service: The source service instance to use for source validation.
            bm25_service: The BM25 domain service for keyword search.
            code_search_service: The code search domain service.
            text_search_service: The text search domain service.
            enrichment_service: The enrichment domain service.
            snippet_application_service: The snippet application service.

        """
        self.repository = repository
        self.source_service = source_service
        self.snippet_application_service = snippet_application_service
        self.log = structlog.get_logger(__name__)
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.enrichment_service = enrichment_service

    async def create(self, source_id: int) -> IndexView:
        """Create a new index for a source.

        This method creates a new index for the specified source, after validating
        that the source exists and doesn't already have an index.

        Args:
            source_id: The ID of the source to create an index for.

        Returns:
            An Index object representing the newly created index.

        Raises:
            ValueError: If the source doesn't exist or already has an index.

        """
        log_event("kodit.index.create")

        # Check if the source exists
        source = await self.source_service.get(source_id)

        # Check if the index already exists
        index = await self.repository.get_by_source_id(source.id)
        if not index:
            index = await self.repository.create(source.id)
        return IndexView(
            id=index.id,
            created_at=index.created_at,
            num_snippets=await self.repository.num_snippets_for_index(index.id),
            source=source.uri,
        )

    async def list_indexes(self) -> list[IndexView]:
        """List all available indexes with their details.

        Returns:
            A list of Index objects containing information about each index,
            including file and snippet counts.

        """
        indexes = await self.repository.list_indexes()

        # Transform database results into DTOs
        indexes = [
            IndexView(
                id=index.id,
                created_at=index.created_at,
                updated_at=index.updated_at,
                num_snippets=await self.repository.num_snippets_for_index(index.id)
                or 0,
                source=source.uri,
            )
            for index, source in indexes
        ]

        # Help Kodit by measuring how much people are using indexes
        log_event(
            "kodit.index.list",
            {
                "num_indexes": len(indexes),
                "num_snippets": sum([index.num_snippets for index in indexes]),
            },
        )

        return indexes

    async def run(self, index_id: int) -> None:
        """Run the indexing process for a specific index."""
        log_event("kodit.index.run")

        # Get and validate index
        index = await self.repository.get_by_id(index_id)
        if not index:
            msg = f"Index not found: {index_id}"
            raise ValueError(msg)

        # Delete old snippets so we don't duplicate. In the future should probably check
        # which files have changed and only change those.
        await self.repository.delete_all_snippets(index.id)

        # Create snippets for supported file types using the new application service
        self.log.info("Creating snippets for files", index_id=index.id)
        command = CreateIndexSnippetsCommand(
            index_id=index.id, strategy=SnippetExtractionStrategy.METHOD_BASED
        )
        await self.snippet_application_service.create_snippets_for_index(command)

        snippets = await self.repository.get_all_snippets(index.id)

        self.log.info("Creating keyword index")
        with Spinner():
            await self.bm25_service.index_documents(
                BM25IndexRequest(
                    documents=[
                        BM25Document(snippet_id=snippet.id, text=snippet.content)
                        for snippet in snippets
                    ]
                )
            )

        self.log.info("Creating semantic code index")
        with tqdm(total=len(snippets), leave=False) as pbar:
            async for result in self.code_search_service.index_documents(
                VectorIndexRequest(
                    documents=[
                        VectorSearchRequest(snippet.id, snippet.content)
                        for snippet in snippets
                    ]
                )
            ):
                pbar.update(len(result))

        self.log.info("Enriching snippets", num_snippets=len(snippets))
        enriched_contents = []
        with tqdm(total=len(snippets), leave=False) as pbar:
            # Create domain request for enrichment
            enrichment_request = EnrichmentIndexRequest(
                requests=[
                    EnrichmentRequest(snippet_id=snippet.id, text=snippet.content)
                    for snippet in snippets
                ]
            )

            async for result in self.enrichment_service.enrich_documents(
                enrichment_request
            ):
                snippet = next(s for s in snippets if s.id == result.snippet_id)
                if snippet:
                    snippet.content = (
                        result.text + "\n\n```\n" + snippet.content + "\n```"
                    )
                    await self.repository.add_snippet(snippet)
                    enriched_contents.append(result)
                pbar.update(1)

        self.log.info("Creating semantic text index")
        with tqdm(total=len(snippets), leave=False) as pbar:
            async for result in self.text_search_service.index_documents(
                VectorIndexRequest(
                    documents=[
                        VectorSearchRequest(snippet.id, snippet.content)
                        for snippet in snippets
                    ]
                )
            ):
                pbar.update(len(result))

        # Update index timestamp
        await self.repository.update_index_timestamp(index)

    async def search(self, request: SearchRequest) -> list[SearchResult]:
        """Search for relevant data."""
        log_event("kodit.index.search")

        fusion_list: list[list[FusionRequest]] = []
        if request.keywords:
            # Gather results for each keyword
            result_ids: list[BM25SearchResult] = []
            for keyword in request.keywords:
                results = await self.bm25_service.search(
                    BM25SearchRequest(query=keyword, top_k=request.top_k)
                )
                result_ids.extend(results)

            fusion_list.append(
                [FusionRequest(id=x.snippet_id, score=x.score) for x in result_ids]
            )

        # Compute embedding for semantic query
        if request.code_query:
            query_embedding = await self.code_search_service.search(
                VectorSearchQueryRequest(query=request.code_query, top_k=request.top_k)
            )
            fusion_list.append(
                [FusionRequest(id=x.snippet_id, score=x.score) for x in query_embedding]
            )

        if request.text_query:
            query_embedding = await self.text_search_service.search(
                VectorSearchQueryRequest(query=request.text_query, top_k=request.top_k)
            )
            fusion_list.append(
                [FusionRequest(id=x.snippet_id, score=x.score) for x in query_embedding]
            )

        if len(fusion_list) == 0:
            return []

        # Combine all results together with RFF if required
        final_results = reciprocal_rank_fusion(
            rankings=fusion_list,
            k=60,
        )

        # Only keep top_k results
        final_results = final_results[: request.top_k]

        # Get snippets from database (up to top_k)
        search_results = await self.repository.list_snippets_by_ids(
            [x.id for x in final_results]
        )

        return [
            SearchResult(
                id=snippet.id,
                uri=file.uri,
                content=snippet.content,
                original_scores=fr.original_scores,
            )
            for (file, snippet), fr in zip(search_results, final_results, strict=True)
        ]
