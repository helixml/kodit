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
            component_observations=[
                "Found 'api' service configured as backend API service",
                "Found 'database' service configured as PostgreSQL database"
            ],
            connection_observations=[
                "API service depends on database service for data persistence"
            ],
            infrastructure_observations=[
                "Docker Compose configuration indicates containerized deployment"
            ],
            discovery_metadata="Analysis completed with high confidence"
        )

        result = formatter.format_discovery_notes(notes)

        # Should be substantial formatted text
        assert isinstance(result, str)
        assert len(result) > 500

        # Should contain expected structure
        assert "# Physical Architecture Discovery Report" in result
        assert "## Executive Summary" in result
        assert "## Repository Analysis Scope" in result
        assert "## Component Architecture" in result
        assert "## Service Communication Patterns" in result
        assert "## Infrastructure and Deployment Patterns" in result
        assert "## Discovery Methodology and Confidence Assessment" in result
        assert "## Architecture Summary" in result

        # Should include the provided content
        assert "Test repository with Docker Compose" in result
        assert "api" in result.lower()
        assert "database" in result.lower()
        assert "depends" in result.lower()
        assert "containerized" in result.lower()

    def test_format_discovery_notes_with_empty_observations(self) -> None:
        """Test formatting discovery notes with no observations."""
        formatter = NarrativeFormatter()

        notes = ArchitectureDiscoveryNotes(
            repository_context="Simple repository with minimal configuration",
            component_observations=[],
            connection_observations=[],
            infrastructure_observations=[],
            discovery_metadata="Analysis completed with limited findings"
        )

        result = formatter.format_discovery_notes(notes)

        # Should still provide comprehensive report
        assert isinstance(result, str)
        assert len(result) > 300

        # Should contain structure even with empty data
        assert "# Physical Architecture Discovery Report" in result
        assert "## Component Architecture" in result
        assert "## Service Communication Patterns" in result

        # Should indicate no findings appropriately
        assert any(phrase in result for phrase in [
            "No distinct architectural components",
            "No explicit service connections",
            "No infrastructure configuration patterns"
        ])

        # Should still provide analysis and recommendations
        assert "simple or monolithic architecture" in result.lower()

    def test_format_provides_llm_optimization(self) -> None:
        """Test that formatting is optimized for LLM consumption."""
        formatter = NarrativeFormatter()

        notes = ArchitectureDiscoveryNotes(
            repository_context="Repository analysis",
            component_observations=["Service A", "Service B"],
            connection_observations=["A connects to B"],
            infrastructure_observations=["Docker configuration"],
            discovery_metadata="Test metadata"
        )

        result = formatter.format_discovery_notes(notes)

        # Should use markdown formatting
        assert result.count("#") >= 6  # Multiple headers
        assert "**" in result  # Bold formatting

        # Should have clear section separation
        sections = result.split("##")
        assert len(sections) >= 7  # Multiple sections

        # Should provide context and explanation, not just data
        result_lower = result.lower()
        assert any(explanatory_word in result_lower for explanatory_word in [
            "indicates", "suggests", "demonstrates", "analysis", "patterns"
        ])

    def test_format_includes_architecture_assessment(self) -> None:
        """Test that formatting includes architecture complexity assessment."""
        formatter = NarrativeFormatter()

        # Test high complexity scenario
        high_complexity_notes = ArchitectureDiscoveryNotes(
            repository_context="Complex microservices repository",
            component_observations=["API service", "Database service", "Cache service"],
            connection_observations=["API to DB", "API to Cache"],
            infrastructure_observations=["Docker Compose", "Volume mounts"],
            discovery_metadata="High confidence analysis"
        )

        result = formatter.format_discovery_notes(high_complexity_notes)

        # Should assess complexity
        assert "## Architecture Summary" in result
        result_lower = result.lower()
        assert any(complexity_word in result_lower for complexity_word in [
            "complexity", "architecture", "distributed", "microservices"
        ])

    def test_format_provides_actionable_recommendations(self) -> None:
        """Test that formatting provides actionable insights and recommendations."""
        formatter = NarrativeFormatter()

        notes = ArchitectureDiscoveryNotes(
            repository_context="Repository with limited infrastructure",
            component_observations=[],
            connection_observations=[],
            infrastructure_observations=[],
            discovery_metadata="Limited analysis"
        )

        result = formatter.format_discovery_notes(notes)

        # Should provide recommendations for improvement
        result_lower = result.lower()
        assert any(recommendation_phrase in result_lower for recommendation_phrase in [
            "may provide additional insights",
            "code-level analysis",
            "runtime analysis",
            "further",
            "additional"
        ])

        # Should acknowledge limitations
        assert "Note:" in result or "note:" in result.lower()
        assert "static file examination" in result.lower()
