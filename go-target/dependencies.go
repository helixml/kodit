package kodit

import (
	"github.com/helixml/kodit/application/handler"
)

// EnrichmentContext holds the stores and services shared by all enrichment handlers.
type EnrichmentContext = handler.EnrichmentContext

// VectorIndex pairs an embedding domain service with its backing vector store.
type VectorIndex = handler.VectorIndex

// RepositoryStores groups the persistence stores for repository-related entities.
type RepositoryStores = handler.RepositoryStores

// GitInfrastructure provides git cloning, updating, and scanning operations.
type GitInfrastructure = handler.GitInfrastructure
