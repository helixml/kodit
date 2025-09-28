"""Core domain service for discovering physical architecture."""

from datetime import datetime
from pathlib import Path

from kodit.physical_architecture.domain.value_objects import (
    ArchitectureComponent,
    ComponentConnection,
    PhysicalArchitecture,
)
from kodit.physical_architecture.infrastructure.architecture_discovery.component_reconciler import (
    ComponentReconciler,
)
from kodit.physical_architecture.infrastructure.architecture_discovery.docker_compose_detector import (
    DockerComposeDetector,
)
from kodit.physical_architecture.infrastructure.architecture_discovery.kubernetes_detector import (
    KubernetesDetector,
)


class PhysicalArchitectureService:
    """Core service for discovering physical architecture."""
    
    def __init__(self):
        """Initialize the service with detectors and reconcilers."""
        self.docker_detector = DockerComposeDetector()
        self.k8s_detector = KubernetesDetector()
        self.component_reconciler = ComponentReconciler()
    
    async def discover_architecture(self, repo_path: Path) -> PhysicalArchitecture:
        """Discover physical architecture - run all detectors and reconcile results."""
        
        # Run all detectors (some may return empty lists)
        docker_components = self.docker_detector.detect(repo_path)
        k8s_components = self.k8s_detector.detect(repo_path)
        
        # Combine all detected components into one list
        all_detected_components = [
            *docker_components,
            *k8s_components,
        ]
        
        # Reconcile/clean the combined list
        components = self.component_reconciler.reconcile(all_detected_components)
        
        # Discover connections based on reconciled components
        connections = await self._discover_connections(repo_path, components)
        
        return PhysicalArchitecture(
            components=components,
            connections=connections,
            discovery_timestamp=datetime.utcnow().isoformat()
        )
    
    async def _discover_connections(
        self,
        repo_path: Path,
        components: list[ArchitectureComponent]
    ) -> list[ComponentConnection]:
        """Find how components connect to each other."""
        # Discover connections from Docker Compose dependencies
        docker_connections = self.docker_detector.detect_connections(repo_path)
        
        # For now, only return Docker Compose connections
        # In the future, we could add Kubernetes service connections, code analysis, etc.
        return docker_connections
    
    def to_json(self, architecture: PhysicalArchitecture) -> str:
        """Serialize architecture for storage as enrichment blob."""
        return architecture.to_json()
    
    def from_json(self, json_str: str) -> PhysicalArchitecture:
        """Deserialize architecture from enrichment blob."""
        return PhysicalArchitecture.from_json(json_str)