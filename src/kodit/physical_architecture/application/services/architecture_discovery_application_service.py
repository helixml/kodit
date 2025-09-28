"""Application service for physical architecture discovery integration."""

from pathlib import Path

from kodit.domain.value_objects import Enrichment, EnrichmentType
from kodit.physical_architecture.domain.services.physical_architecture_service import (
    PhysicalArchitectureService,
)


class PhysicalArchitectureDiscoveryApplicationService:
    """Application service for integrating physical architecture discovery with kodit."""
    
    def __init__(self):
        """Initialize the application service."""
        self.architecture_service = PhysicalArchitectureService()
    
    async def discover_and_create_enrichment(self, repo_path: Path) -> Enrichment:
        """Discover physical architecture and create an enrichment for storage.
        
        Args:
            repo_path: Path to the repository to analyze
            
        Returns:
            An enrichment containing the physical architecture as JSON
        """
        # Discover the physical architecture
        architecture = await self.architecture_service.discover_architecture(repo_path)
        
        # Convert to JSON for storage
        architecture_json = self.architecture_service.to_json(architecture)
        
        # Create an enrichment to be stored in the database
        return Enrichment(
            type=EnrichmentType.PHYSICAL_ARCHITECTURE,  # We'll need to add this enum value
            content=architecture_json
        )
    
    async def get_architecture_from_enrichment(self, enrichment: Enrichment):
        """Extract physical architecture from an enrichment.
        
        Args:
            enrichment: The enrichment containing architecture JSON
            
        Returns:
            PhysicalArchitecture object
            
        Raises:
            ValueError: If the enrichment is not a physical architecture type
        """
        if enrichment.type != EnrichmentType.PHYSICAL_ARCHITECTURE:
            raise ValueError(f"Expected enrichment type PHYSICAL_ARCHITECTURE, got {enrichment.type}")
        
        # Deserialize from JSON
        return self.architecture_service.from_json(enrichment.content)
    
    async def rediscover_architecture(self, repo_path: Path) -> Enrichment:
        """Rediscover physical architecture (useful for updates).
        
        Args:
            repo_path: Path to the repository to analyze
            
        Returns:
            Updated enrichment containing the physical architecture
        """
        # This is the same as discover_and_create_enrichment for now
        # In the future, we could add logic to compare with existing architecture
        return await self.discover_and_create_enrichment(repo_path)