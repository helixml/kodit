"""Tests for Docker Compose detector."""

import asyncio
from pathlib import Path

from kodit.infrastructure.physical_architecture.detectors.docker_compose_detector import (  # noqa: E501
    DockerComposeDetector,
)


class TestDockerComposeDetector:
    """Test Docker Compose detector for narrative generation."""

    def test_analyze_simple_compose_generates_observations(self) -> None:
        """Test analyzing simple docker-compose.yml generates narrative observations."""
        detector = DockerComposeDetector()
        fixture_path = Path(__file__).parent.parent / "fixtures" / "simple_web_app"

        # Run async analysis
        component_notes, connection_notes, infrastructure_notes = (
            asyncio.run(detector.analyze(fixture_path))
        )

        # Should have component observations
        assert len(component_notes) > 0

        # Should have connection observations
        assert len(connection_notes) > 0

        # Should have infrastructure observations
        assert len(infrastructure_notes) > 0

        # Check that observations contain expected service names
        component_text = " ".join(component_notes)
        assert "api" in component_text.lower()
        assert "postgres" in component_text.lower()
        assert "redis" in component_text.lower()
        assert "frontend" in component_text.lower()
        assert "worker" in component_text.lower()

        # Check that component observations describe roles
        assert any("database" in note.lower() for note in component_notes)
        assert any("cache" in note.lower() for note in component_notes)
        assert any("worker" in note.lower() for note in component_notes)

        # Check that connection observations mention dependencies
        connection_text = " ".join(connection_notes)
        assert "depends" in connection_text.lower()

        # Check that infrastructure observations mention Docker Compose
        infrastructure_text = " ".join(infrastructure_notes)
        assert "docker compose" in infrastructure_text.lower()

    def test_analyze_empty_directory_returns_empty_observations(self) -> None:
        """Test analyzing directory with no compose files returns empty observations."""
        detector = DockerComposeDetector()
        empty_path = Path("/tmp")  # Unlikely to have docker-compose files

        component_notes, connection_notes, infrastructure_notes = (
            asyncio.run(detector.analyze(empty_path))
        )

        # Should return empty observations
        assert component_notes == []
        assert connection_notes == []
        assert infrastructure_notes == []

    def test_analyze_generates_descriptive_component_observations(self) -> None:
        """Test that component observations are descriptive and informative."""
        detector = DockerComposeDetector()
        fixture_path = Path(__file__).parent.parent / "fixtures" / "simple_web_app"

        component_notes, connection_notes, infrastructure_notes = (
            asyncio.run(detector.analyze(fixture_path))
        )

        # Each component observation should be descriptive (more than just names)
        for note in component_notes:
            assert len(note) > 50  # Should be substantial narrative text
            assert "service" in note.lower()  # Should mention it's a service
            assert "configuration" in note.lower() or "configured" in note.lower()

        # Should describe service purposes, not just list them
        component_text = " ".join(component_notes)
        assert ("database service" in component_text.lower() or
                "database configuration" in component_text.lower())
        assert "cache" in component_text.lower() or "caching" in component_text.lower()

    def test_analyze_generates_meaningful_connection_observations(self) -> None:
        """Test that connection observations explain relationships meaningfully."""
        detector = DockerComposeDetector()
        fixture_path = Path(__file__).parent.parent / "fixtures" / "simple_web_app"

        component_notes, connection_notes, infrastructure_notes = (
            asyncio.run(detector.analyze(fixture_path))
        )

        # Connection observations should explain dependency relationships
        for note in connection_notes:
            assert len(note) > 30  # Should be meaningful descriptions
            assert any(word in note.lower() for word in
                      ["depends", "dependency", "requires", "startup"])

        # Should mention specific service relationships
        connection_text = " ".join(connection_notes)
        assert "api" in connection_text.lower()
        assert "postgres" in connection_text.lower()

    def test_analyze_generates_infrastructure_insights(self) -> None:
        """Test that infrastructure observations provide deployment insights."""
        detector = DockerComposeDetector()
        fixture_path = Path(__file__).parent.parent / "fixtures" / "simple_web_app"

        component_notes, connection_notes, infrastructure_notes = (
            asyncio.run(detector.analyze(fixture_path))
        )

        # Infrastructure observations should describe deployment patterns
        infrastructure_text = " ".join(infrastructure_notes)
        assert "docker" in infrastructure_text.lower()
        assert "service" in infrastructure_text.lower()
        assert any(word in infrastructure_text.lower() for word in [
            "containerized", "orchestrated", "deployment", "architecture"
        ])

        # Should mention number of services or configuration details
        assert any(char.isdigit() for char in infrastructure_text)  # Should contain numbers  # noqa: E501
