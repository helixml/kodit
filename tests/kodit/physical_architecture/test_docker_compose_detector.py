"""Tests for DockerComposeDetector."""

from pathlib import Path

import pytest

from kodit.physical_architecture.infrastructure.architecture_discovery.docker_compose_detector import (
    DockerComposeDetector,
)


class TestDockerComposeDetector:
    """Test cases for DockerComposeDetector."""
    
    def test_detect_components_from_simple_compose_file(self):
        """Test extracting components from a simple docker-compose file."""
        detector = DockerComposeDetector()
        fixture_path = Path(__file__).parent / "fixtures" / "simple_web_app"
        
        components = detector.detect(fixture_path)
        
        # Should detect 3 components: api, frontend, postgres
        assert len(components) == 3
        
        # Check component names
        component_names = {comp.name for comp in components}
        expected_names = {"api", "frontend", "postgres"}
        assert component_names == expected_names
        
        # Check specific components
        api_component = next(comp for comp in components if comp.name == "api")
        assert api_component.ports == [8080]
        assert api_component.inferred_role == "web_server"
        assert api_component.sources == ["docker-compose"]
        assert "HTTP" in api_component.protocols
        
        frontend_component = next(comp for comp in components if comp.name == "frontend")
        assert frontend_component.ports == [3000]
        assert frontend_component.inferred_role == "client_app"
        assert "HTTP" in frontend_component.protocols
        
        postgres_component = next(comp for comp in components if comp.name == "postgres")
        assert postgres_component.ports == [5432]
        assert postgres_component.inferred_role == "database"
        assert "TCP" in postgres_component.protocols
    
    def test_detect_connections_from_depends_on(self):
        """Test extracting connections from depends_on relationships."""
        detector = DockerComposeDetector()
        fixture_path = Path(__file__).parent / "fixtures" / "simple_web_app"
        
        connections = detector.detect_connections(fixture_path)
        
        # Should detect 1 connection: api depends on postgres
        assert len(connections) == 1
        
        connection = connections[0]
        assert connection.source_component == "api"
        assert connection.target_component == "postgres"
        assert connection.direction == "client_to_server"
        assert connection.protocol == "TCP"  # Database connection
    
    def test_detect_with_no_compose_files(self, tmp_path):
        """Test detection when no docker-compose files are present."""
        detector = DockerComposeDetector()
        
        components = detector.detect(tmp_path)
        connections = detector.detect_connections(tmp_path)
        
        assert len(components) == 0
        assert len(connections) == 0
    
    def test_detect_with_empty_compose_file(self, tmp_path):
        """Test detection with an empty or invalid compose file."""
        detector = DockerComposeDetector()
        
        # Create an empty compose file
        empty_compose = tmp_path / "docker-compose.yml"
        empty_compose.write_text("")
        
        components = detector.detect(tmp_path)
        connections = detector.detect_connections(tmp_path)
        
        assert len(components) == 0
        assert len(connections) == 0