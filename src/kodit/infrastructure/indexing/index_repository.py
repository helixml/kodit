"""Infrastructure implementation of the index repository."""

from datetime import UTC, datetime

from pydantic import AnyUrl
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import (
    AuthorFileMapping,
    Index,
    Source,
)
from kodit.domain.models.entities import Index as DomainIndex
from kodit.domain.models.entities import WorkingCopy
from kodit.domain.models.protocols import IndexRepository
from kodit.infrastructure.mappers.index_mapper import IndexMapper


class SQLAlchemyIndexRepository(IndexRepository):
    """SQLAlchemy implementation of the index repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the index repository.

        Args:
            session: The SQLAlchemy async session to use for database operations.

        """
        self.session = session
        self.mapper = IndexMapper(session)

    async def create(self, uri: AnyUrl, working_copy: WorkingCopy) -> DomainIndex:
        """Create an index for a source.

        Args:
            uri: The URI of the source
            working_copy: The working copy containing files and authors

        Returns:
            The created domain index.

        """
        # Check if index already exists
        existing_index = await self.get_by_uri(uri)
        if existing_index:
            return existing_index

        # Create domain index with working copy
        from kodit.domain.models.entities import Source as DomainSource

        domain_source = DomainSource(working_copy=working_copy)
        domain_index = DomainIndex(source=domain_source)

        # Convert to database entities and save in hierarchical order
        db_index, db_source, db_files, db_authors = await self.mapper.from_domain_index(
            domain_index
        )

        # Save authors first (no dependencies)
        for db_author in db_authors:
            self.session.add(db_author)
        await self.session.flush()  # Get author IDs

        # Save source (no dependencies)
        self.session.add(db_source)
        await self.session.flush()  # Get source ID

        # Update file source_ids and save files
        for db_file in db_files:
            db_file.source_id = db_source.id
            self.session.add(db_file)
        await self.session.flush()  # Get file IDs

        # Create author-file mappings
        for domain_file in working_copy.files:
            db_file = next(f for f in db_files if f.uri == str(domain_file.uri))
            for domain_author in domain_file.authors:
                db_author = next(
                    a
                    for a in db_authors
                    if a.name == domain_author.name and a.email == domain_author.email
                )
                mapping = AuthorFileMapping(author_id=db_author.id, file_id=db_file.id)
                self.session.add(mapping)

        # Save index (depends on source)
        db_index.source_id = db_source.id
        self.session.add(db_index)
        await self.session.flush()  # Get index ID

        # Update domain objects with generated IDs
        domain_index.id = db_index.id
        domain_source.id = db_source.id
        for i, domain_file in enumerate(working_copy.files):
            domain_file.id = db_files[i].id
        # Create unique authors list (Authors are not hashable, so use dict)
        unique_authors_dict = {}
        for file in working_copy.files:
            for author in file.authors:
                key = (author.name, author.email)
                if key not in unique_authors_dict:
                    unique_authors_dict[key] = author

        for i, domain_author in enumerate(unique_authors_dict.values()):
            domain_author.id = db_authors[i].id

        return domain_index

    async def get(self, id: int) -> DomainIndex | None:  # noqa: A002
        """Get an index by its ID.

        Args:
            id: The ID of the index to retrieve.

        Returns:
            The domain index if found, None otherwise.

        """
        db_index = await self.session.get(Index, id)
        if not db_index:
            return None

        return await self.mapper.to_domain_index(db_index)

    async def get_by_uri(self, uri: AnyUrl) -> DomainIndex | None:
        """Get an index by source URI.

        Args:
            uri: The URI of the source to retrieve an index for.

        Returns:
            The domain index if found, None otherwise.

        """
        query = (
            select(Index)
            .join(Source, Index.source_id == Source.id)
            .where(Source.uri == str(uri))
        )
        result = await self.session.execute(query)
        db_index = result.scalar_one_or_none()

        if not db_index:
            return None

        return await self.mapper.to_domain_index(db_index)

    async def list(self) -> list[DomainIndex]:
        """List all indexes.

        Returns:
            A list of domain indexes.

        """
        query = select(Index)
        result = await self.session.execute(query)
        db_indexes = result.scalars().all()

        domain_indexes = []
        for db_index in db_indexes:
            domain_index = await self.mapper.to_domain_index(db_index)
            domain_indexes.append(domain_index)

        return domain_indexes

    async def update_index_timestamp(self, index_id: int) -> None:
        """Update the timestamp of an index.

        Args:
            index_id: The ID of the index to update.

        """
        query = select(Index).where(Index.id == index_id)
        result = await self.session.execute(query)
        index = result.scalar_one_or_none()

        if index:
            index.updated_at = datetime.now(UTC)
