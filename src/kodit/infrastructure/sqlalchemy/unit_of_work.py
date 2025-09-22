"""SQLAlchemy implementation of Unit of Work pattern."""

from collections.abc import Callable
from types import TracebackType

from sqlalchemy.ext.asyncio import AsyncSession


class SqlAlchemyUnitOfWork:
    """SQLAlchemy implementation of Unit of Work pattern."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the unit of work with a session factory."""
        self._session_factory = session_factory
        self._session: AsyncSession | None = None

    async def __aenter__(self) -> "AsyncSession":
        """Enter the unit of work context."""
        self._session = self._session_factory()
        return self._session

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc_val: BaseException | None,
        exc_tb: TracebackType | None,
    ) -> None:
        """Exit the unit of work context."""
        if self._session:
            try:
                if exc_type is None:  # Only commit if no exception
                    await self._session.commit()
                else:
                    await self._session.rollback()
            finally:
                await self._session.close()
                self._session = None

    async def commit(self) -> None:
        """Commit the current transaction."""
        if self._session is None:
            raise RuntimeError("UnitOfWork must be used within async context")
        await self._session.commit()

    async def rollback(self) -> None:
        """Rollback the current transaction."""
        if self._session is None:
            raise RuntimeError("UnitOfWork must be used within async context")
        await self._session.rollback()

    async def flush(self) -> None:
        """Flush pending changes to the database without committing."""
        if self._session is None:
            raise RuntimeError("UnitOfWork must be used within async context")
        await self._session.flush()
