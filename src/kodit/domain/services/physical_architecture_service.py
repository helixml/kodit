"""Core service for discovering physical architecture and generating narrative observations."""  # noqa: E501

from datetime import UTC, datetime
from pathlib import Path

from kodit.domain.physical_architecture import ArchitectureDiscoveryNotes
from kodit.infrastructure.physical_architecture.detectors import docker_compose_detector


class PhysicalArchitectureService:
    """Core service for discovering physical architecture and generating narrative observations."""  # noqa: E501

    def __init__(self) -> None:
        """Initialize the service with detectors."""
        self.docker_detector = docker_compose_detector.DockerComposeDetector()

    async def discover_architecture(self, repo_path: Path) -> str:
        """Discover physical architecture and generate rich narrative observations."""
        # Generate repository context overview
        repo_context = await self._analyze_repository_context(repo_path)

        # Collect observations from all detectors
        component_notes = []
        connection_notes = []
        infrastructure_notes = []

        # Run detectors and collect narrative observations
        docker_component_notes, docker_connection_notes, docker_infrastructure_notes = (
            await self.docker_detector.analyze(repo_path)
        )
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
            discovery_metadata=discovery_metadata
        )

        return self._format_notes_for_llm(notes)

    async def _analyze_repository_context(self, repo_path: Path) -> str:
        """Generate high-level repository context and scope."""
        context_observations = []

        # Check for basic repository structure
        context_observations.append(f"Analyzing repository at {repo_path}")

        # Check for common project indicators
        has_docker_compose = bool(list(repo_path.glob("docker-compose*.yml")) +
                                list(repo_path.glob("docker-compose*.yaml")))
        has_dockerfile = bool(list(repo_path.glob("Dockerfile*")))
        has_k8s = bool(list(repo_path.glob("**/k8s/**/*.yaml")) +
                      list(repo_path.glob("**/kubernetes/**/*.yaml")))
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
            "runtime behavior patterns not captured"
        ]
        metadata_parts.append(f"Current limitations: {', '.join(limitations)}.")

        return " ".join(metadata_parts)

    def _format_notes_for_llm(self, notes: ArchitectureDiscoveryNotes) -> str:
        """Format observations as natural text optimized for LLM consumption."""
        sections = []

        # Repository Context section
        sections.append("# Repository Architecture Discovery")
        sections.append("")
        sections.append("## Repository Context")
        sections.append(notes.repository_context)
        sections.append("")

        # Component Observations section
        if notes.component_observations:
            sections.append("## Component Observations")
            sections.append("")
            for i, observation in enumerate(notes.component_observations, 1):
                sections.append(f"**Component {i}**")
                sections.append(observation)
                sections.append("")
        else:
            sections.append("## Component Observations")
            sections.append("No distinct components identified in the repository architecture.")  # noqa: E501
            sections.append("")

        # Connection Observations section
        if notes.connection_observations:
            sections.append("## Connection Observations")
            sections.append("")
            for i, observation in enumerate(notes.connection_observations, 1):
                sections.append(f"**Connection Pattern {i}**")
                sections.append(observation)
                sections.append("")
        else:
            sections.append("## Connection Observations")
            sections.append("No explicit service connections or dependencies identified.")  # noqa: E501
            sections.append("")

        # Infrastructure Observations section
        if notes.infrastructure_observations:
            sections.append("## Infrastructure Observations")
            sections.append("")
            for i, observation in enumerate(notes.infrastructure_observations, 1):
                sections.append(f"**Infrastructure Pattern {i}**")
                sections.append(observation)
                sections.append("")
        else:
            sections.append("## Infrastructure Observations")
            sections.append("No infrastructure configuration patterns identified.")
            sections.append("")

        # Discovery Metadata section
        sections.append("## Discovery Metadata")
        sections.append(notes.discovery_metadata)

        return "\n".join(sections)
