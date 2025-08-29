"""Progress state."""

from dataclasses import dataclass


@dataclass
class ProgressState:
    """Progress state."""

    current: int = 0
    total: int = 0
    message: str | None = None

    @property
    def percentage(self) -> float:
        """Calculate the percentage of completion."""
        return (self.current / self.total) * 100 if self.total > 0 else 0.0
