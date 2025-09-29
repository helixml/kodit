"""Kubernetes detector for architecture discovery."""

from pathlib import Path
from typing import Any

import yaml

from kodit.physical_architecture.domain.value_objects import (
    ArchitectureComponent,
    InferredRole,
)


class KubernetesDetector:
    """Detector for extracting components from Kubernetes manifests."""

    def detect(self, repo_path: Path) -> list[ArchitectureComponent]:
        """Detect architecture components from Kubernetes manifests."""
        components = []

        # Find all kubernetes manifest files
        k8s_files = self._find_k8s_files(repo_path)

        # Parse all found files and extract components
        for k8s_file in k8s_files:
            components.extend(self._parse_k8s_file(k8s_file))

        # Deduplicate components by name (basic deduplication before reconciler)
        return self._deduplicate_by_name(components)

    def _find_k8s_files(self, repo_path: Path) -> list[Path]:
        """Find all Kubernetes manifest files in the repository."""
        k8s_files = []

        # Look in common Kubernetes directories
        k8s_dirs = [
            "k8s", "kubernetes", "manifests", "deploy", "deployment",
            ".k8s", "kube", "charts", "helm"
        ]

        # Search in root and common directories
        search_paths = [repo_path]
        for k8s_dir in k8s_dirs:
            potential_dir = repo_path / k8s_dir
            if potential_dir.exists() and potential_dir.is_dir():
                search_paths.append(potential_dir)

        # Find YAML files in search paths
        for search_path in search_paths:
            k8s_files.extend(search_path.rglob("*.yaml"))
            k8s_files.extend(search_path.rglob("*.yml"))

        # Filter files that look like Kubernetes manifests
        return [f for f in k8s_files if self._is_k8s_manifest(f)]

    def _is_k8s_manifest(self, file_path: Path) -> bool:
        """Check if a YAML file looks like a Kubernetes manifest."""
        try:
            with open(file_path, encoding="utf-8") as file:
                content = file.read(1024)  # Read first 1KB to check
                # Simple heuristic: contains common Kubernetes fields
                k8s_indicators = [
                    "apiVersion:", "kind:", "metadata:",
                    "kind: Deployment", "kind: Service", "kind: Pod"
                ]
                return any(indicator in content for indicator in k8s_indicators)
        except (OSError, UnicodeDecodeError):
            return False

    def _parse_k8s_file(self, k8s_file: Path) -> list[ArchitectureComponent]:
        """Parse a single Kubernetes manifest file and extract components."""
        components = []

        try:
            with open(k8s_file, encoding="utf-8") as file:
                # Handle both single documents and multi-document YAML files
                documents = yaml.safe_load_all(file)

                for doc in documents:
                    if doc and isinstance(doc, dict):
                        component = self._extract_component_from_resource(doc)
                        if component:
                            components.append(component)

        except (OSError, yaml.YAMLError):
            # If we can't parse the file, skip it silently
            pass

        return components

    def _extract_component_from_resource(self, resource: dict[str, Any]) -> ArchitectureComponent | None:
        """Extract a component from a Kubernetes resource."""
        kind = resource.get("kind", "")

        # We're primarily interested in Deployments and Services
        if kind not in ["Deployment", "Service", "StatefulSet", "DaemonSet"]:
            return None

        metadata = resource.get("metadata", {})
        name = metadata.get("name")

        if not name:
            return None

        # Extract ports and protocols
        ports, protocols = self._extract_ports_and_protocols(resource)

        # Infer role based on resource type and name
        inferred_role = self._infer_role_from_k8s_resource(name, kind, resource)

        # Extract entry points (limited info available from K8s manifests)
        entry_points = self._extract_entry_points_from_k8s(resource)

        return ArchitectureComponent(
            name=name,
            inferred_role=inferred_role,
            entry_points=entry_points,
            ports=ports,
            protocols=protocols,
            sources=["k8s"]
        )

    def _extract_ports_and_protocols(self, resource: dict[str, Any]) -> tuple[list[int], list[str]]:
        """Extract ports and protocols from Kubernetes resource."""
        ports = []
        protocols = set()

        kind = resource.get("kind", "")

        if kind == "Service":
            # Extract ports from Service spec
            spec = resource.get("spec", {})
            service_ports = spec.get("ports", [])

            for port_spec in service_ports:
                if isinstance(port_spec, dict):
                    # Extract port number
                    port = port_spec.get("port") or port_spec.get("targetPort")
                    if port and isinstance(port, int):
                        ports.append(port)

                    # Extract protocol
                    protocol = port_spec.get("protocol", "TCP").upper()
                    protocols.add(protocol)

        elif kind in ["Deployment", "StatefulSet", "DaemonSet"]:
            # Extract ports from container specs
            spec = resource.get("spec", {})
            template = spec.get("template", {})
            pod_spec = template.get("spec", {})
            containers = pod_spec.get("containers", [])

            for container in containers:
                container_ports = container.get("ports", [])
                for port_spec in container_ports:
                    if isinstance(port_spec, dict):
                        port = port_spec.get("containerPort")
                        if port and isinstance(port, int):
                            ports.append(port)

                        protocol = port_spec.get("protocol", "TCP").upper()
                        protocols.add(protocol)

        # Convert common protocols to more specific ones based on ports
        final_protocols = []
        for protocol in protocols:
            if protocol == "TCP":
                # Check if any ports suggest HTTP
                if any(port in [80, 8080, 3000, 4000, 5000, 8000, 9000] for port in ports):
                    final_protocols.append("HTTP")
                else:
                    final_protocols.append("TCP")
            else:
                final_protocols.append(protocol)

        if not final_protocols:
            final_protocols.append("TCP")  # Default

        return sorted(set(ports)), final_protocols

    def _infer_role_from_k8s_resource(self, name: str, kind: str, resource: dict[str, Any]) -> str:
        """Infer component role from Kubernetes resource."""
        name_lower = name.lower()

        # Database services
        if any(db_name in name_lower for db_name in
               ["postgres", "mysql", "mongodb", "db", "database"]):
            return InferredRole.DATABASE.value

        # Cache services
        if any(cache_name in name_lower for cache_name in
               ["redis", "memcached", "cache"]):
            return InferredRole.CACHE.value

        # Message queue services
        if any(queue_name in name_lower for queue_name in
               ["rabbitmq", "kafka", "queue", "broker"]):
            return InferredRole.MESSAGE_QUEUE.value

        # Frontend/client applications
        if any(frontend_name in name_lower for frontend_name in
               ["frontend", "client", "ui", "web", "app"]):
            return InferredRole.CLIENT_APP.value

        # Worker services - especially for DaemonSet and some Deployments
        if (kind in ["DaemonSet"] or
            any(worker_name in name_lower for worker_name in
                ["worker", "job", "task", "processor"])):
            return InferredRole.BACKGROUND_WORKER.value

        # API/Backend services
        if any(api_name in name_lower for api_name in
               ["api", "backend", "server", "service"]):
            return InferredRole.WEB_SERVER.value

        # Default to web server for Deployments and Services with common web ports
        ports, _ = self._extract_ports_and_protocols(resource)
        if any(port in [80, 8080, 3000, 4000, 5000, 8000, 9000] for port in ports):
            return InferredRole.WEB_SERVER.value

        return InferredRole.UNKNOWN.value

    def _extract_entry_points_from_k8s(self, resource: dict[str, Any]) -> list[str]:
        """Extract entry points from Kubernetes resource (limited information)."""
        entry_points = []
        kind = resource.get("kind", "")

        if kind in ["Deployment", "StatefulSet", "DaemonSet"]:
            spec = resource.get("spec", {})
            template = spec.get("template", {})
            pod_spec = template.get("spec", {})
            containers = pod_spec.get("containers", [])

            for container in containers:
                # Add image as a form of entry point reference
                image = container.get("image")
                if image:
                    entry_points.append(f"image: {image}")

                # Add command if specified
                command = container.get("command")
                if command and isinstance(command, list):
                    entry_points.append(f"command: {' '.join(command)}")

        return entry_points

    def _deduplicate_by_name(self, components: list[ArchitectureComponent]) -> list[ArchitectureComponent]:
        """Simple deduplication by exact name match."""
        seen_names = set()
        deduplicated = []

        for component in components:
            if component.name not in seen_names:
                deduplicated.append(component)
                seen_names.add(component.name)

        return deduplicated
