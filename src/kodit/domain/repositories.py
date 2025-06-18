"""Source repository."""

from collections.abc import Sequence
from typing import Protocol

from kodit.domain.models import Author, AuthorFileMapping, File, Source, SourceType


class SourceRepository(Protocol):
    """Source repository."""

    async def get(self, source_id: int) -> Source | None:
        """Get a source by ID."""
        ...

    async def get_by_uri(self, uri: str) -> Source | None:
        """Get a source by URI."""
        ...

    async def list(self, *, source_type: SourceType | None = None) -> Sequence[Source]:
        """List sources."""
        ...

    async def add(self, source: Source) -> None:
        """Add a source."""
        ...

    async def remove(self, source: Source) -> None:
        """Remove a source."""
        ...

    async def create_file(self, file: File) -> File:
        """Create a new file record."""
        ...

    async def upsert_author(self, author: Author) -> Author:
        """Create a new author or return existing one if email already exists."""
        ...

    async def upsert_author_file_mapping(
        self, mapping: AuthorFileMapping
    ) -> AuthorFileMapping:
        """Create a new author file mapping or return existing one if already exists."""
        ...


class AuthorRepository(Protocol):
    """Author repository."""

    async def get(self, author_id: int) -> Author | None:
        """Get an author by ID."""
        ...

    async def get_by_name(self, name: str) -> Author | None:
        """Get an author by name."""
        ...

    async def get_by_email(self, email: str) -> Author | None:
        """Get an author by email."""
        ...

    async def list(self) -> Sequence[Author]:
        """List authors."""
        ...

    async def add(self, author: Author) -> None:
        """Add an author."""
        ...

    async def remove(self, author: Author) -> None:
        """Remove an author."""
        ...
