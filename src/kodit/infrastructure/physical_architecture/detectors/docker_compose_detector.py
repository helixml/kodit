"""Docker Compose detector for physical architecture discovery."""

import contextlib
import re
from pathlib import Path

import yaml


class DockerComposeDetector:
    """Detects physical components from Docker Compose files and generates narrative observations."""  # noqa: E501

    # Regex pattern to detect communication addresses in environment variables
    # Matches complete URLs with hostnames:
    # - Simple URLs: http://api:8080, redis://cache:6379
    # - Connection strings with auth: postgresql://user:pass@db:5432/dbname
    # - Connection strings with asyncpg: postgresql+asyncpg://user:pass@db:5432
    # Note: This captures the hostname portion, avoiding false matches in
    # passwords or other parts of the URL
    COMMUNICATION_PATTERN = re.compile(
        r"(?:"
        # Protocol-based URLs with optional auth (user:pass@)
        r"(?:https?|tcp|grpc|ws|wss|amqp|kafka|redis|memcached|"
        r"postgres(?:ql)?(?:\+\w+)?|mysql|mongodb)://"
        r"(?:[^@/]+@)?"  # Optional user:pass@ (non-capturing, skip it)
        r"([\w\-\.]+(?::\d+)?)"  # Capture hostname:port after @ or ://
        r")",
        re.IGNORECASE,
    )

    async def analyze(self, repo_path: Path) -> tuple[list[str], list[str], list[str]]:
        """Generate narrative observations from Docker Compose analysis."""
        component_notes: list[str] = []
        connection_notes: list[str] = []
        infrastructure_notes: list[str] = []

        # Find all docker-compose files
        yml_files = list(repo_path.glob("docker-compose*.yml"))
        yaml_files = list(repo_path.glob("docker-compose*.yaml"))
        compose_files = yml_files + yaml_files

        if not compose_files:
            return ([], [], [])

        # Analyze each compose file
        for compose_file in compose_files:
            try:
                with compose_file.open(encoding="utf-8") as f:
                    compose_data = yaml.safe_load(f)

                if not compose_data or "services" not in compose_data:
                    continue

                self._analyze_compose_file(
                    compose_file,
                    compose_data,
                    component_notes,
                    connection_notes,
                    infrastructure_notes,
                )

            except (yaml.YAMLError, OSError, KeyError):
                infrastructure_notes.append(
                    f"Unable to parse Docker Compose file at {compose_file}. "
                    "File may be malformed or inaccessible."
                )

        return (component_notes, connection_notes, infrastructure_notes)

    def _analyze_compose_file(
        self,
        compose_file: Path,
        compose_data: dict,
        component_notes: list[str],
        connection_notes: list[str],
        infrastructure_notes: list[str],
    ) -> None:
        """Analyze a single Docker Compose file and generate observations."""
        services = compose_data.get("services", {})

        # High-level infrastructure observation
        infrastructure_notes.append(
            f"Found Docker Compose configuration at {compose_file.name} defining "
            f"{len(services)} services. This suggests a containerized application "
            f"architecture with orchestrated service dependencies."
        )

        # Analyze each service
        for service_name, service_config in services.items():
            self._analyze_service(
                service_name,
                service_config,
                component_notes,
                connection_notes,
            )

        # Analyze service dependencies
        self._analyze_service_dependencies(services, connection_notes)

        # Check for additional Docker Compose features
        self._analyze_compose_features(compose_data, infrastructure_notes)

    def _analyze_service(
        self,
        service_name: str,
        service_config: dict,
        component_notes: list[str],
        _connection_notes: list[str],
    ) -> None:
        """Generate narrative observations for a single service."""
        # Extract key configuration details
        image = service_config.get("image", "")
        build = service_config.get("build", "")
        ports = self._extract_ports(service_config)

        component_observation = (
            f"Found '{service_name}' service in Docker Compose configuration."
        )

        # Add deployment details
        if image:
            component_observation += f" Service uses '{image}' Docker image"
            if ":" in image:
                tag = image.split(":")[-1]
                component_observation += f" with tag '{tag}'"
            component_observation += "."
        elif build:
            component_observation += f" Service builds from local source at '{build}'."

        # Add port information
        if ports:
            port_list = ", ".join(str(p) for p in ports)
            component_observation += f" Exposes ports {port_list}"
            protocol_info = self._infer_protocol_description(ports)
            if protocol_info:
                component_observation += f" suggesting {protocol_info}"
            component_observation += "."

        component_notes.append(component_observation)

    def _analyze_service_dependencies(  # noqa: PLR0912, C901
        self, services: dict, connection_notes: list[str]
    ) -> None:
        """Analyze dependencies between services."""
        for service_name, service_config in services.items():
            depends_on = service_config.get("depends_on", [])

            if isinstance(depends_on, dict):
                dependencies = list(depends_on.keys())
                condition_info = []
                for dep, condition in depends_on.items():
                    if isinstance(condition, dict) and "condition" in condition:
                        condition_info.append(f"{dep} ({condition['condition']})")

                if condition_info:
                    connection_notes.append(
                        f"Service '{service_name}' has conditional dependencies on "
                        f"{', '.join(condition_info)}, indicating sophisticated "
                        "startup orchestration with health checks."
                    )
                else:
                    dependencies = list(depends_on.keys())
            elif isinstance(depends_on, list):
                dependencies = depends_on
            else:
                continue

            if dependencies:
                dep_list = "', '".join(dependencies)
                connection_notes.append(
                    f"Docker Compose 'depends_on' configuration shows '{service_name}' "
                    f"requires '{dep_list}' to start first, indicating service startup "
                    "dependency and likely runtime communication pattern."
                )

        # Check for communication patterns in environment variables
        # and command arguments
        service_names = {name for name, _ in services.items()}
        # Track which connections we've already recorded to avoid duplicates
        recorded_connections: set[tuple[str, str]] = set()

        for service_name, service_config in services.items():
            # Check environment variables
            env = service_config.get("environment", [])
            if isinstance(env, list):
                for var in env:
                    self._check_communication_pattern(
                        var,
                        service_name,
                        service_names,
                        "environment variable",
                        connection_notes,
                        recorded_connections,
                    )
            elif isinstance(env, dict):
                for value in env.values():
                    if isinstance(value, str):
                        self._check_communication_pattern(
                            value,
                            service_name,
                            service_names,
                            "environment variable",
                            connection_notes,
                            recorded_connections,
                        )

            # Check command arguments
            args = service_config.get("command", [])
            if isinstance(args, list):
                for arg in args:
                    if isinstance(arg, str):
                        self._check_communication_pattern(
                            arg,
                            service_name,
                            service_names,
                            "command argument",
                            connection_notes,
                            recorded_connections,
                        )
            elif isinstance(args, str):
                self._check_communication_pattern(
                    args,
                    service_name,
                    service_names,
                    "command argument",
                    connection_notes,
                    recorded_connections,
                )

    def _check_communication_pattern(  # noqa: PLR0913
        self,
        text: str,
        service_name: str,
        service_names: set[str],
        source_type: str,
        connection_notes: list[str],
        recorded_connections: set[tuple[str, str]],
    ) -> None:
        """Check if text contains communication patterns referencing other services."""
        # Find all matches and extract hostnames from captured groups
        matches = self.COMMUNICATION_PATTERN.finditer(text)
        hostnames = set()

        for match in matches:
            # Group 1 contains the hostname
            if match.group(1):
                # Extract just the hostname (without port)
                hostname = match.group(1).split(":")[0]
                hostnames.add(hostname)

        if not hostnames:
            return

        # Check if any extracted hostname matches a service name
        for target_service in service_names:
            if target_service == service_name:
                continue

            # Check if the target service is in the extracted hostnames
            if target_service in hostnames:
                connection_key = (service_name, target_service)
                if connection_key not in recorded_connections:
                    connection_notes.append(
                        f"'{service_name}' has a communication address referencing "
                        f"'{target_service}' in its {source_type}, indicating a "
                        "direct runtime dependency."
                    )
                    recorded_connections.add(connection_key)
                    break

    def _analyze_compose_features(
        self, compose_data: dict, infrastructure_notes: list[str]
    ) -> None:
        """Analyze additional Docker Compose features."""
        # Check for networks
        networks = compose_data.get("networks", {})
        if networks:
            infrastructure_notes.append(
                f"Docker Compose defines {len(networks)} custom networks, "
                "indicating network segmentation and controlled service communication."
            )

    def _extract_ports(self, service_config: dict) -> list[int]:
        """Extract port numbers from service configuration."""
        ports = []

        # Extract from 'ports' section
        port_specs = service_config.get("ports", [])
        for port_spec in port_specs:
            if isinstance(port_spec, str):
                if ":" in port_spec:
                    external_port = port_spec.split(":")[0]
                    with contextlib.suppress(ValueError):
                        ports.append(int(external_port))
                else:
                    with contextlib.suppress(ValueError):
                        ports.append(int(port_spec))
            elif isinstance(port_spec, int):
                ports.append(port_spec)

        # Extract from 'expose' section
        expose_specs = service_config.get("expose", [])
        for expose_spec in expose_specs:
            with contextlib.suppress(ValueError, TypeError):
                ports.append(int(expose_spec))

        return sorted(set(ports))

    def _infer_protocol_description(self, ports: list[int]) -> str:
        """Infer protocol information from ports and return descriptive text."""
        protocols = []

        # HTTP ports
        http_ports = {80, 8080, 3000, 4200, 5000, 8000, 8443, 443}
        if any(port in http_ports for port in ports):
            protocols.append("HTTP/HTTPS web traffic")

        # gRPC ports
        grpc_ports = {9090, 50051}
        if any(port in grpc_ports for port in ports):
            protocols.append("gRPC API communication")

        # Cache/Redis ports
        if 6379 in ports:
            protocols.append("cache service")

        # Database ports (excluding Redis which is handled above)
        db_ports = {5432, 3306, 27017}
        if any(port in db_ports for port in ports):
            protocols.append("database service")

        if protocols:
            return " and ".join(protocols)
        if ports:
            return "TCP-based service communication"
        return ""
