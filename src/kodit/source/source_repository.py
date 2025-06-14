"""Source repository for database operations."""

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.source.source_models import (
    Author,
    AuthorFileMapping,
    File,
    Source,
    SourceType,
)


class SourceRepository:
    """Repository for managing source database operations.

    This class provides methods for creating and retrieving source records from the
    database. It handles the low-level database operations and transaction management.

    Args:
        session: The SQLAlchemy async session to use for database operations.

    """

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the source repository."""
        self.session = session

    async def create_source(self, source: Source) -> Source:
        """Add a new source to the database."""
        # Validate the source
        if source.type == SourceType.UNKNOWN:
            msg = "Source type is required"
            raise ValueError(msg)

        self.session.add(source)
        await self.session.commit()
        return source

    async def create_file(self, file: File) -> File:
        """Create a new file record in the database.

        This method creates a new File record and adds it to the session.

        """
        self.session.add(file)
        await self.session.commit()
        return file

    async def list_files_for_source(self, source_id: int) -> list[File]:
        """List all files for a source."""
        query = select(File).where(File.source_id == source_id)
        result = await self.session.execute(query)
        return list(result.scalars())

    async def num_files_for_source(self, source_id: int) -> int:
        """Get the number of files for a source.

        Args:
            source_id: The ID of the source to get the number of files for.

        Returns:
            The number of files for the source.

        """
        query = (
            select(func.count()).select_from(File).where(File.source_id == source_id)
        )
        result = await self.session.execute(query)
        return result.scalar_one()

    async def list_sources(self) -> list[Source]:
        """Retrieve all sources from the database.

        Returns:
            A list of Source instances.

        """
        query = select(Source).limit(10)
        result = await self.session.execute(query)
        return list(result.scalars())

    async def get_source_by_uri(self, uri: str) -> Source | None:
        """Get a source by its URI.

        Args:
            uri: The URI of the source to get.

        Returns:
            The source with the given URI, or None if it does not exist.

        """
        query = select(Source).where(Source.uri == uri)
        result = await self.session.execute(query)
        return result.scalar_one_or_none()

    async def get_source_by_id(self, source_id: int) -> Source | None:
        """Get a source by its ID.

        Args:
            source_id: The ID of the source to get.

        """
        query = select(Source).where(Source.id == source_id)
        result = await self.session.execute(query)
        return result.scalar_one_or_none()

    async def get_author_by_email(self, email: str) -> Author | None:
        """Get an author by email."""
        query = select(Author).where(Author.email == email)
        result = await self.session.execute(query)
        return result.scalar_one_or_none()

    async def upsert_author(self, author: Author) -> Author:
        """Create a new author or return existing one if email already exists.

        Args:
            author: The Author instance to upsert.

        Returns:
            The existing Author if one with the same email exists, otherwise the newly
            created Author.

        """
        # First check if author already exists with same name and email
        query = select(Author).where(
            Author.name == author.name, Author.email == author.email
        )
        result = await self.session.execute(query)
        existing_author = result.scalar_one_or_none()

        if existing_author:
            return existing_author

        # Author doesn't exist, create new one
        self.session.add(author)
        await self.session.commit()
        return author

    async def upsert_author_file_mapping(
        self, mapping: AuthorFileMapping
    ) -> AuthorFileMapping:
        """Create a new author file mapping or return existing one if already exists."""
        # First check if mapping already exists with same author_id and file_id
        query = select(AuthorFileMapping).where(
            AuthorFileMapping.author_id == mapping.author_id,
            AuthorFileMapping.file_id == mapping.file_id,
        )
        result = await self.session.execute(query)
        existing_mapping = result.scalar_one_or_none()

        if existing_mapping:
            return existing_mapping

        # Mapping doesn't exist, create new one
        self.session.add(mapping)
        await self.session.commit()
        return mapping

    async def list_files_for_author(self, author_id: int) -> list[File]:
        """List all files for an author."""
        query = (
            select(File)
            .join(AuthorFileMapping)
            .where(AuthorFileMapping.author_id == author_id)
        )
        result = await self.session.execute(query)
        return list(result.scalars())
