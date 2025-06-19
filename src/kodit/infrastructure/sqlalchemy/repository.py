"""SQLAlchemy repository."""

from collections.abc import Sequence

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Author, AuthorFileMapping, File, Source, SourceType
from kodit.domain.repositories import AuthorRepository, SourceRepository


class SqlAlchemySourceRepository(SourceRepository):
    """SQLAlchemy source repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the repository."""
        self._session = session

    async def get(self, source_id: int) -> Source | None:
        """Get a source by ID."""
        return await self._session.get(Source, source_id)

    async def get_by_uri(self, uri: str) -> Source | None:
        """Get a source by URI."""
        stmt = select(Source).where(Source.uri == uri)
        return await self._session.scalar(stmt)  # None if no row

    async def list(self, *, source_type: SourceType | None = None) -> Sequence[Source]:
        """List sources."""
        stmt = select(Source)
        if source_type is not None:
            stmt = stmt.where(Source.type == source_type)
        return (await self._session.scalars(stmt)).all()

    async def add(self, source: Source) -> None:
        """Add a source."""
        self._session.add(source)  # INSERT on flush
        await self._session.flush()  # Flush to get the ID

    async def create_source(self, source: Source) -> Source:
        """Create a source and commit it."""
        self._session.add(source)
        await self._session.commit()
        return source

    async def remove(self, source: Source) -> None:
        """Remove a source."""
        await self._session.delete(source)  # DELETE on flush

    async def create_file(self, file: File) -> File:
        """Create a new file record."""
        self._session.add(file)
        await self._session.commit()
        return file

    async def upsert_author(self, author: Author) -> Author:
        """Create a new author or return existing one if email already exists."""
        # First check if author already exists with same name and email
        stmt = select(Author).where(
            Author.name == author.name, Author.email == author.email
        )
        existing_author = await self._session.scalar(stmt)

        if existing_author:
            return existing_author

        # Author doesn't exist, create new one
        self._session.add(author)
        await self._session.commit()
        return author

    async def upsert_author_file_mapping(
        self, mapping: AuthorFileMapping
    ) -> AuthorFileMapping:
        """Create a new author file mapping or return existing one if already exists."""
        # First check if mapping already exists with same author_id and file_id
        stmt = select(AuthorFileMapping).where(
            AuthorFileMapping.author_id == mapping.author_id,
            AuthorFileMapping.file_id == mapping.file_id,
        )
        existing_mapping = await self._session.scalar(stmt)

        if existing_mapping:
            return existing_mapping

        # Mapping doesn't exist, create new one
        self._session.add(mapping)
        await self._session.commit()
        return mapping


class SqlAlchemyAuthorRepository(AuthorRepository):
    """SQLAlchemy author repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the repository."""
        self._session = session

    async def get(self, author_id: int) -> Author | None:
        """Get an author by ID."""
        return await self._session.get(Author, author_id)

    async def get_by_name(self, name: str) -> Author | None:
        """Get an author by name."""
        return await self._session.scalar(select(Author).where(Author.name == name))

    async def get_by_email(self, email: str) -> Author | None:
        """Get an author by email."""
        return await self._session.scalar(select(Author).where(Author.email == email))

    async def list(self) -> Sequence[Author]:
        """List authors."""
        return (await self._session.scalars(select(Author))).all()

    async def add(self, author: Author) -> None:
        """Add an author."""
        self._session.add(author)

    async def remove(self, author: Author) -> None:
        """Remove an author."""
        await self._session.delete(author)
