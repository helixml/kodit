"""Tests for ComponentReconciler."""


from kodit.physical_architecture.domain.value_objects import ArchitectureComponent
from kodit.physical_architecture.infrastructure.architecture_discovery.component_reconciler import (
    ComponentReconciler,
)


class TestComponentReconciler:
    """Test cases for ComponentReconciler."""

    def test_reconcile_no_duplicates(self) -> None:
        """Test reconciling when there are no duplicate components."""
        reconciler = ComponentReconciler()

        components = [
            ArchitectureComponent(
                name="api",
                inferred_role="web_server",
                entry_points=["api/main.go"],
                ports=[8080],
                protocols=["HTTP"],
                sources=["docker-compose"]
            ),
            ArchitectureComponent(
                name="frontend",
                inferred_role="client_app",
                entry_points=["frontend/src/index.tsx"],
                ports=[3000],
                protocols=["HTTP"],
                sources=["docker-compose"]
            )
        ]

        result = reconciler.reconcile(components)

        assert len(result) == 2
        assert {comp.name for comp in result} == {"api", "frontend"}

    def test_reconcile_exact_name_duplicates(self) -> None:
        """Test reconciling components with exact name matches."""
        reconciler = ComponentReconciler()

        # Same component detected by different sources
        components = [
            ArchitectureComponent(
                name="api",
                inferred_role="web_server",
                entry_points=["api/main.go"],
                ports=[8080],
                protocols=["HTTP"],
                sources=["docker-compose"]
            ),
            ArchitectureComponent(
                name="api",
                inferred_role="web_server",
                entry_points=["image: myapp/api:latest"],
                ports=[8080, 80],  # Additional port from k8s
                protocols=["HTTP"],
                sources=["k8s"]
            )
        ]

        result = reconciler.reconcile(components)

        assert len(result) == 1
        merged_component = result[0]
        assert merged_component.name == "api"
        assert merged_component.inferred_role == "web_server"
        assert set(merged_component.ports) == {8080, 80}  # Merged ports
        assert set(merged_component.sources) == {"docker-compose", "k8s"}  # Merged sources
        assert "api/main.go" in merged_component.entry_points
        assert "image: myapp/api:latest" in merged_component.entry_points

    def test_reconcile_case_insensitive_names(self) -> None:
        """Test reconciling components with different case names."""
        reconciler = ComponentReconciler()

        components = [
            ArchitectureComponent(
                name="API",
                inferred_role="web_server",
                entry_points=[],
                ports=[8080],
                protocols=["HTTP"],
                sources=["docker-compose"]
            ),
            ArchitectureComponent(
                name="api",
                inferred_role="web_server",
                entry_points=[],
                ports=[80],
                protocols=["HTTP"],
                sources=["k8s"]
            )
        ]

        result = reconciler.reconcile(components)

        assert len(result) == 1
        merged_component = result[0]
        # Should prefer docker-compose name (first in priority)
        assert merged_component.name == "API"
        assert set(merged_component.ports) == {8080, 80}
        assert set(merged_component.sources) == {"docker-compose", "k8s"}

    def test_reconcile_role_conflict_resolution(self) -> None:
        """Test role conflict resolution using priority and majority voting."""
        reconciler = ComponentReconciler()

        components = [
            ArchitectureComponent(
                name="service",
                inferred_role="web_server",
                entry_points=[],
                ports=[8080],
                protocols=["HTTP"],
                sources=["docker-compose"]  # Infrastructure source, higher weight
            ),
            ArchitectureComponent(
                name="service",
                inferred_role="unknown",
                entry_points=[],
                ports=[8080],
                protocols=["HTTP"],
                sources=["code"]  # Lower weight
            )
        ]

        result = reconciler.reconcile(components)

        assert len(result) == 1
        # Should prefer role from infrastructure source
        assert result[0].inferred_role == "web_server"

    def test_reconcile_empty_list(self) -> None:
        """Test reconciling an empty list of components."""
        reconciler = ComponentReconciler()

        result = reconciler.reconcile([])

        assert len(result) == 0

    def test_reconcile_complex_merge(self) -> None:
        """Test reconciling multiple components with complex overlaps."""
        reconciler = ComponentReconciler()

        components = [
            # First detection of 'api' from docker-compose
            ArchitectureComponent(
                name="api",
                inferred_role="web_server",
                entry_points=["./api/Dockerfile"],
                ports=[8080],
                protocols=["HTTP"],
                sources=["docker-compose"]
            ),
            # Second detection of 'api' from k8s
            ArchitectureComponent(
                name="api",
                inferred_role="web_server",
                entry_points=["image: myapp/api:latest"],
                ports=[80],
                protocols=["HTTP"],
                sources=["k8s"]
            ),
            # Unique 'postgres' component
            ArchitectureComponent(
                name="postgres",
                inferred_role="database",
                entry_points=[],
                ports=[5432],
                protocols=["TCP"],
                sources=["docker-compose"]
            )
        ]

        result = reconciler.reconcile(components)

        assert len(result) == 2
        component_names = {comp.name for comp in result}
        assert component_names == {"api", "postgres"}

        # Check the merged 'api' component
        api_component = next(comp for comp in result if comp.name == "api")
        assert set(api_component.ports) == {8080, 80}
        assert set(api_component.sources) == {"docker-compose", "k8s"}
        assert len(api_component.entry_points) == 2
