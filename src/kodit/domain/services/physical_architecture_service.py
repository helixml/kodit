"""Core service for discovering physical architecture and generating narrative observations."""  # noqa: E501

from datetime import UTC, datetime
from pathlib import Path

from kodit.domain.enrichments.architecture.physical.discovery_notes import (
    ArchitectureDiscoveryNotes,
)
from kodit.domain.enrichments.architecture.physical.formatter import (
    PhysicalArchitectureFormatter,
)
from kodit.infrastructure.physical_architecture.detectors import docker_compose_detector

ARCHITECTURE_ENRICHMENT_SYSTEM_PROMPT = """You are an expert software architect.
Deliver the user's request succinctly.
"""

ARCHITECTURE_ENRICHMENT_TASK_PROMPT = """Convert the raw architecture discovery logs
into a clean, structured summary written in markdown.

<architecture_narrative>
{architecture_narrative}
</architecture_narrative>

**Return the following information**

## Services List

For each service, write one line:
- **[Service Name]**: [what it does] | Tech: [technology] | Ports: [ports]

## Service Dependencies

List the important connections:
- [Service A] → [Service B]: [why they connect]

## Mermaid Diagram

Output a Mermaid diagram depicting the architecture using the names of the services and
the ports that they expose.

## Key Information

Answer these questions in 1-2 sentences each:
1. What databases are used and for what?
2. What are the critical services that everything else depends on?
3. Are there any unusual communication patterns between services that people should be
aware of? (e.g. a different direction to what you'd expect)

## Rules:
- Skip duplicate services (keep only one instance)
- Don't list environment variables
- Don't describe Docker volumes in detail
- Focus on WHAT each service does, not HOW it's configured
- If a service name is unclear, make your best guess based on the information
- Keep descriptions to 10 words or less per service
"""


class PhysicalArchitectureService:
    """Core service for discovering physical architecture and generating narrative observations."""  # noqa: E501

    def __init__(self, formatter: PhysicalArchitectureFormatter) -> None:
        """Initialize the service with detectors and formatter."""
        self.docker_detector = docker_compose_detector.DockerComposeDetector()
        self.formatter = formatter

    async def discover_architecture(self, repo_path: Path) -> str:
        """Discover physical architecture and generate rich narrative observations."""
        # Generate repository context overview
        repo_context = await self._analyze_repository_context(repo_path)

        # Collect observations from all detectors
        component_notes = []
        connection_notes = []
        infrastructure_notes = []

        # Run detectors and collect narrative observations
        (
            docker_component_notes,
            docker_connection_notes,
            docker_infrastructure_notes,
        ) = await self.docker_detector.analyze(repo_path)
        component_notes.extend(docker_component_notes)
        connection_notes.extend(docker_connection_notes)
        infrastructure_notes.extend(docker_infrastructure_notes)

        # Future: Add Kubernetes and code structure detectors when available

        # Generate discovery metadata
        discovery_metadata = self._generate_discovery_metadata(repo_path)

        # Create comprehensive notes
        notes = ArchitectureDiscoveryNotes(
            repository_context=repo_context,
            component_observations=component_notes,
            connection_observations=connection_notes,
            infrastructure_observations=infrastructure_notes,
            discovery_metadata=discovery_metadata,
        )

        return self.formatter.format_for_llm(notes)

    async def _analyze_repository_context(self, repo_path: Path) -> str:
        """Generate high-level repository context and scope."""
        context_observations = []

        # Check for basic repository structure
        context_observations.append(f"Analyzing repository at {repo_path}")

        # Check for common project indicators
        has_docker_compose = bool(
            list(repo_path.glob("docker-compose*.yml"))
            + list(repo_path.glob("docker-compose*.yaml"))
        )
        has_dockerfile = bool(list(repo_path.glob("Dockerfile*")))
        has_k8s = bool(
            list(repo_path.glob("**/k8s/**/*.yaml"))
            + list(repo_path.glob("**/kubernetes/**/*.yaml"))
        )
        has_package_json = (repo_path / "package.json").exists()
        has_requirements_txt = (repo_path / "requirements.txt").exists()
        has_go_mod = (repo_path / "go.mod").exists()

        # Determine likely project type
        project_indicators = []
        if has_docker_compose:
            project_indicators.append("Docker Compose orchestration")
        if has_dockerfile:
            project_indicators.append("containerized deployment")
        if has_k8s:
            project_indicators.append("Kubernetes deployment")
        if has_package_json:
            project_indicators.append("Node.js/JavaScript components")
        if has_requirements_txt:
            project_indicators.append("Python components")
        if has_go_mod:
            project_indicators.append("Go components")

        if project_indicators:
            context_observations.append(
                f"Repository shows evidence of {', '.join(project_indicators)}, "
                "suggesting a modern containerized application architecture."
            )
        else:
            context_observations.append(
                "Repository structure analysis shows limited infrastructure configuration. "  # noqa: E501
                "This may be a simple application or library without complex deployment requirements."  # noqa: E501
            )

        return " ".join(context_observations)

    def _generate_discovery_metadata(self, _repo_path: Path) -> str:
        """Document discovery methodology, confidence, and limitations."""
        timestamp = datetime.now(UTC).isoformat()

        metadata_parts = [
            f"Analysis completed on {timestamp} using physical architecture discovery system version 1.0.",  # noqa: E501
            "Discovery methodology: Docker Compose parsing and infrastructure configuration analysis.",  # noqa: E501
        ]

        # Document detection sources used
        sources_used = ["Docker Compose file analysis"]
        # Future: Add Kubernetes manifest and code analysis sources

        metadata_parts.append(f"Detection sources: {', '.join(sources_used)}.")

        # Document confidence levels
        metadata_parts.append(
            "Confidence levels: High confidence for infrastructure-defined components, "
            "medium confidence for inferred roles based on naming and configuration patterns."  # noqa: E501
        )

        # Document limitations
        limitations = [
            "analysis limited to Docker Compose configurations",
            "code-level analysis not yet implemented",
            "runtime behavior patterns not captured",
        ]
        metadata_parts.append(f"Current limitations: {', '.join(limitations)}.")

        return " ".join(metadata_parts)
