"""TQDM progress."""

from tqdm import tqdm

from kodit.infrastructure.reporting.progress import Progress
from kodit.infrastructure.reporting.progress_state import ProgressState


class TQDMProgress(Progress):
    """TQDM-based progress callback implementation."""

    def __init__(self) -> None:
        """Initialize with a TQDM progress bar."""
        self.pbar = tqdm()

    def on_update(self, state: ProgressState) -> None:
        """Update the TQDM progress bar."""
        # Update total if it changes
        if state.total != self.pbar.total:
            self.pbar.total = state.total

        # Update the progress bar
        self.pbar.n = state.current
        self.pbar.refresh()

        # Update description if message is provided
        if state.message:
            # Fix the event message to a specific size so it's not jumping around
            # If it's too small, add spaces
            # If it's too large, truncate
            if len(state.message) < 30:
                self.pbar.set_description(
                    state.message + " " * (30 - len(state.message))
                )
            else:
                self.pbar.set_description(state.message[-30:])

    def on_complete(self) -> None:
        """Complete the progress bar."""
        self.pbar.close()
