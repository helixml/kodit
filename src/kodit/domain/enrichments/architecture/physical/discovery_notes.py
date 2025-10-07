"""Physical architecture domain value objects."""

from dataclasses import dataclass


@dataclass
class ArchitectureDiscoveryNotes:
    """Rich, narrative observations about repository architecture for LLM consumption."""  # noqa: E501

    repository_context: str  # High-level overview and discovery scope
    component_observations: list[str]  # Detailed findings about each component
    connection_observations: list[str]  # How components interact and communicate
    infrastructure_observations: list[str]  # Deployment, config, operational patterns
    discovery_metadata: str  # Methodology, confidence, limitations, timestamp
