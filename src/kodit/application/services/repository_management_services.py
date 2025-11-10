"""Bundle of repository management services."""

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from kodit.application.services.repository_deletion_service import (
        RepositoryDeletionService,
    )
    from kodit.application.services.repository_lifecycle_service import (
        RepositoryLifecycleService,
    )
    from kodit.application.services.repository_query_service import (
        RepositoryQueryService,
    )


class RepositoryManagementServices:
    """Bundles repository management services.

    This is a Parameter Object pattern to reduce constructor complexity.
    """

    def __init__(
        self,
        lifecycle: "RepositoryLifecycleService",
        deletion: "RepositoryDeletionService",
        query: "RepositoryQueryService",
    ) -> None:
        """Initialize repository management services bundle."""
        self.lifecycle = lifecycle
        self.deletion = deletion
        self.query = query
