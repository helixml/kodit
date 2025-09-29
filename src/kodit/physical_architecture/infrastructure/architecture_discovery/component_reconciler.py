"""Component reconciler for merging duplicate components from different detectors."""


from kodit.physical_architecture.domain.value_objects import ArchitectureComponent


class ComponentReconciler:
    """Reconciles components detected from different sources to eliminate duplicates."""

    def reconcile(self, all_components: list[ArchitectureComponent]) -> list[ArchitectureComponent]:
        """Take flat list of components from all detectors and reconcile duplicates."""
        if not all_components:
            return []

        # Group components by name (exact match for now)
        component_groups = self._group_by_name(all_components)

        # Merge each group into a single component
        reconciled_components = []
        for components_in_group in component_groups.values():
            merged_component = self._merge_components(components_in_group)
            reconciled_components.append(merged_component)

        return reconciled_components

    def _group_by_name(self, components: list[ArchitectureComponent]) -> dict[str, list[ArchitectureComponent]]:
        """Group components by exact name match."""
        groups: dict[str, list[ArchitectureComponent]] = {}

        for component in components:
            name = component.name.lower()  # Case-insensitive matching
            if name not in groups:
                groups[name] = []
            groups[name].append(component)

        return groups

    def _merge_components(self, components: list[ArchitectureComponent]) -> ArchitectureComponent:
        """Merge multiple components with the same name into one."""
        if len(components) == 1:
            return components[0]

        # Use the first component as the base
        components[0]

        # Merge all properties
        merged_name = self._merge_names(components)
        merged_role = self._merge_roles(components)
        merged_entry_points = self._merge_entry_points(components)
        merged_ports = self._merge_ports(components)
        merged_protocols = self._merge_protocols(components)
        merged_sources = self._merge_sources(components)

        return ArchitectureComponent(
            name=merged_name,
            inferred_role=merged_role,
            entry_points=merged_entry_points,
            ports=merged_ports,
            protocols=merged_protocols,
            sources=merged_sources
        )

    def _merge_names(self, components: list[ArchitectureComponent]) -> str:
        """Merge names - prefer Docker Compose names > K8s names > directory names."""
        # For now, use the name from the component with the highest priority source
        priority_order = ["docker-compose", "k8s", "code", "build"]

        for source_type in priority_order:
            for component in components:
                if source_type in component.sources:
                    return component.name

        # Fallback to first component name
        return components[0].name

    def _merge_roles(self, components: list[ArchitectureComponent]) -> str:
        """Merge roles - use majority voting or prioritize infrastructure sources."""
        # Count role occurrences
        role_counts: dict[str, int] = {}

        # Weight roles from infrastructure sources more heavily
        for component in components:
            weight = 2 if any(source in ["docker-compose", "k8s"] for source in component.sources) else 1

            if component.inferred_role not in role_counts:
                role_counts[component.inferred_role] = 0
            role_counts[component.inferred_role] += weight

        # Return the role with the highest count
        if role_counts:
            return max(role_counts.items(), key=lambda x: x[1])[0]

        return "unknown"

    def _merge_entry_points(self, components: list[ArchitectureComponent]) -> list[str]:
        """Merge entry points - combine all unique entry points."""
        all_entry_points = []

        for component in components:
            all_entry_points.extend(component.entry_points)

        # Remove duplicates while preserving order
        unique_entry_points = []
        seen = set()
        for entry_point in all_entry_points:
            if entry_point not in seen:
                unique_entry_points.append(entry_point)
                seen.add(entry_point)

        return unique_entry_points

    def _merge_ports(self, components: list[ArchitectureComponent]) -> list[int]:
        """Merge ports - combine all unique ports from all sources."""
        all_ports = []

        for component in components:
            all_ports.extend(component.ports)

        # Remove duplicates and sort
        return sorted(set(all_ports))

    def _merge_protocols(self, components: list[ArchitectureComponent]) -> list[str]:
        """Merge protocols - combine all unique protocols."""
        all_protocols = []

        for component in components:
            all_protocols.extend(component.protocols)

        # Remove duplicates while preserving order
        unique_protocols = []
        seen = set()
        for protocol in all_protocols:
            if protocol not in seen:
                unique_protocols.append(protocol)
                seen.add(protocol)

        return unique_protocols

    def _merge_sources(self, components: list[ArchitectureComponent]) -> list[str]:
        """Merge sources - combine all unique source types."""
        all_sources = []

        for component in components:
            all_sources.extend(component.sources)

        # Remove duplicates while preserving order
        unique_sources = []
        seen = set()
        for source in all_sources:
            if source not in seen:
                unique_sources.append(source)
                seen.add(source)

        return unique_sources
