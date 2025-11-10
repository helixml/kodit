"""Bundle of domain services for business logic."""

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from kodit.domain.enrichments.enricher import Enricher
    from kodit.domain.services.bm25_service import BM25DomainService
    from kodit.domain.services.cookbook_context_service import (
        CookbookContextService,
    )
    from kodit.domain.services.embedding_service import EmbeddingDomainService
    from kodit.domain.services.git_repository_service import (
        GitRepositoryScanner,
        RepositoryCloner,
    )
    from kodit.domain.services.physical_architecture_service import (
        PhysicalArchitectureService,
    )
    from kodit.infrastructure.database_schema.database_schema_detector import (
        DatabaseSchemaDetector,
    )
    from kodit.infrastructure.slicing.slicer import Slicer


class DomainServices:
    """Bundles all domain services for business logic.

    This is a Parameter Object pattern to reduce constructor complexity.
    """

    def __init__(  # noqa: PLR0913
        self,
        scanner: "GitRepositoryScanner",
        cloner: "RepositoryCloner",
        slicer: "Slicer",
        bm25_service: "BM25DomainService",
        code_search_service: "EmbeddingDomainService",
        text_search_service: "EmbeddingDomainService",
        architecture_service: "PhysicalArchitectureService",
        cookbook_context_service: "CookbookContextService",
        database_schema_detector: "DatabaseSchemaDetector",
        enricher_service: "Enricher",
    ) -> None:
        """Initialize domain services bundle."""
        self.scanner = scanner
        self.cloner = cloner
        self.slicer = slicer
        self.bm25 = bm25_service
        self.code_search = code_search_service
        self.text_search = text_search_service
        self.architecture = architecture_service
        self.cookbook_context = cookbook_context_service
        self.database_schema_detector = database_schema_detector
        self.enricher = enricher_service
