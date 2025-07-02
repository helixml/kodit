"""Unified logging and progress-reporting helper.

This utility consolidates the repeated pattern where services:
1. Log a message (usually via structlog) and
2. Emit a ProgressEvent via a ProgressCallback.

Using Reporter removes boiler-plate and guarantees consistent telemetry.
"""

import structlog

from kodit.domain.interfaces import NullProgressCallback, ProgressCallback
from kodit.domain.value_objects import ProgressEvent


class Reporter:
    """Emit log and progress updates with a single call."""

    def __init__(
        self,
        logger: structlog.BoundLogger | None = None,
        progress: ProgressCallback | None = None,
    ) -> None:
        """Initialize the reporter."""
        self.log: structlog.BoundLogger = logger or structlog.get_logger(__name__)
        self.progress: ProgressCallback = progress or NullProgressCallback()

    # ---------------------------------------------------------------------
    # Life-cycle helpers
    # ---------------------------------------------------------------------
    async def start(
        self, operation: str, total: int, message: str | None = None
    ) -> None:
        """Log *operation.start* and emit initial ProgressEvent."""
        self.log.debug(
            "operation.start", operation=operation, total=total, message=message
        )
        await self.progress.on_progress(
            ProgressEvent(operation=operation, current=0, total=total, message=message)
        )

    async def step(
        self,
        operation: str,
        current: int,
        total: int,
        message: str | None = None,
    ) -> None:
        """Emit an intermediate progress step (no log by default)."""
        await self.progress.on_progress(
            ProgressEvent(
                operation=operation, current=current, total=total, message=message
            )
        )

    async def done(self, operation: str, message: str | None = None) -> None:
        """Log *operation.done* and emit completion event."""
        self.log.debug("operation.done", operation=operation, message=message)
        await self.progress.on_complete(operation)

    async def advance(
        self,
        operation: str,
        current: int,
        total: int,
        message: str | None = None,
        log_every: int | None = None,
    ) -> None:
        """Emit step; optionally log when *current % log_every == 0*."""
        if log_every and current % log_every == 0:
            self.log.debug(
                "operation.progress",
                operation=operation,
                current=current,
                total=total,
                message=message,
            )
        await self.step(operation, current, total, message)
