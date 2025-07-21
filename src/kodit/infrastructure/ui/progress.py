"""Progress UI implementations."""

from collections.abc import Callable

import structlog
from tqdm import tqdm  # type: ignore[import-untyped]

from kodit.domain.interfaces import ProgressCallback
from kodit.domain.value_objects import ProgressEvent


class TQDMProgressCallback(ProgressCallback):
    """TQDM-based progress callback implementation."""

    def __init__(self, pbar: "tqdm") -> None:
        """Initialize with a TQDM progress bar."""
        self.pbar = pbar

    async def on_progress(self, event: ProgressEvent) -> None:
        """Update the TQDM progress bar."""
        # Update total if it changes
        if event.total != self.pbar.total:
            self.pbar.total = event.total

        # Update the progress bar
        self.pbar.n = event.current
        self.pbar.refresh()

        # Update description if message is provided
        if event.message:
            # Fix the event message to a specific size so it's not jumping around
            # If it's too small, add spaces
            # If it's too large, truncate
            if len(event.message) < 30:
                self.pbar.set_description(
                    event.message + " " * (30 - len(event.message))
                )
            else:
                self.pbar.set_description(event.message[-30:])

    async def on_complete(self, operation: str) -> None:
        """Complete the progress bar."""
        # TQDM will handle cleanup with leave=False


class LogProgressCallback(ProgressCallback):
    """Log-based progress callback for server environments."""

    def __init__(self, milestone_interval: int = 10) -> None:
        """Initialize with milestone logging interval.

        Args:
            milestone_interval: Percentage interval for logging (default: 10%)

        """
        self.milestone_interval = milestone_interval
        self._last_logged_percentage = -1
        self.log = structlog.get_logger()

    async def on_progress(self, event: ProgressEvent) -> None:
        """Log progress at milestone intervals."""
        percentage = int(event.percentage)

        # Log at milestone intervals (0%, 10%, 20%, etc.)
        milestone = (percentage // self.milestone_interval) * self.milestone_interval

        if milestone > self._last_logged_percentage and milestone <= percentage:
            self.log.info(
                "Progress milestone reached",
                operation=event.operation,
                percentage=milestone,
                current=event.current,
                total=event.total,
                message=event.message,
            )
            self._last_logged_percentage = milestone

    async def on_complete(self, operation: str) -> None:
        """Log completion of the operation."""
        self.log.info("Operation completed", operation=operation)


class LazyProgressCallback(ProgressCallback):
    """Progress callback that only shows progress when there's actual work to do."""

    def __init__(self, create_pbar_func: Callable[[], tqdm]) -> None:
        """Initialize with a function that creates a progress bar."""
        self.create_pbar_func = create_pbar_func
        self._callback: ProgressCallback | None = None
        self._has_work = False

    async def on_progress(self, event: ProgressEvent) -> None:
        """Update progress, creating the actual callback if needed."""
        if not self._has_work:
            self._has_work = True
            # Only create the progress bar when we actually have work to do
            pbar = self.create_pbar_func()
            self._callback = TQDMProgressCallback(pbar)

        if self._callback:
            await self._callback.on_progress(event)

    async def on_complete(self, operation: str) -> None:
        """Complete the progress operation."""
        if self._callback:
            await self._callback.on_complete(operation)


class MultiStageProgressCallback(ProgressCallback):
    """Progress callback that handles multiple stages with separate progress bars."""

    def __init__(self, create_pbar_func: Callable[[str], tqdm]) -> None:
        """Initialize with a function that creates progress bars."""
        self.create_pbar_func = create_pbar_func
        self._current_callback: ProgressCallback | None = None
        self._current_operation: str | None = None

    async def on_progress(self, event: ProgressEvent) -> None:
        """Update progress for the current operation."""
        # If this is a new operation, create a new progress bar
        if self._current_operation != event.operation:
            # Create a new progress bar for this operation
            pbar = self.create_pbar_func(event.operation)
            self._current_callback = TQDMProgressCallback(pbar)
            self._current_operation = event.operation

        # Update the current progress bar
        if self._current_callback:
            await self._current_callback.on_progress(event)

    async def on_complete(self, operation: str) -> None:
        """Complete the current operation."""
        if self._current_callback and self._current_operation == operation:
            await self._current_callback.on_complete(operation)
            self._current_callback = None
            self._current_operation = None


def create_progress_bar(desc: str = "Processing", unit: str = "items") -> "tqdm":
    """Create a progress bar with the given description and unit."""
    from tqdm import tqdm

    return tqdm(
        desc=desc,
        unit=unit,
        leave=False,
        dynamic_ncols=True,
        total=None,  # Will be set dynamically
        position=0,  # Position at top
        mininterval=0.1,  # Update at most every 0.1 seconds
    )


def create_lazy_progress_callback() -> LazyProgressCallback:
    """Create a lazy progress callback that only shows progress when needed."""
    return LazyProgressCallback(
        lambda: create_progress_bar("Processing files", "files")
    )


def create_multi_stage_progress_callback() -> MultiStageProgressCallback:
    """Create a multi-stage progress callback for indexing operations."""
    return MultiStageProgressCallback(
        lambda operation: create_progress_bar(operation, "items")
    )


def create_log_progress_callback(milestone_interval: int = 10) -> LogProgressCallback:
    """Create a log-based progress callback for server environments."""
    return LogProgressCallback(milestone_interval=milestone_interval)
