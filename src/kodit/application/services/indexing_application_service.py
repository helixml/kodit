"""Application service for indexing operations."""

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.commands.snippet_commands import CreateIndexSnippetsCommand
from kodit.application.services.snippet_application_service import (
    SnippetApplicationService,
)
from kodit.domain.entities import Snippet
from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.errors import EmptySourceError
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.indexing_service import IndexingDomainService
from kodit.domain.services.source_service import SourceService
from kodit.domain.value_objects import (
    BM25Document,
    BM25IndexRequest,
    BM25SearchRequest,
    BM25SearchResult,
    EnrichmentIndexRequest,
    EnrichmentRequest,
    FusionRequest,
    IndexCreateRequest,
    IndexView,
    MultiSearchRequest,
    MultiSearchResult,
    VectorIndexRequest,
    VectorSearchQueryRequest,
    VectorSearchRequest,
)
from kodit.log import log_event
from kodit.reporting import Reporter


class IndexingApplicationService:
    """Application service for indexing operations.

    This service orchestrates the business logic for creating, listing, and running
    code indexes. It coordinates between domain services and provides a clean API
    for index management.
    """

    def __init__(  # noqa: PLR0913
        self,
        indexing_domain_service: IndexingDomainService,
        source_service: SourceService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        enrichment_service: EnrichmentDomainService,
        snippet_application_service: SnippetApplicationService,
        session: AsyncSession,
    ) -> None:
        """Initialize the indexing application service.

        Args:
            indexing_domain_service: The indexing domain service.
            source_service: The source service for source validation.
            bm25_service: The BM25 domain service for keyword search.
            code_search_service: The code search domain service.
            text_search_service: The text search domain service.
            enrichment_service: The enrichment domain service.
            snippet_application_service: The snippet application service.
            session: The database session for transaction management.

        """
        self.indexing_domain_service = indexing_domain_service
        self.source_service = source_service
        self.snippet_application_service = snippet_application_service
        self.session = session
        self.log = structlog.get_logger(__name__)
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.enrichment_service = enrichment_service

    async def create_index(self, source_id: int) -> IndexView:
        """Create a new index for a source.

        Args:
            source_id: The ID of the source to create an index for.

        Returns:
            An IndexView representing the newly created index.

        Raises:
            ValueError: If the source doesn't exist.

        """
        log_event("kodit.index.create")

        # Check if the source exists
        source = await self.source_service.get(source_id)

        # Create the index
        request = IndexCreateRequest(source_id=source.id)
        index_view = await self.indexing_domain_service.create_index(request)

        # Commit the index creation
        await self.session.commit()

        return index_view

    async def list_indexes(self) -> list[IndexView]:
        """List all available indexes with their details.

        Returns:
            A list of IndexView objects containing information about each index.

        """
        indexes = await self.indexing_domain_service.list_indexes()

        # Help Kodit by measuring how much people are using indexes
        log_event(
            "kodit.index.list",
            {
                "num_indexes": len(indexes),
                "num_snippets": sum([index.num_snippets for index in indexes]),
            },
        )

        return indexes

    async def run_index(
        self, index_id: int, progress_callback: ProgressCallback | None = None
    ) -> None:
        """Run the indexing process for a specific index.

        Args:
            index_id: The ID of the index to run.
            progress_callback: Optional progress callback for reporting progress.

        Raises:
            ValueError: If the index doesn't exist or no indexable snippets are found.

        """
        log_event("kodit.index.run")

        # Get and validate index
        index = await self.indexing_domain_service.get_index(index_id)
        if not index:
            msg = f"Index not found: {index_id}"
            raise ValueError(msg)

        # Delete old snippets so we don't duplicate
        await self.indexing_domain_service.delete_all_snippets(index.id)
        # Commit the deletion
        await self.session.commit()

        # Create snippets for supported file types using the snippet application service
        # (snippet_application_service handles its own commits)
        self.log.info("Creating snippets for files", index_id=index.id)
        command = CreateIndexSnippetsCommand(
            index_id=index.id, strategy=SnippetExtractionStrategy.METHOD_BASED
        )
        await self.snippet_application_service.create_snippets_for_index(
            command, progress_callback
        )

        snippets = await self.indexing_domain_service.get_snippets_for_index(index.id)

        # Check if any snippets were extracted
        if not snippets:
            msg = f"No indexable snippets found for index {index.id}"
            raise EmptySourceError(msg)

        # Create BM25 index
        self.log.info("Creating keyword index")
        reporter = Reporter(self.log, progress_callback)
        await reporter.start("bm25_index", len(snippets), "Creating keyword index...")
        await self._create_bm25_index(snippets, progress_callback)
        await reporter.done("bm25_index", "Keyword index created")

        # Create code embeddings
        self.log.info("Creating semantic code index")
        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "code_embeddings", len(snippets), "Creating code embeddings..."
        )
        await self._create_code_embeddings(snippets, progress_callback)
        await reporter.done("code_embeddings")

        # Enrich snippets
        self.log.info("Enriching snippets", num_snippets=len(snippets))
        reporter = Reporter(self.log, progress_callback)
        await reporter.start("enrichment", len(snippets), "Enriching snippets...")
        await self._enrich_snippets(snippets, progress_callback)
        await reporter.done("enrichment")

        # Create text embeddings
        self.log.info("Creating semantic text index")
        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "text_embeddings", len(snippets), "Creating text embeddings..."
        )
        await self._create_text_embeddings(snippets, progress_callback)
        await reporter.done("text_embeddings")

        # Update index timestamp
        await self.indexing_domain_service.update_index_timestamp(index.id)
        # Commit the timestamp update
        await self.session.commit()

    async def _create_bm25_index(
        self, snippets: list[Snippet], progress_callback: ProgressCallback | None = None
    ) -> None:
        """Create BM25 keyword index."""
        reporter = Reporter(self.log, progress_callback)
        await reporter.start("bm25_index", len(snippets), "Creating keyword index...")
        await self.bm25_service.index_documents(
            BM25IndexRequest(
                documents=[
                    BM25Document(snippet_id=snippet.id, text=snippet.content)
                    for snippet in snippets
                ]
            )
        )
        await reporter.done("bm25_index", "Keyword index created")

    async def _create_code_embeddings(
        self, snippets: list[Snippet], progress_callback: ProgressCallback | None = None
    ) -> None:
        """Create code embeddings."""
        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "code_embeddings", len(snippets), "Creating code embeddings..."
        )
        processed = 0
        async for result in self.code_search_service.index_documents(
            VectorIndexRequest(
                documents=[
                    VectorSearchRequest(snippet.id, snippet.content)
                    for snippet in snippets
                ]
            )
        ):
            processed += len(result)
            await reporter.step(
                "code_embeddings",
                processed,
                len(snippets),
                "Creating code embeddings...",
            )
        await reporter.done("code_embeddings")

    async def _enrich_snippets(
        self, snippets: list[Snippet], progress_callback: ProgressCallback | None = None
    ) -> None:
        """Enrich snippets with additional context."""
        reporter = Reporter(self.log, progress_callback)
        await reporter.start("enrichment", len(snippets), "Enriching snippets...")
        enriched_contents = []
        enrichment_request = EnrichmentIndexRequest(
            requests=[
                EnrichmentRequest(snippet_id=snippet.id, text=snippet.content)
                for snippet in snippets
            ]
        )

        processed = 0
        async for result in self.enrichment_service.enrich_documents(
            enrichment_request
        ):
            # Find the snippet by ID
            snippet = next(s for s in snippets if s.id == result.snippet_id)
            if snippet:
                # Update the content in the local entity for subsequent processing
                enriched_content = result.text + "\n\n```\n" + snippet.content + "\n```"
                snippet.content = enriched_content

                # UPDATE the existing snippet entity instead of creating a new one
                # This follows DDD principles and avoids duplicates
                await self.indexing_domain_service.update_snippet_content(
                    snippet.id, enriched_content
                )
                enriched_contents.append(result)

            processed += 1
            await reporter.step(
                "enrichment", processed, len(snippets), "Enriching snippets..."
            )

        # Commit all snippet content updates as a single transaction
        if enriched_contents:
            await self.session.commit()

        await reporter.done("enrichment")

    async def _create_text_embeddings(
        self, snippets: list[Snippet], progress_callback: ProgressCallback | None = None
    ) -> None:
        """Create text embeddings."""
        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "text_embeddings", len(snippets), "Creating text embeddings..."
        )
        processed = 0
        async for result in self.text_search_service.index_documents(
            VectorIndexRequest(
                documents=[
                    VectorSearchRequest(snippet.id, snippet.content)
                    for snippet in snippets
                ]
            )
        ):
            processed += len(result)
            await reporter.step(
                "text_embeddings",
                processed,
                len(snippets),
                "Creating text embeddings...",
            )
        await reporter.done("text_embeddings")

    async def search(self, request: MultiSearchRequest) -> list[MultiSearchResult]:
        """Search for relevant data.

        Args:
            request: The search request.

        Returns:
            A list of search results.

        """
        log_event("kodit.index.search")

        # If filters are provided, use the snippet repository search
        if request.filters:
            snippet_results = await self.snippet_application_service.search(request)
            return [
                MultiSearchResult(
                    id=snippet.id,
                    uri=snippet.source_uri,
                    content=snippet.content,
                    original_scores=[1.0],  # Default score for filtered results
                )
                for snippet in snippet_results
            ]

        # Otherwise use the existing fusion logic
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
        final_results = self.indexing_domain_service.perform_fusion(
            rankings=fusion_list,
            k=60,
        )

        # Only keep top_k results
        final_results = final_results[: request.top_k]

        # Get snippets from database (up to top_k)
        search_results = await self.indexing_domain_service.get_snippets_by_ids(
            [x.id for x in final_results]
        )

        return [
            MultiSearchResult(
                id=snippet["id"],
                uri=file["uri"],
                content=snippet["content"],
                original_scores=fr.original_scores,
            )
            for (file, snippet), fr in zip(search_results, final_results, strict=True)
        ]
