"""Narrative formatter for converting observations to LLM-optimized text."""

import re

from kodit.domain.enrichments.architecture.physical.discovery_notes import (
    ArchitectureDiscoveryNotes,
)


class NarrativeFormatter:
    """Formats architecture observations into narrative text optimized for LLM consumption."""  # noqa: E501

    def format_for_llm(self, notes: ArchitectureDiscoveryNotes) -> str:
        """Convert discovery notes into a comprehensive narrative format."""
        sections = []

        # Title and overview
        sections.append("# Physical Architecture Discovery Report")
        sections.append("")
        sections.append(notes.repository_context)
        sections.append("")

        # Component Analysis
        self._add_component_section(sections, notes.component_observations)

        # Connection Analysis
        self._add_connection_section(sections, notes.connection_observations)

        # Infrastructure Analysis
        self._add_infrastructure_section(sections, notes.infrastructure_observations)

        # Methodology
        sections.append("## Discovery Methodology")
        sections.append(notes.discovery_metadata)
        sections.append("")

        # Conclusion
        self._add_conclusion_section(sections, notes)

        return "\n".join(sections)

    def _add_component_section(
        self, sections: list[str], component_observations: list[str]
    ) -> None:
        """Add component observations section with proper formatting."""
        sections.append("## Components")
        sections.append("")

        if component_observations:
            for i, observation in enumerate(component_observations, 1):
                sections.append(f"**{i}.** {observation}")
            sections.append("")

            # Extract and highlight port information
            port_info = self._extract_port_information(component_observations)
            if port_info:
                sections.append("### Port Mappings")
                sections.append("")
                for component, ports_desc in port_info.items():
                    sections.append(f"- **{component}**: {ports_desc}")
                sections.append("")
        else:
            sections.append("None. Likely monolithic or library architecture.")
            sections.append("")

    def _add_connection_section(
        self, sections: list[str], connection_observations: list[str]
    ) -> None:
        """Add connection observations section with proper formatting."""
        sections.append("## Connections")
        sections.append("")

        if connection_observations:
            for i, observation in enumerate(connection_observations, 1):
                sections.append(f"**{i}.** {observation}")
            sections.append("")
        else:
            sections.append("None. Possible monolithic or independent services.")
            sections.append("")

    def _add_infrastructure_section(
        self, sections: list[str], infrastructure_observations: list[str]
    ) -> None:
        """Add infrastructure observations section with proper formatting."""
        sections.append("## Infrastructure")
        sections.append("")

        if infrastructure_observations:
            for i, observation in enumerate(infrastructure_observations, 1):
                sections.append(f"**{i}.** {observation}")
            sections.append("")
        else:
            sections.append("None. May use external or cloud-native deployment.")
            sections.append("")

    def _add_conclusion_section(
        self, sections: list[str], notes: ArchitectureDiscoveryNotes
    ) -> None:
        """Add a conclusion section summarizing the findings."""
        sections.append("## Summary")
        sections.append("")

        # Determine architecture characteristics
        has_components = bool(notes.component_observations)
        has_connections = bool(notes.connection_observations)
        has_infrastructure = bool(notes.infrastructure_observations)

        if has_components and has_connections and has_infrastructure:
            arch_type = "distributed microservices"
            complexity = "high"
        elif has_components and (has_connections or has_infrastructure):
            arch_type = "multi-component"
            complexity = "medium"
        elif has_components or has_infrastructure:
            arch_type = "structured application"
            complexity = "medium"
        else:
            arch_type = "monolithic"
            complexity = "low"

        sections.append(f"**Architecture:** {arch_type} | **Complexity:** {complexity}")
        sections.append("")
        sections.append("**Note:** Static analysis only. Runtime behavior may differ.")

    def _extract_port_information(
        self, component_observations: list[str]
    ) -> dict[str, str]:
        """Extract port information from component observations."""
        port_info = {}

        # Pattern to extract service name and port information
        service_pattern = r"Found '([^']+)' service"
        port_pattern = r"Exposes ports ([\d, ]+)(?: suggesting ([^.]+))?"

        for observation in component_observations:
            service_match = re.search(service_pattern, observation)
            port_match = re.search(port_pattern, observation)

            if service_match and port_match:
                service_name = service_match.group(1)
                ports = port_match.group(1).strip()
                protocol = port_match.group(2)

                if protocol:
                    port_info[service_name] = f"{ports} ({protocol})"
                else:
                    port_info[service_name] = ports

        return port_info
