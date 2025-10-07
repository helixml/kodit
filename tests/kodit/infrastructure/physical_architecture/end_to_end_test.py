"""End-to-end test for physical architecture discovery."""

from pathlib import Path

import pytest

from kodit.domain.services.physical_architecture_service import (
    PhysicalArchitectureService,
)
from kodit.infrastructure.physical_architecture.formatters.narrative_formatter import (
    NarrativeFormatter,
)


class TestPhysicalArchitectureEndToEnd:
    """End-to-end test for physical architecture discovery with narrative output."""

    @pytest.mark.asyncio
    async def test_discover_architecture_simple_web_app(self) -> None:
        """Test full architecture discovery on simple web app fixture generates narrative."""  # noqa: E501
        formatter = NarrativeFormatter()
        service = PhysicalArchitectureService(formatter=formatter)
        fixture_path = Path(__file__).parent / "fixtures" / "simple_web_app"

        narrative = await service.discover_architecture(fixture_path)

        # Should return narrative text, not structured data
        assert isinstance(narrative, str)
        assert len(narrative) > 100  # Should be substantial text

        # Should contain markdown structure
        assert "# Physical Architecture Discovery Report" in narrative
        assert "## Components" in narrative
        assert "## Connections" in narrative
        assert "## Infrastructure" in narrative
        assert "## Discovery Methodology" in narrative

        # Should mention the services from the fixture
        narrative_lower = narrative.lower()
        assert "api" in narrative_lower
        assert "postgres" in narrative_lower
        assert "redis" in narrative_lower
        assert "frontend" in narrative_lower
        assert "worker" in narrative_lower

        # Should describe architectural concepts
        assert any(word in narrative_lower for word in [
            "service", "component", "database", "cache", "dependency"
        ])

        # Should mention Docker Compose
        assert "docker" in narrative_lower
        assert "compose" in narrative_lower

    @pytest.mark.asyncio
    async def test_discover_architecture_empty_directory(self) -> None:
        """Test architecture discovery on empty directory generates appropriate narrative."""  # noqa: E501
        formatter = NarrativeFormatter()
        service = PhysicalArchitectureService(formatter=formatter)
        empty_path = Path("/tmp")  # Unlikely to have docker-compose files

        narrative = await service.discover_architecture(empty_path)

        # Should still return narrative text
        assert isinstance(narrative, str)
        assert len(narrative) > 100  # Should have basic structure

        # Should contain markdown structure
        assert "# Physical Architecture Discovery Report" in narrative
        assert "## Components" in narrative

        # Should indicate no components found
        narrative_lower = narrative.lower()
        assert any(phrase in narrative_lower for phrase in [
            "no distinct components",
            "no infrastructure configuration",
            "limited infrastructure",
            "no components detected"
        ])

    @pytest.mark.asyncio
    async def test_narrative_structure_and_content(self) -> None:
        """Test that narrative has proper structure and descriptive content."""
        formatter = NarrativeFormatter()
        service = PhysicalArchitectureService(formatter=formatter)
        fixture_path = Path(__file__).parent / "fixtures" / "simple_web_app"

        narrative = await service.discover_architecture(fixture_path)

        # Should have clear section structure
        assert "## Discovery Methodology" in narrative
        sections = narrative.split("##")
        assert len(sections) >= 5

        # Should describe relationships and context
        narrative_lower = narrative.lower()
        assert any(word in narrative_lower for word in [
            "depends", "dependency", "database", "configuration"
        ])
