"""Integration tests for embedding functionality."""

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.value_objects import (
    Document,
    FileProcessingStatus,
    IndexRequest,
    SearchRequest,
    SearchResult,
)
from kodit.infrastructure.embedding.embedding_providers.hash_embedding_provider import (
    HashEmbeddingProvider,
)
from kodit.infrastructure.embedding.local_vector_search_repository import (
    LocalVectorSearchRepository,
)
from kodit.infrastructure.sqlalchemy.embedding_repository import (
    SqlAlchemyEmbeddingRepository,
)
from kodit.infrastructure.sqlalchemy.entities import (
    EmbeddingType,
    File,
    Index,
    Snippet,
    Source,
    SourceType,
)


class TestEmbeddingIntegration:
    """Integration tests for embedding functionality."""

    @pytest.mark.asyncio
    async def test_full_embedding_pipeline_local(self, session: AsyncSession) -> None:
        """Test the full embedding pipeline with hash provider."""
        # Create real components
        embedding_repository = SqlAlchemyEmbeddingRepository(session=session)
        embedding_provider = HashEmbeddingProvider()
        vector_search_repository = LocalVectorSearchRepository(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
            embedding_type=EmbeddingType.CODE,
        )

        # Create domain service
        domain_service = EmbeddingDomainService(
            embedding_provider=embedding_provider,
            vector_search_repository=vector_search_repository,
        )

        # Create actual snippets in the database first
        from datetime import UTC, datetime

        # Create source
        source = Source(
            uri="test_repo",
            cloned_path="/tmp/test_repo",  # noqa: S108
            source_type=SourceType.GIT,
        )
        session.add(source)
        await session.commit()

        # Create file
        file = File(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source_id=source.id,
            mime_type="text/plain",
            uri="test.py",
            cloned_path="/tmp/test_repo/test.py",  # noqa: S108
            sha256="abc123",
            size_bytes=100,
            extension="py",
            file_processing_status=FileProcessingStatus.CLEAN.value,
        )
        session.add(file)
        await session.commit()

        # Create index
        index = Index(source_id=source.id)
        session.add(index)
        await session.commit()

        # Create snippets
        snippet1 = Snippet(
            file_id=file.id,
            index_id=index.id,
            content="python programming language",
            summary="",
        )
        snippet2 = Snippet(
            file_id=file.id,
            index_id=index.id,
            content="javascript web development",
            summary="",
        )
        snippet3 = Snippet(
            file_id=file.id,
            index_id=index.id,
            content="java enterprise applications",
            summary="",
        )
        session.add(snippet1)
        session.add(snippet2)
        session.add(snippet3)
        await session.commit()

        # Test indexing with real snippet IDs
        index_request = IndexRequest(
            documents=[
                Document(snippet_id=snippet1.id, text="python programming language"),
                Document(snippet_id=snippet2.id, text="javascript web development"),
                Document(snippet_id=snippet3.id, text="java enterprise applications"),
            ]
        )

        index_results = []
        async for batch in domain_service.index_documents(index_request):
            index_results.extend(batch)

        assert len(index_results) == 3
        assert all(
            result.snippet_id in [snippet1.id, snippet2.id, snippet3.id]
            for result in index_results
        )

        # Test search
        search_request = SearchRequest(query="python programming language", top_k=2)

        search_results = await domain_service.search(search_request)

        assert len(search_results) == 2
        assert all(isinstance(r, SearchResult) for r in search_results)
        assert all(0 <= r.score <= 1 for r in search_results)

        # Test has_embedding
        has_embedding = await domain_service.has_embedding(
            snippet1.id, EmbeddingType.CODE
        )
        assert has_embedding is True

        has_embedding = await domain_service.has_embedding(999, EmbeddingType.CODE)
        assert has_embedding is False

    @pytest.mark.asyncio
    async def test_embedding_similarity_ranking(self, session: AsyncSession) -> None:
        """Test that embeddings produce meaningful similarity rankings."""
        embedding_repository = SqlAlchemyEmbeddingRepository(session=session)
        embedding_provider = HashEmbeddingProvider()
        vector_search_repository = LocalVectorSearchRepository(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
            embedding_type=EmbeddingType.CODE,
        )

        domain_service = EmbeddingDomainService(
            embedding_provider=embedding_provider,
            vector_search_repository=vector_search_repository,
        )

        # Create actual snippets in the database first
        from datetime import UTC, datetime

        # Create source
        source = Source(
            uri="test_repo",
            cloned_path="/tmp/test_repo",  # noqa: S108
            source_type=SourceType.GIT,
        )
        session.add(source)
        await session.commit()

        # Create file
        file = File(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source_id=source.id,
            mime_type="text/plain",
            uri="test.py",
            cloned_path="/tmp/test_repo/test.py",  # noqa: S108
            sha256="abc123",
            size_bytes=100,
            extension="py",
            file_processing_status=FileProcessingStatus.CLEAN.value,
        )
        session.add(file)
        await session.commit()

        # Create index
        index = Index(source_id=source.id)
        session.add(index)
        await session.commit()

        # Create snippets
        snippet1 = Snippet(
            file_id=file.id,
            index_id=index.id,
            content="python function definition",
            summary="",
        )
        snippet2 = Snippet(
            file_id=file.id,
            index_id=index.id,
            content="javascript function definition",
            summary="",
        )
        snippet3 = Snippet(
            file_id=file.id,
            index_id=index.id,
            content="java function definition",
            summary="",
        )
        snippet4 = Snippet(
            file_id=file.id,
            index_id=index.id,
            content="database query optimization",
            summary="",
        )
        session.add(snippet1)
        session.add(snippet2)
        session.add(snippet3)
        session.add(snippet4)
        await session.commit()

        # Index documents with different content
        index_request = IndexRequest(
            documents=[
                Document(snippet_id=snippet1.id, text="python function definition"),
                Document(snippet_id=snippet2.id, text="javascript function definition"),
                Document(snippet_id=snippet3.id, text="java function definition"),
                Document(snippet_id=snippet4.id, text="database query optimization"),
            ]
        )

        async for _ in domain_service.index_documents(index_request):
            pass

        # Search for python-related content
        search_request = SearchRequest(query="python function", top_k=3)

        results = await domain_service.search(search_request)

        assert len(results) == 3
        # Results should be ranked by similarity
        assert results[0].score >= results[1].score >= results[2].score

        # The python-related snippet should be among the top results
        python_snippet_ids = [
            r.snippet_id for r in results if r.snippet_id == snippet1.id
        ]
        assert len(python_snippet_ids) > 0

    @pytest.mark.asyncio
    async def test_embedding_batch_processing(self, session: AsyncSession) -> None:
        """Test that embedding processing works correctly in batches."""
        embedding_repository = SqlAlchemyEmbeddingRepository(session=session)
        embedding_provider = HashEmbeddingProvider()
        vector_search_repository = LocalVectorSearchRepository(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
            embedding_type=EmbeddingType.CODE,
        )

        domain_service = EmbeddingDomainService(
            embedding_provider=embedding_provider,
            vector_search_repository=vector_search_repository,
        )

        # Create actual snippets in the database first
        from datetime import UTC, datetime

        # Create source
        source = Source(
            uri="test_repo",
            cloned_path="/tmp/test_repo",  # noqa: S108
            source_type=SourceType.GIT,
        )
        session.add(source)
        await session.commit()

        # Create file
        file = File(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source_id=source.id,
            mime_type="text/plain",
            uri="test.py",
            cloned_path="/tmp/test_repo/test.py",  # noqa: S108
            sha256="abc123",
            size_bytes=100,
            extension="py",
            file_processing_status=FileProcessingStatus.CLEAN.value,
        )
        session.add(file)
        await session.commit()

        # Create index
        index = Index(source_id=source.id)
        session.add(index)
        await session.commit()

        # Create snippets
        snippets = []
        for i in range(25):
            snippet = Snippet(
                file_id=file.id,
                index_id=index.id,
                content=f"document {i} content",
                summary="",
            )
            snippets.append(snippet)
            session.add(snippet)
        await session.commit()

        # Create many documents to test batch processing
        documents = []
        for i, snippet in enumerate(snippets):
            documents.append(
                Document(snippet_id=snippet.id, text=f"document {i} content")
            )

        index_request = IndexRequest(documents=documents)

        batch_count = 0
        total_results = []
        async for batch in domain_service.index_documents(index_request):
            batch_count += 1
            total_results.extend(batch)

        assert len(total_results) == 25
        assert batch_count >= 2  # Should be processed in multiple batches

    @pytest.mark.asyncio
    async def test_embedding_error_handling(self, session: AsyncSession) -> None:
        """Test error handling in the embedding pipeline."""
        # Test with invalid requests
        embedding_repository = SqlAlchemyEmbeddingRepository(session=session)
        embedding_provider = HashEmbeddingProvider()
        vector_search_repository = LocalVectorSearchRepository(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
            embedding_type=EmbeddingType.CODE,
        )

        domain_service = EmbeddingDomainService(
            embedding_provider=embedding_provider,
            vector_search_repository=vector_search_repository,
        )

        # Test empty search query
        with pytest.raises(ValueError, match="Search query cannot be empty"):
            await domain_service.search(SearchRequest(query="", top_k=10))

        # Test invalid top_k
        with pytest.raises(ValueError, match="Top-k must be positive"):
            await domain_service.search(SearchRequest(query="test", top_k=0))

        # Test invalid snippet_id
        with pytest.raises(ValueError, match="Snippet ID must be positive"):
            await domain_service.has_embedding(-1, EmbeddingType.CODE)

    @pytest.mark.asyncio
    async def test_embedding_deterministic_behavior(
        self, session: AsyncSession
    ) -> None:
        """Test that embeddings are deterministic."""
        embedding_repository = SqlAlchemyEmbeddingRepository(session=session)
        embedding_provider = HashEmbeddingProvider()
        vector_search_repository = LocalVectorSearchRepository(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
            embedding_type=EmbeddingType.CODE,
        )

        domain_service = EmbeddingDomainService(
            embedding_provider=embedding_provider,
            vector_search_repository=vector_search_repository,
        )

        # Create actual snippets in the database first
        from datetime import UTC, datetime

        # Create source
        source = Source(
            uri="test_repo",
            cloned_path="/tmp/test_repo",  # noqa: S108
            source_type=SourceType.GIT,
        )
        session.add(source)
        await session.commit()

        # Create file
        file = File(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source_id=source.id,
            mime_type="text/plain",
            uri="test.py",
            cloned_path="/tmp/test_repo/test.py",  # noqa: S108
            sha256="abc123",
            size_bytes=100,
            extension="py",
            file_processing_status=FileProcessingStatus.CLEAN.value,
        )
        session.add(file)
        await session.commit()

        # Create index
        index = Index(source_id=source.id)
        session.add(index)
        await session.commit()

        # Create snippet
        snippet = Snippet(
            file_id=file.id, index_id=index.id, content="python programming", summary=""
        )
        session.add(snippet)
        await session.commit()

        # Index the same content twice
        index_request = IndexRequest(
            documents=[
                Document(snippet_id=snippet.id, text="python programming"),
            ]
        )

        # First indexing
        async for _ in domain_service.index_documents(index_request):
            pass

        # Search first time
        search_request = SearchRequest(query="python programming", top_k=1)
        results1 = await domain_service.search(search_request)

        # Second indexing (should be idempotent)
        async for _ in domain_service.index_documents(index_request):
            pass

        # Search second time
        results2 = await domain_service.search(search_request)

        # Results should be consistent
        assert len(results1) == len(results2)
        if results1 and results2:
            assert results1[0].snippet_id == results2[0].snippet_id
            assert abs(results1[0].score - results2[0].score) < 1e-6

    @pytest.mark.asyncio
    async def test_embedding_type_separation(self, session: AsyncSession) -> None:
        """Test that different embedding types are handled separately."""
        embedding_repository = SqlAlchemyEmbeddingRepository(session=session)
        embedding_provider = HashEmbeddingProvider()

        # Create two repositories with different embedding types
        code_repository = LocalVectorSearchRepository(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
            embedding_type=EmbeddingType.CODE,
        )

        text_repository = LocalVectorSearchRepository(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
            embedding_type=EmbeddingType.TEXT,
        )

        code_service = EmbeddingDomainService(
            embedding_provider=embedding_provider,
            vector_search_repository=code_repository,
        )

        text_service = EmbeddingDomainService(
            embedding_provider=embedding_provider,
            vector_search_repository=text_repository,
        )

        # Create actual snippets in the database first
        from datetime import UTC, datetime

        # Create source
        source = Source(
            uri="test_repo",
            cloned_path="/tmp/test_repo",  # noqa: S108
            source_type=SourceType.GIT,
        )
        session.add(source)
        await session.commit()

        # Create file
        file = File(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source_id=source.id,
            mime_type="text/plain",
            uri="test.py",
            cloned_path="/tmp/test_repo/test.py",  # noqa: S108
            sha256="abc123",
            size_bytes=100,
            extension="py",
            file_processing_status=FileProcessingStatus.CLEAN.value,
        )
        session.add(file)
        await session.commit()

        # Create index
        index = Index(source_id=source.id)
        session.add(index)
        await session.commit()

        # Create snippet
        snippet = Snippet(
            file_id=file.id, index_id=index.id, content="python programming", summary=""
        )
        session.add(snippet)
        await session.commit()

        # Index same content with different types
        index_request = IndexRequest(
            documents=[
                Document(snippet_id=snippet.id, text="python programming"),
            ]
        )

        # Index in code repository
        async for _ in code_service.index_documents(index_request):
            pass

        # Index in text repository
        async for _ in text_service.index_documents(index_request):
            pass

        # Check that embeddings exist for both types
        assert await code_service.has_embedding(snippet.id, EmbeddingType.CODE) is True
        assert await text_service.has_embedding(snippet.id, EmbeddingType.TEXT) is True

        # Search should work for both types
        search_request = SearchRequest(query="python programming", top_k=1)

        code_results = await code_service.search(search_request)
        text_results = await text_service.search(search_request)

        assert len(code_results) == 1
        assert len(text_results) == 1
        assert code_results[0].snippet_id == snippet.id
        assert text_results[0].snippet_id == snippet.id

    @pytest.mark.asyncio
    async def test_embedding_performance_characteristics(
        self, session: AsyncSession
    ) -> None:
        """Test basic performance characteristics of embedding operations."""
        embedding_repository = SqlAlchemyEmbeddingRepository(session=session)
        embedding_provider = HashEmbeddingProvider()
        vector_search_repository = LocalVectorSearchRepository(
            embedding_repository=embedding_repository,
            embedding_provider=embedding_provider,
            embedding_type=EmbeddingType.CODE,
        )

        domain_service = EmbeddingDomainService(
            embedding_provider=embedding_provider,
            vector_search_repository=vector_search_repository,
        )

        # Create actual snippets in the database first
        from datetime import UTC, datetime

        # Create source
        source = Source(
            uri="test_repo",
            cloned_path="/tmp/test_repo",  # noqa: S108
            source_type=SourceType.GIT,
        )
        session.add(source)
        await session.commit()

        # Create file
        file = File(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source_id=source.id,
            mime_type="text/plain",
            uri="test.py",
            cloned_path="/tmp/test_repo/test.py",  # noqa: S108
            sha256="abc123",
            size_bytes=100,
            extension="py",
            file_processing_status=FileProcessingStatus.CLEAN.value,
        )
        session.add(file)
        await session.commit()

        # Create index
        index = Index(source_id=source.id)
        session.add(index)
        await session.commit()

        # Create snippets
        snippets = []
        for i in range(10):
            snippet = Snippet(
                file_id=file.id,
                index_id=index.id,
                content=f"document {i} with some content",
                summary="",
            )
            snippets.append(snippet)
            session.add(snippet)
        await session.commit()

        # Test indexing performance with multiple documents
        import time

        documents = []
        for i, snippet in enumerate(snippets):
            documents.append(
                Document(snippet_id=snippet.id, text=f"document {i} with some content")
            )

        index_request = IndexRequest(documents=documents)

        start_time = time.time()
        async for _ in domain_service.index_documents(index_request):
            pass
        indexing_time = time.time() - start_time

        # Indexing should complete in reasonable time
        assert indexing_time < 10.0  # Should complete within 10 seconds

        # Test search performance
        search_request = SearchRequest(query="document content", top_k=5)

        start_time = time.time()
        results = await domain_service.search(search_request)
        search_time = time.time() - start_time

        # Search should complete quickly
        assert search_time < 5.0  # Should complete within 5 seconds
        assert len(results) == 5
