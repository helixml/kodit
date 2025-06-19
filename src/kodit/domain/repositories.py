"""Domain repositories with generic patterns."""

from collections.abc import Sequence
from typing import Protocol, TypeVar

from kodit.domain.entities import (
    Author,
    AuthorFileMapping,
    File,
    Snippet,
    Source,
    SourceType,
)

T = TypeVar("T")


class GenericRepository(Protocol[T]):
    """Generic repository interface."""

    async def get(self, id: int) -> T | None:  # noqa: A002
        """Get entity by ID."""
        ...

    async def save(self, entity: T) -> T:
        """Save entity."""
        ...

    async def delete(self, id: int) -> None:  # noqa: A002
        """Delete entity by ID."""
        ...

    async def list(self) -> Sequence[T]:
        """List all entities."""
        ...


class SourceRepository(GenericRepository[Source]):
    """Source repository with specific methods."""

    async def get_by_uri(self, uri: str) -> Source | None:
        """Get a source by URI."""
        raise NotImplementedError

    async def list_by_type(
        self, source_type: SourceType | None = None
    ) -> Sequence[Source]:
        """List sources by type."""
        raise NotImplementedError

    async def create_file(self, file: File) -> File:
        """Create a new file record."""
        raise NotImplementedError

    async def upsert_author(self, author: Author) -> Author:
        """Create a new author or return existing one if email already exists."""
        raise NotImplementedError

    async def upsert_author_file_mapping(
        self, mapping: "AuthorFileMapping"
    ) -> "AuthorFileMapping":
        """Create a new author file mapping or return existing one if already exists."""
        raise NotImplementedError


class AuthorRepository(GenericRepository[Author]):
    """Author repository with specific methods."""

    async def get_by_name(self, name: str) -> Author | None:
        """Get an author by name."""
        raise NotImplementedError

    async def get_by_email(self, email: str) -> Author | None:
        """Get an author by email."""
        raise NotImplementedError


class SnippetRepository(GenericRepository[Snippet]):
    """Snippet repository with specific methods."""

    async def get_by_index(self, index_id: int) -> Sequence[Snippet]:
        """Get all snippets for an index."""
        raise NotImplementedError

    async def delete_by_index(self, index_id: int) -> None:
        """Delete all snippets for an index."""
        raise NotImplementedError


class FileRepository(GenericRepository[File]):
    """File repository with specific methods."""

    async def get_files_for_index(self, index_id: int) -> Sequence[File]:
        """Get all files for an index."""
        raise NotImplementedError
