"""Docker Compose detector for physical architecture discovery."""

import contextlib
from pathlib import Path

import yaml


class DockerComposeDetector:
    """Detects physical components from Docker Compose files and generates narrative observations."""  # noqa: E501

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
                    infrastructure_notes
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
        infrastructure_notes: list[str]
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
                infrastructure_notes
            )

        # Analyze service dependencies
        self._analyze_service_dependencies(
            services,
            connection_notes
        )

        # Check for additional Docker Compose features
        self._analyze_compose_features(
            compose_data,
            infrastructure_notes
        )

    def _analyze_service(
        self,
        service_name: str,
        service_config: dict,
        component_notes: list[str],
        _connection_notes: list[str],
        infrastructure_notes: list[str]
    ) -> None:
        """Generate narrative observations for a single service."""
        # Extract key configuration details
        image = service_config.get("image", "")
        build = service_config.get("build", "")
        ports = self._extract_ports(service_config)
        volumes = service_config.get("volumes", [])
        environment = service_config.get("environment", {})

        # Infer service role and generate component observation
        role_description = self._infer_service_role_description(
            service_name, service_config
        )

        component_observation = (
            f"Found '{service_name}' service in Docker Compose configuration "
            f"{role_description}"
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

        # Add infrastructure observations for advanced features
        if volumes:
            infrastructure_notes.append(
                f"Service '{service_name}' configures {len(volumes)} volume mounts, "
                "indicating persistent data storage requirements and stateful operation."  # noqa: E501
            )

        if environment:
            infrastructure_notes.append(
                f"Service '{service_name}' defines {len(environment)} environment variables, "  # noqa: E501
                "suggesting configuration management through environment injection."
            )

    def _analyze_service_dependencies(
        self,
        services: dict,
        connection_notes: list[str]
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

    def _analyze_compose_features(
        self,
        compose_data: dict,
        infrastructure_notes: list[str]
    ) -> None:
        """Analyze additional Docker Compose features."""
        # Check for networks
        networks = compose_data.get("networks", {})
        if networks:
            infrastructure_notes.append(
                f"Docker Compose defines {len(networks)} custom networks, "
                "indicating network segmentation and controlled service communication."
            )

        # Check for volumes
        volumes = compose_data.get("volumes", {})
        if volumes:
            infrastructure_notes.append(
                f"Docker Compose defines {len(volumes)} named volumes, "
                "suggesting shared persistent storage across container restarts."
            )

        # Check for secrets
        secrets = compose_data.get("secrets", {})
        if secrets:
            infrastructure_notes.append(
                f"Docker Compose defines {len(secrets)} secrets, "
                "indicating secure credential management for production deployment."
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

    def _infer_service_role_description(  # noqa: PLR0911
        self, service_name: str, service_config: dict
    ) -> str:
        """Infer service role and return descriptive text."""
        name_lower = service_name.lower()
        image = service_config.get("image", "").lower()
        ports = self._extract_ports(service_config)

        # Database services
        db_indicators = ["postgres", "mysql", "mongodb", "mongo", "mariadb", "database", "db"]  # noqa: E501
        if any(indicator in name_lower for indicator in db_indicators) or \
           any(indicator in image for indicator in db_indicators):
            return ("configured as a database service. Database configuration "
                   "suggests this handles persistent data storage for the application")

        # Cache services
        cache_indicators = ["redis", "memcached", "cache"]
        if any(indicator in name_lower for indicator in cache_indicators) or \
           any(indicator in image for indicator in cache_indicators):
            return ("configured as a caching service. Cache configuration "
                   "indicates performance optimization through data caching")

        # Message queue services
        mq_indicators = ["rabbitmq", "kafka", "activemq", "queue", "broker"]
        if any(indicator in name_lower for indicator in mq_indicators) or \
           any(indicator in image for indicator in mq_indicators):
            return ("configured as a message queue service. Queue configuration "
                   "suggests asynchronous communication and task processing")

        # Worker services
        worker_indicators = ["worker", "job", "task", "celery"]
        if any(indicator in name_lower for indicator in worker_indicators):
            return ("configured as a background worker service. Worker configuration "
                   "indicates asynchronous task processing and job execution")

        # Frontend services
        frontend_indicators = ["frontend", "client", "ui", "web", "app"]
        static_servers = ["nginx", "apache", "caddy"]
        if any(indicator in name_lower for indicator in frontend_indicators) and \
           any(server in image for server in static_servers):
            return ("configured as a frontend application service. Static server "
                   "configuration suggests serving of web assets and user interface")

        # API/Backend services
        backend_indicators = ["api", "backend", "server", "service"]
        if any(indicator in name_lower for indicator in backend_indicators):
            return ("configured as a backend API service. Service configuration "
                   "suggests handling of business logic and data processing")

        # Default based on ports
        if ports:
            return ("configured as a web service exposing network ports. "
                   "Port configuration indicates external accessibility")
        return ("configured as an internal service. No exposed ports "
               "suggest internal-only operation within the container network")

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

        # Database ports
        db_ports = {5432, 3306, 27017, 6379}
        if any(port in db_ports for port in ports):
            protocols.append("database connectivity")

        if protocols:
            return " and ".join(protocols)
        if ports:
            return "TCP-based service communication"
        return ""
