"""Docker Compose detector for architecture discovery."""

import contextlib
import re
from pathlib import Path

import yaml

from kodit.physical_architecture.domain.value_objects import (
    ArchitectureComponent,
    ComponentConnection,
    ConnectionDirection,
    InferredRole,
)


class DockerComposeDetector:
    """Detector for extracting components from Docker Compose files."""

    def detect(self, repo_path: Path) -> list[ArchitectureComponent]:
        """Detect architecture components from Docker Compose files."""
        components = []

        # Find all docker-compose files
        compose_files = self._find_compose_files(repo_path)

        for compose_file in compose_files:
            components.extend(self._parse_compose_file(compose_file))

        return components

    def detect_connections(self, repo_path: Path) -> list[ComponentConnection]:
        """Detect connections between components from Docker Compose files."""
        connections = []

        # Find all docker-compose files
        compose_files = self._find_compose_files(repo_path)

        for compose_file in compose_files:
            connections.extend(self._parse_connections_from_compose_file(compose_file))

        return connections

    def _find_compose_files(self, repo_path: Path) -> list[Path]:
        """Find all docker-compose files in the repository."""
        compose_files = []

        # Common docker-compose file patterns
        patterns = [
            "docker-compose.yml",
            "docker-compose.yaml",
            "docker-compose.*.yml",
            "docker-compose.*.yaml"
        ]

        for pattern in patterns:
            compose_files.extend(repo_path.rglob(pattern))

        return compose_files

    def _parse_compose_file(self, compose_file: Path) -> list[ArchitectureComponent]:
        """Parse a single docker-compose file and extract components."""
        components = []

        try:
            with open(compose_file, encoding="utf-8") as file:
                compose_data = yaml.safe_load(file)

            if not compose_data or "services" not in compose_data:
                return components

            services = compose_data["services"]

            for service_name, service_config in services.items():
                component = self._create_component_from_service(
                    service_name, service_config
                )
                components.append(component)

        except (OSError, yaml.YAMLError, KeyError):
            # If we can't parse the file, skip it silently for now
            # In production, we might want to log this
            pass

        return components

    def _create_component_from_service(
        self,
        service_name: str,
        service_config: dict
    ) -> ArchitectureComponent:
        """Create an ArchitectureComponent from a Docker Compose service."""
        # Extract ports
        ports = self._extract_ports(service_config)

        # Infer role based on service name and configuration
        inferred_role = self._infer_role(service_name, service_config)

        # Extract entry points (if dockerfile or build context available)
        entry_points = self._extract_entry_points(service_config)

        # Determine protocols based on ports and service config
        protocols = self._extract_protocols(service_config, ports)

        return ArchitectureComponent(
            name=service_name,
            inferred_role=inferred_role,
            entry_points=entry_points,
            ports=ports,
            protocols=protocols,
            sources=["docker-compose"]
        )

    def _extract_ports(self, service_config: dict) -> list[int]:
        """Extract port numbers from service configuration."""
        ports = []

        # Check 'ports' configuration
        if "ports" in service_config:
            for port_mapping in service_config["ports"]:
                if isinstance(port_mapping, str):
                    # Handle formats like "8080:8080", "3000", etc.
                    port_match = re.search(r":?(\d+)(?::(\d+))?", port_mapping)
                    if port_match:
                        # Take the external port (first one), or internal if only one
                        external_port = port_match.group(1)
                        if external_port:
                            ports.append(int(external_port))
                elif isinstance(port_mapping, int):
                    ports.append(port_mapping)
                elif isinstance(port_mapping, dict):
                    # Handle object format like {target: 8080, published: "8080"}
                    if "published" in port_mapping:
                        with contextlib.suppress(ValueError, TypeError):
                            ports.append(int(port_mapping["published"]))

        # Check 'expose' configuration
        if "expose" in service_config:
            for exposed_port in service_config["expose"]:
                with contextlib.suppress(ValueError, TypeError):
                    ports.append(int(exposed_port))

        # Remove duplicates and sort
        return sorted(set(ports))

    def _infer_role(self, service_name: str, service_config: dict) -> str:
        """Infer the role of a service based on name and configuration."""
        service_name_lower = service_name.lower()

        # Database services
        if any(db_name in service_name_lower for db_name in
               ["postgres", "mysql", "mongodb", "db", "database"]):
            return InferredRole.DATABASE.value

        # Cache services
        if any(cache_name in service_name_lower for cache_name in
               ["redis", "memcached", "cache"]):
            return InferredRole.CACHE.value

        # Message queue services
        if any(queue_name in service_name_lower for queue_name in
               ["rabbitmq", "kafka", "queue", "broker"]):
            return InferredRole.MESSAGE_QUEUE.value

        # Frontend/client applications
        if any(frontend_name in service_name_lower for frontend_name in
               ["frontend", "client", "ui", "web", "app"]):
            return InferredRole.CLIENT_APP.value

        # Worker services
        if any(worker_name in service_name_lower for worker_name in
               ["worker", "job", "task", "queue"]):
            return InferredRole.BACKGROUND_WORKER.value

        # API/Backend services (check for common web server ports)
        ports = self._extract_ports(service_config)
        if any(port in [80, 8000, 8080, 3000, 4000, 5000, 9000] for port in ports):
            return InferredRole.WEB_SERVER.value

        # API services by name
        if any(api_name in service_name_lower for api_name in
               ["api", "backend", "server"]):
            return InferredRole.WEB_SERVER.value

        return InferredRole.UNKNOWN.value

    def _extract_entry_points(self, service_config: dict) -> list[str]:
        """Extract entry points from service configuration."""
        entry_points = []

        # Check if there's a build context that might indicate entry points
        if "build" in service_config:
            build_config = service_config["build"]
            if isinstance(build_config, str):
                # Simple build path
                entry_points.append(f"{build_config}/Dockerfile")
            elif isinstance(build_config, dict) and "context" in build_config:
                # Build object with context
                context = build_config["context"]
                dockerfile = build_config.get("dockerfile", "Dockerfile")
                entry_points.append(f"{context}/{dockerfile}")

        # Check for command or entrypoint
        if "command" in service_config:
            command = service_config["command"]
            if isinstance(command, list) and command:
                entry_points.append(" ".join(command))
            elif isinstance(command, str):
                entry_points.append(command)

        return entry_points

    def _extract_protocols(self, service_config: dict, ports: list[int]) -> list[str]:
        """Extract protocols based on service configuration."""
        protocols = []

        # Default protocol inference based on common ports
        for port in ports:
            if port in [80, 8080, 3000, 4000, 5000, 8000, 9000]:
                if "HTTP" not in protocols:
                    protocols.append("HTTP")
            elif port == 443:
                if "HTTPS" not in protocols:
                    protocols.append("HTTPS")
            elif "TCP" not in protocols:
                protocols.append("TCP")

        # If no ports found, default to TCP for most services
        if not protocols and not ports:
            protocols.append("TCP")

        return protocols

    def _parse_connections_from_compose_file(self, compose_file: Path) -> list[ComponentConnection]:
        """Parse connections from a single docker-compose file."""
        connections = []

        try:
            with open(compose_file, encoding="utf-8") as file:
                compose_data = yaml.safe_load(file)

            if not compose_data or "services" not in compose_data:
                return connections

            services = compose_data["services"]

            for service_name, service_config in services.items():
                # Extract connections from depends_on
                depends_on = service_config.get("depends_on", [])

                if depends_on:
                    # Handle different depends_on formats
                    if isinstance(depends_on, list):
                        # Simple list format: depends_on: [service1, service2]
                        for dependency in depends_on:
                            connection = self._create_connection(
                                service_name, dependency, service_config
                            )
                            connections.append(connection)

                    elif isinstance(depends_on, dict):
                        # Object format with conditions: depends_on: {service1: {condition: service_healthy}}
                        for dependency in depends_on:
                            connection = self._create_connection(
                                service_name, dependency, service_config
                            )
                            connections.append(connection)

        except (OSError, yaml.YAMLError, KeyError):
            # If we can't parse the file, skip it silently
            pass

        return connections

    def _create_connection(
        self,
        source_service: str,
        target_service: str,
        source_config: dict
    ) -> ComponentConnection:
        """Create a ComponentConnection from docker-compose dependency."""
        # Infer protocol based on target service and source configuration
        protocol = self._infer_connection_protocol(source_config, target_service)

        # Most docker-compose dependencies are client-to-server
        # (the dependent service connects to the service it depends on)
        direction = ConnectionDirection.CLIENT_TO_SERVER.value

        return ComponentConnection(
            source_component=source_service,
            target_component=target_service,
            direction=direction,
            protocol=protocol
        )

    def _infer_connection_protocol(self, source_config: dict, target_service: str) -> str:
        """Infer the protocol used for connection between services."""
        target_lower = target_service.lower()

        # Database connections typically use TCP
        if any(db_name in target_lower for db_name in
               ["postgres", "mysql", "mongodb", "db", "database"]):
            return "TCP"

        # Cache connections
        if any(cache_name in target_lower for cache_name in
               ["redis", "memcached", "cache"]):
            return "TCP"

        # Message queue connections
        if any(queue_name in target_lower for queue_name in
               ["rabbitmq", "kafka", "queue", "broker"]):
            return "TCP"

        # API/Web services likely use HTTP
        if any(web_name in target_lower for web_name in
               ["api", "backend", "server", "frontend", "web"]):
            return "HTTP"

        # Default to TCP for unknown connections
        return "TCP"
