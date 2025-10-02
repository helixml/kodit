"""Tests for narrative formatter."""

from kodit.domain.physical_architecture import ArchitectureDiscoveryNotes
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

    def test_format_extracts_port_information(self) -> None:
        """Test that port information is extracted and highlighted."""
        formatter = NarrativeFormatter()

        notes = ArchitectureDiscoveryNotes(
            repository_context="Test with port information",
            component_observations=[
                "Found 'api' service in Docker Compose configuration. "
                "Service uses 'python:3.11' Docker image. "
                "Exposes ports 8080, 443 suggesting HTTP/HTTPS service.",
                "Found 'database' service in Docker Compose configuration. "
                "Service uses 'postgres:15' Docker image. "
                "Exposes ports 5432 suggesting database service.",
            ],
            connection_observations=["API depends on database"],
            infrastructure_observations=["Docker Compose"],
            discovery_metadata="Analysis completed"
        )

        result = formatter.format_for_llm(notes)

        # Check port mappings section exists
        assert "### Port Mappings" in result
        # Check port information is extracted
        assert "api" in result
        assert "8080, 443" in result
        assert "HTTP/HTTPS service" in result
        assert "database" in result
        assert "5432" in result
        assert "database service" in result
