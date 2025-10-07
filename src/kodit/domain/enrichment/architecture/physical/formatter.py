"""Physical architecture formatter protocol."""

from typing import Any, Protocol


class PhysicalArchitectureFormatter(Protocol):
    """Formatter for converting architecture discovery notes to LLM-optimized text."""

    def format_for_llm(self, notes: Any) -> str:
        """Format architecture discovery notes for LLM consumption."""
        ...
