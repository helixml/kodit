"""Value objects for physical architecture discovery."""

import json
from dataclasses import asdict, dataclass
from datetime import datetime
from enum import Enum
from typing import Any


class InferredRole(Enum):
    """Inferred role of a component based on code patterns."""

    WEB_SERVER = "web_server"
    DATABASE = "database"
    BACKGROUND_WORKER = "background_worker"
    CACHE = "cache"
    MESSAGE_QUEUE = "message_queue"
    CLIENT_APP = "client_app"
    UNKNOWN = "unknown"


class ConnectionDirection(Enum):
    """Direction of connection between components."""

    CLIENT_TO_SERVER = "client_to_server"  # Normal: frontend → backend
    REVERSE_CONNECTION = "reverse_connection"  # Worker → control plane
    BIDIRECTIONAL = "bidirectional"


@dataclass
class ArchitectureComponent:
    """A physical component in the system."""

    name: str  # Extracted from actual project structure (e.g., "api", "frontend", "worker")
    inferred_role: str  # What we think it does (e.g., "web_server", "database", "background_worker")
    entry_points: list[str]  # Main files that start this component
    ports: list[int]
    protocols: list[str]  # HTTP, WebSocket, gRPC, TCP
    sources: list[str]  # Where this component was detected (e.g., ["docker-compose", "k8s", "code"])


@dataclass
class ComponentConnection:
    """How components connect to each other."""

    source_component: str
    target_component: str
    direction: str  # ConnectionDirection value
    protocol: str


@dataclass
class PhysicalArchitecture:
    """Serializable result of architecture discovery - stored as JSON blob."""

    components: list[ArchitectureComponent]
    connections: list[ComponentConnection]
    discovery_timestamp: str  # ISO format datetime string

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "PhysicalArchitecture":
        """Create PhysicalArchitecture from dictionary."""
        components = [ArchitectureComponent(**comp) for comp in data["components"]]
        connections = [ComponentConnection(**conn) for conn in data["connections"]]
        return cls(
            components=components,
            connections=connections,
            discovery_timestamp=data["discovery_timestamp"]
        )

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for JSON serialization."""
        return asdict(self)

    def to_json(self) -> str:
        """Serialize architecture for storage as enrichment blob."""
        return json.dumps(self.to_dict(), indent=2)

    @classmethod
    def from_json(cls, json_str: str) -> "PhysicalArchitecture":
        """Deserialize architecture from enrichment blob."""
        data = json.loads(json_str)
        return cls.from_dict(data)

    @classmethod
    def create_empty(cls) -> "PhysicalArchitecture":
        """Create empty architecture for initialization."""
        return cls(
            components=[],
            connections=[],
            discovery_timestamp=datetime.utcnow().isoformat()
        )
