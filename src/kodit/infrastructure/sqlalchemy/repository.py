"""SQLAlchemy repository."""

from collections.abc import Sequence
from typing import cast

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Author, AuthorFileMapping, File, Source, SourceType
from kodit.domain.repositories import AuthorRepository, SourceRepository


class SqlAlchemySourceRepository(SourceRepository):
    """SQLAlchemy source repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the repository."""
        self._session = session

    async def get(self, id: int) -> Source | None:  # noqa: A002
        """Get a source by ID."""
        return await self._session.get(Source, id)

    async def save(self, entity: Source) -> Source:
        """Save entity."""
        self._session.add(entity)
        return entity

    async def delete(self, id: int) -> None:  # noqa: A002
        """Delete entity by ID."""
        source = await self.get(id)
        if source:
            await self._session.delete(source)

    async def list(self) -> Sequence[Source]:
        """List all entities."""
        stmt = select(Source)
        return (await self._session.scalars(stmt)).all()

    async def get_by_uri(self, uri: str) -> Source | None:
        """Get a source by URI."""
        stmt = select(Source).where(Source.uri == uri)
        return cast("Source | None", await self._session.scalar(stmt))

    async def list_by_type(
        self, source_type: SourceType | None = None
    ) -> Sequence[Source]:
        """List sources by type."""
        stmt = select(Source)
        if source_type is not None:
            stmt = stmt.where(Source.type == source_type)
        return (await self._session.scalars(stmt)).all()

    async def create_file(self, file: File) -> File:
        """Create a new file record."""
        self._session.add(file)
        return file

    async def upsert_author(self, author: Author) -> Author:
        """Create a new author or return existing one if email already exists."""
        # First check if author already exists with same name and email
        stmt = select(Author).where(
            Author.name == author.name, Author.email == author.email
        )
        existing_author = cast("Author | None", await self._session.scalar(stmt))

        if existing_author:
            return existing_author

        # Author doesn't exist, create new one
        self._session.add(author)
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
        existing_mapping = cast(
            "AuthorFileMapping | None", await self._session.scalar(stmt)
        )

        if existing_mapping:
            return existing_mapping

        # Mapping doesn't exist, create new one
        self._session.add(mapping)
        return mapping


class SqlAlchemyAuthorRepository(AuthorRepository):
    """SQLAlchemy author repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the repository."""
        self._session = session

    async def get(self, id: int) -> Author | None:  # noqa: A002
        """Get an author by ID."""
        return await self._session.get(Author, id)

    async def save(self, entity: Author) -> Author:
        """Save entity."""
        self._session.add(entity)
        return entity

    async def delete(self, id: int) -> None:  # noqa: A002
        """Delete entity by ID."""
        author = await self.get(id)
        if author:
            await self._session.delete(author)

    async def list(self) -> Sequence[Author]:
        """List authors."""
        return (await self._session.scalars(select(Author))).all()

    async def get_by_name(self, name: str) -> Author | None:
        """Get an author by name."""
        return cast(
            "Author | None",
            await self._session.scalar(select(Author).where(Author.name == name)),
        )

    async def get_by_email(self, email: str) -> Author | None:
        """Get an author by email."""
        return cast(
            "Author | None",
            await self._session.scalar(select(Author).where(Author.email == email)),
        )
