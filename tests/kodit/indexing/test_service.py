# """Tests for the indexing service module."""

# from datetime import UTC, datetime
# from pathlib import Path

# import pytest
# from sqlalchemy.ext.asyncio import AsyncSession

# from kodit.indexing.models import File, Snippet
# from kodit.indexing.models import Index as IndexModel
# from kodit.indexing.repository import IndexRepository
# from kodit.indexing.service import IndexService
# from kodit.sources.models import Source


# @pytest.fixture
# def repository(session: AsyncSession) -> IndexRepository:
#     """Create a real repository instance with a database session."""
#     return IndexRepository(session)


# @pytest.fixture
# def service(repository: IndexRepository) -> IndexService:
#     """Create a service instance with a real repository."""
#     return IndexService(repository)


# @pytest.mark.asyncio
# async def test_create_index(
#     service: IndexService, repository: IndexRepository, session: AsyncSession
# ) -> None:
#     """Test creating a new index through the service."""
#     # Create a test source
#     source = Source(created_at=datetime(2024, 1, 1, 0, 0, tzinfo=UTC))
#     session.add(source)
#     await session.commit()

#     # Create folder source
#     folder_source = FolderSource(source_id=source.id, path="test_folder")
#     session.add(folder_source)
#     await session.commit()

#     index = await service.create(source.id)

#     assert index.id is not None
#     assert index.created_at is not None

#     # Verify the index was created in the database
#     db_index = await repository.get_by_id(index.id)
#     assert db_index is not None
#     assert db_index.source_id == source.id


# @pytest.mark.asyncio
# async def test_create_index_source_not_found(service: IndexService):
#     """Test creating an index for a non-existent source."""
#     with pytest.raises(ValueError, match="Source not found"):
#         await service.create(999)


# @pytest.mark.asyncio
# async def test_create_index_already_exists(
#     service: IndexService, session: AsyncSession
# ):
#     """Test creating an index that already exists."""
#     # Create a test source
#     source = Source(created_at=datetime(2024, 1, 1, 0, 0, tzinfo=UTC))
#     session.add(source)
#     await session.commit()

#     # Create folder source
#     folder_source = FolderSource(source_id=source.id, path="test_folder")
#     session.add(folder_source)
#     await session.commit()

#     # Create first index
#     await service.create(source.id)

#     # Try to create second index
#     with pytest.raises(ValueError, match="Index already exists"):
#         await service.create(source.id)


# @pytest.mark.asyncio
# async def test_list_indexes(
#     service: IndexService, repository: IndexRepository, session: AsyncSession
# ):
#     """Test listing all indexes through the service."""
#     # Create test data
#     source = Source(created_at=datetime(2024, 1, 1, 0, 0, tzinfo=UTC))
#     session.add(source)
#     await session.commit()

#     folder_source = FolderSource(source_id=source.id, path="test_folder")
#     session.add(folder_source)
#     await session.commit()

#     # Create index
#     index = await service.create(source.id)

#     # Create some files and snippets
#     file1 = File(
#         index_id=index.id,
#         source_id=source.id,
#         mime_type="text/plain",
#         path="test1.txt",
#         sha256="hash1",
#         size_bytes=100,
#     )
#     file2 = File(
#         index_id=index.id,
#         source_id=source.id,
#         mime_type="text/plain",
#         path="test2.txt",
#         sha256="hash2",
#         size_bytes=200,
#     )
#     await repository.add_file(file1)
#     await repository.add_file(file2)

#     snippet1 = Snippet(index_id=index.id, file_id=file1.id, content=b"test1")
#     snippet2 = Snippet(index_id=index.id, file_id=file2.id, content=b"test2")
#     await repository.add_snippet(snippet1)
#     await repository.add_snippet(snippet2)

#     indexes = await service.list_indexes()

#     assert len(indexes) == 1
#     assert indexes[0].id == index.id
#     assert indexes[0].source_uri == "test_folder"
#     assert indexes[0].num_files == 2
#     assert indexes[0].num_snippets == 2


# @pytest.mark.asyncio
# async def test_run_index(
#     service: IndexService,
#     repository: IndexRepository,
#     session: AsyncSession,
#     tmp_path: Path,
# ):
#     """Test running an index through the service."""
#     # Create test files
#     test_dir = tmp_path / "test_folder"
#     test_dir.mkdir()
#     test_file = test_dir / "test.py"
#     test_file.write_text("print('hello')")

#     # Create test source
#     source = Source(created_at=datetime(2024, 1, 1, 0, 0, tzinfo=UTC))
#     session.add(source)
#     await session.commit()

#     # Create folder source
#     folder_source = FolderSource(source_id=source.id, path=str(test_dir))
#     session.add(folder_source)
#     await session.commit()

#     # Create index
#     index = await service.create(source.id)

#     # Run the index
#     await service.run(index.id, str(test_dir))

#     # Verify files were indexed
#     files = await repository.get_files_by_source(source.id)
#     assert len(files) == 1
#     assert files[0].path == str(test_file)

#     # Verify snippets were created
#     snippets = await repository.get_existing_snippets(index.id)
#     assert len(snippets) == 1


# @pytest.mark.asyncio
# async def test_run_index_source_not_found(
#     service: IndexService, session: AsyncSession
# ) -> None:
#     """Test running an index with a non-existent source."""
#     # Create an index with a non-existent source
#     index = IndexModel(source_id=999, created_at=datetime(2024, 1, 1, 0, 0, tzinfo=UTC))
#     session.add(index)
#     await session.commit()

#     with pytest.raises(ValueError, match="Source not found"):
#         await service.run(index.id)
