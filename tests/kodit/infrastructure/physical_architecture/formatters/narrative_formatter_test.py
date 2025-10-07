"""Tests for narrative formatter."""

from kodit.domain.enrichments.architecture.physical.discovery_notes import (
    ArchitectureDiscoveryNotes,
)
from kodit.infrastructure.physical_architecture.formatters.narrative_formatter import (
    NarrativeFormatter,
)


class TestNarrativeFormatter:
    """Test narrative formatter for LLM-optimized output."""

    def test_format_discovery_notes_with_full_data(self) -> None:
        """Test formatting discovery notes with comprehensive data."""
        formatter = NarrativeFormatter()

        notes = ArchitectureDiscoveryNotes(
            repository_context="Test repository with Docker Compose configuration",
            component_observations=["API service", "Database service"],
            connection_observations=["API depends on database"],
            infrastructure_observations=["Docker Compose"],
            discovery_metadata="Analysis completed"
        )

        result = formatter.format_for_llm(notes)
        assert isinstance(result, str)
        assert len(result) > 0

    def test_format_discovery_notes_with_empty_observations(self) -> None:
        """Test formatting discovery notes with no observations."""
        formatter = NarrativeFormatter()

        notes = ArchitectureDiscoveryNotes(
            repository_context="Simple repository",
            component_observations=[],
            connection_observations=[],
            infrastructure_observations=[],
            discovery_metadata="Limited findings"
        )

        result = formatter.format_for_llm(notes)
        assert isinstance(result, str)
        assert len(result) > 0
