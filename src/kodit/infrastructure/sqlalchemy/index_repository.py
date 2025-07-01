"""SQLAlchemy implementation of IndexRepository using Index aggregate root."""


from pydantic import AnyUrl
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain import entities as db_entities
from kodit.domain.models import entities as domain_entities
from kodit.domain.models.protocols import IndexRepository
from kodit.infrastructure.mappers.index_mapper import IndexMapper


class SqlAlchemyIndexRepository(IndexRepository):
    """SQLAlchemy implementation of IndexRepository.
    
    This repository manages the complete Index aggregate, including:
    - Index entity
    - Source entity and WorkingCopy value object
    - File entities and Author relationships
    - Snippet entities with their contents
    """

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the repository."""
        self._session = session
        self._mapper = IndexMapper(session)

    async def create(self, uri: AnyUrl) -> domain_entities.Index:
        """Create an index for a source.
        
        This creates a minimal Index with Source but no files or snippets yet.
        """
        # Check if source already exists
        existing_source = await self._get_source_by_uri(uri)
        if existing_source:
            # Check if index already exists for this source
            existing_index = await self._get_index_by_source_id(existing_source.id)
            if existing_index:
                return await self._mapper.to_domain_index(existing_index)

        # Create new source
        from datetime import UTC, datetime
        from pathlib import Path

        from kodit.domain.models.value_objects import SourceType

        now = datetime.now(UTC)

        # Create minimal working copy - will be populated later
        working_copy = domain_entities.WorkingCopy(
            created_at=now,
            updated_at=now,
            remote_uri=uri,
            cloned_path=Path("/tmp"),  # Temporary, will be updated when cloning
            source_type=SourceType.UNKNOWN,  # Will be determined during cloning
            files=[]
        )

        # Create source
        domain_source = domain_entities.Source(
            id=0,  # Will be assigned by database
            created_at=now,
            updated_at=now,
            working_copy=working_copy
        )

        # Create index
        domain_index = domain_entities.Index(
            id=0,  # Will be assigned by database
            created_at=now,
            updated_at=now,
            source=domain_source
        )

        # Convert to database entities and save
        db_index, db_source, db_files, db_authors = await self._mapper.from_domain_index(domain_index)

        # Save source first
        self._session.add(db_source)
        await self._session.flush()  # Get source ID

        # Update index with source ID
        db_index.source_id = db_source.id
        self._session.add(db_index)
        await self._session.flush()  # Get index ID

        # Return the created index as domain entity
        return await self._mapper.to_domain_index(db_index)

    async def get(self, id: int) -> domain_entities.Index | None:
        """Get an index by ID."""
        db_index = await self._session.get(db_entities.Index, id)
        if not db_index:
            return None

        return await self._mapper.to_domain_index(db_index)

    async def get_by_uri(self, uri: AnyUrl) -> domain_entities.Index | None:
        """Get an index by source URI."""
        db_source = await self._get_source_by_uri(uri)
        if not db_source:
            return None

        db_index = await self._get_index_by_source_id(db_source.id)
        if not db_index:
            return None

        return await self._mapper.to_domain_index(db_index)

    async def set_working_copy(
        self, index_id: int, working_copy: domain_entities.WorkingCopy
    ) -> None:
        """Set the working copy for an index.

        This updates the source's cloned_path and source_type, and manages files.
        """
        # Get the index and source
        db_index = await self._session.get(db_entities.Index, index_id)
        if not db_index:
            raise ValueError(f"Index with ID {index_id} not found")

        db_source = await self._session.get(db_entities.Source, db_index.source_id)
        if not db_source:
            raise ValueError(f"Source for index {index_id} not found")

        # Update source with working copy information
        db_source.cloned_path = str(working_copy.cloned_path)
        db_source.type = db_entities.SourceType(working_copy.source_type.value)
        db_source.updated_at = working_copy.updated_at

        # Clear existing files for this source
        files_stmt = select(db_entities.File).where(
            db_entities.File.source_id == db_source.id
        )
        existing_files = (await self._session.scalars(files_stmt)).all()
        for file in existing_files:
            await self._session.delete(file)

        # Add new files from working copy
        await self.add_files(index_id, working_copy.files)

    async def add_files(self, index_id: int, files: list[domain_entities.File]) -> None:
        """Add files to an index."""
        # Get the index to verify it exists and get source_id
        db_index = await self._session.get(db_entities.Index, index_id)
        if not db_index:
            raise ValueError(f"Index with ID {index_id} not found")

        # Convert and save files
        for domain_file in files:
            # Create file entity
            db_file = db_entities.File(
                created_at=domain_file.created_at,
                updated_at=domain_file.updated_at,
                source_id=db_index.source_id,
                mime_type="",  # Would need to be determined
                uri=str(domain_file.uri),
                cloned_path="",  # Derived from working copy + relative path
                sha256=domain_file.sha256,
                size_bytes=0,  # Would need to be calculated
                extension=""   # Would need to be extracted from URI
            )

            self._session.add(db_file)
            await self._session.flush()  # Get file ID

            # Handle authors
            for domain_author in domain_file.authors:
                # Find or create author
                db_author = await self._find_or_create_author(domain_author)

                # Create author-file mapping
                mapping = db_entities.AuthorFileMapping(
                    author_id=db_author.id,
                    file_id=db_file.id
                )
                self._session.add(mapping)

    async def add_snippets(
        self, index_id: int, snippets: list[domain_entities.Snippet]
    ) -> None:
        """Add snippets to an index."""
        # Get the index to verify it exists
        db_index = await self._session.get(db_entities.Index, index_id)
        if not db_index:
            raise ValueError(f"Index with ID {index_id} not found")

        # For now, we'll associate snippets with the first file in the index
        # In a real implementation, determine which file each snippet belongs to
        files_stmt = select(db_entities.File).where(
            db_entities.File.source_id == db_index.source_id
        )
        db_files = (await self._session.scalars(files_stmt)).all()

        if not db_files:
            raise ValueError(f"No files found for index {index_id}")

        file_id = db_files[0].id  # Use first file for now

        # Convert and save snippets
        for domain_snippet in snippets:
            db_snippet = await self._mapper.from_domain_snippet(
                domain_snippet, file_id, index_id
            )
            self._session.add(db_snippet)

    async def _get_source_by_uri(self, uri: AnyUrl) -> db_entities.Source | None:
        """Get source by URI."""
        stmt = select(db_entities.Source).where(db_entities.Source.uri == str(uri))
        result = await self._session.scalar(stmt)
        return result

    async def _get_index_by_source_id(self, source_id: int) -> db_entities.Index | None:
        """Get index by source ID."""
        stmt = select(db_entities.Index).where(db_entities.Index.source_id == source_id)
        result = await self._session.scalar(stmt)
        return result

    async def _find_or_create_author(
        self, domain_author: domain_entities.Author
    ) -> db_entities.Author:
        """Find existing author or create new one."""
        # Try to find existing author
        stmt = select(db_entities.Author).where(
            db_entities.Author.name == domain_author.name,
            db_entities.Author.email == domain_author.email
        )
        db_author = await self._session.scalar(stmt)

        if db_author:
            return db_author

        # Create new author
        db_author = db_entities.Author(
            name=domain_author.name, email=domain_author.email
        )
        self._session.add(db_author)
        await self._session.flush()  # Get ID

        return db_author

