"""Spinner for long-running tasks in the UI layer."""

import itertools
import sys
import threading
import time


class Spinner:
    """Spinner for long-running tasks.

    This class provides visual feedback for long-running operations by displaying
    a spinning animation in the terminal. It's designed to be used as a context
    manager for operations that may take some time to complete.
    """

    def __init__(self, delay: float = 0.1) -> None:
        """Initialize the spinner.

        Args:
            delay: The delay between spinner updates in seconds.

        """
        self.spinner = itertools.cycle(["-", "/", "|", "\\"])
        self.delay = delay
        self.busy = False
        self.spinner_visible = False

    def write_next(self) -> None:
        """Write the next character of the spinner."""
        with self._screen_lock:
            if not self.spinner_visible:
                sys.stdout.write(next(self.spinner))
                self.spinner_visible = True
                sys.stdout.flush()

    def remove_spinner(self, cleanup: bool = False) -> None:  # noqa: FBT001, FBT002
        """Remove the spinner.

        Args:
            cleanup: Whether to clean up the spinner display.

        """
        with self._screen_lock:
            if self.spinner_visible:
                sys.stdout.write("\b")
                self.spinner_visible = False
                if cleanup:
                    sys.stdout.write(" ")  # overwrite spinner with blank
                    sys.stdout.write("\r")  # move to next line
                sys.stdout.flush()

    def spinner_task(self) -> None:
        """Task that runs the spinner."""
        while self.busy:
            self.write_next()
            time.sleep(self.delay)
            self.remove_spinner()

    def __enter__(self) -> None:
        """Enter the context manager."""
        if sys.stdout.isatty():
            self._screen_lock = threading.Lock()
            self.busy = True
            self.thread = threading.Thread(target=self.spinner_task)
            self.thread.start()

    def __exit__(self, exception: object, value: object, tb: object) -> None:
        """Exit the context manager."""
        if sys.stdout.isatty():
            self.busy = False
            self.remove_spinner(cleanup=True)
        else:
            sys.stdout.write("\r")
