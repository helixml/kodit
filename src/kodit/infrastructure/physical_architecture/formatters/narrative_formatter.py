"""Narrative formatter for converting observations to LLM-optimized text."""

from kodit.domain.physical_architecture import ArchitectureDiscoveryNotes


class NarrativeFormatter:
    """Formats architecture observations into narrative text optimized for LLM consumption."""  # noqa: E501

    def format_discovery_notes(self, notes: ArchitectureDiscoveryNotes) -> str:
        """Convert discovery notes into a comprehensive narrative format."""
        sections = []

        # Title and overview
        sections.append("# Physical Architecture Discovery Report")
        sections.append("")
        sections.append("## Executive Summary")
        sections.append(
            "This report provides a comprehensive analysis of the repository's physical "  # noqa: E501
            "architecture, including component identification, connection patterns, and "  # noqa: E501
            "infrastructure observations derived from automated discovery processes."
        )
        sections.append("")

        # Repository Context
        sections.append("## Repository Analysis Scope")
        sections.append(notes.repository_context)
        sections.append("")

        # Component Analysis
        self._add_component_section(sections, notes.component_observations)

        # Connection Analysis
        self._add_connection_section(sections, notes.connection_observations)

        # Infrastructure Analysis
        self._add_infrastructure_section(sections, notes.infrastructure_observations)

        # Methodology and Confidence
        sections.append("## Discovery Methodology and Confidence Assessment")
        sections.append(notes.discovery_metadata)
        sections.append("")

        # Conclusion
        self._add_conclusion_section(sections, notes)

        return "\n".join(sections)

    def _add_component_section(self, sections: list[str], component_observations: list[str]) -> None:  # noqa: E501
        """Add component observations section with proper formatting."""
        sections.append("## Component Architecture")
        sections.append("")

        if component_observations:
            sections.append(
                "The following components were identified through infrastructure analysis "  # noqa: E501
                "and configuration pattern recognition:"
            )
            sections.append("")

            for i, observation in enumerate(component_observations, 1):
                sections.append(f"### Component {i}: Service Analysis")
                sections.append(observation)
                sections.append("")
        else:
            sections.append(
                "**No distinct architectural components identified.** "
                "This repository may represent a simple application, library, or "
                "monolithic architecture without explicit service decomposition. "
                "Further code-level analysis may reveal internal modular structure."
            )
            sections.append("")

    def _add_connection_section(self, sections: list[str], connection_observations: list[str]) -> None:  # noqa: E501
        """Add connection observations section with proper formatting."""
        sections.append("## Service Communication Patterns")
        sections.append("")

        if connection_observations:
            sections.append(
                "The following communication patterns and service dependencies were "
                "identified through configuration analysis:"
            )
            sections.append("")

            for i, observation in enumerate(connection_observations, 1):
                sections.append(f"### Connection Pattern {i}")
                sections.append(observation)
                sections.append("")
        else:
            sections.append(
                "**No explicit service connections identified.** "
                "This may indicate a monolithic architecture, independent services, "
                "or communication patterns not captured by current analysis methods. "
                "Runtime analysis or code-level inspection may reveal additional "
                "communication patterns."
            )
            sections.append("")

    def _add_infrastructure_section(self, sections: list[str], infrastructure_observations: list[str]) -> None:  # noqa: E501
        """Add infrastructure observations section with proper formatting."""
        sections.append("## Infrastructure and Deployment Patterns")
        sections.append("")

        if infrastructure_observations:
            sections.append(
                "The following infrastructure patterns and deployment configurations "
                "were observed:"
            )
            sections.append("")

            for i, observation in enumerate(infrastructure_observations, 1):
                sections.append(f"### Infrastructure Pattern {i}")
                sections.append(observation)
                sections.append("")
        else:
            sections.append(
                "**No infrastructure configuration patterns identified.** "
                "This repository may use external deployment configurations, "
                "cloud-native deployment platforms, or simple deployment strategies "
                "not captured by file-based analysis."
            )
            sections.append("")

    def _add_conclusion_section(self, sections: list[str], notes: ArchitectureDiscoveryNotes) -> None:  # noqa: E501
        """Add a conclusion section summarizing the findings."""
        sections.append("## Architecture Summary")
        sections.append("")

        # Determine architecture characteristics
        has_components = bool(notes.component_observations)
        has_connections = bool(notes.connection_observations)
        has_infrastructure = bool(notes.infrastructure_observations)

        if has_components and has_connections and has_infrastructure:
            architecture_type = "distributed microservices architecture"
            complexity = "high"
        elif has_components and (has_connections or has_infrastructure):
            architecture_type = "multi-component architecture"
            complexity = "medium"
        elif has_components or has_infrastructure:
            architecture_type = "structured application architecture"
            complexity = "medium"
        else:
            architecture_type = "simple or monolithic architecture"
            complexity = "low"

        sections.append(
            f"Based on the analysis, this repository demonstrates a **{architecture_type}** "  # noqa: E501
            f"with **{complexity} complexity**. "
        )

        # Add specific recommendations
        if not has_components:
            sections.append(
                "The absence of clearly defined components suggests either a monolithic "  # noqa: E501
                "design or the need for deeper code-level analysis to identify internal "  # noqa: E501
                "architectural boundaries."
            )
        elif not has_connections:
            sections.append(
                "While components are identified, limited connection patterns may indicate "  # noqa: E501
                "loose coupling or the need for runtime analysis to understand "
                "inter-service communication."
            )

        if not has_infrastructure:
            sections.append(
                "Limited infrastructure configuration suggests either simple deployment "  # noqa: E501
                "requirements or external infrastructure management."
            )

        sections.append("")
        sections.append(
            "**Note:** This analysis is based on static file examination. Runtime "
            "behavior, dynamic service discovery, and code-level architecture patterns "
            "may provide additional insights not captured in this report."
        )
