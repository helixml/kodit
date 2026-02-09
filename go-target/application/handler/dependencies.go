package handler

import (
	"log/slog"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/infrastructure/git"
)

// EnrichmentContext holds the stores and services shared by all enrichment handlers.
type EnrichmentContext struct {
	Enrichments  enrichment.EnrichmentStore
	Associations enrichment.AssociationStore
	Query        *service.EnrichmentQuery
	Enricher     domainservice.Enricher // nil if no text provider configured
	Tracker      TrackerFactory
	Logger       *slog.Logger
}

// VectorIndex pairs an embedding domain service with its backing vector store.
type VectorIndex struct {
	Embedding domainservice.Embedding
	Store     search.VectorStore
}

// RepositoryStores groups the persistence stores for repository-related entities.
type RepositoryStores struct {
	Repositories repository.RepositoryStore
	Commits      repository.CommitStore
	Branches     repository.BranchStore
	Tags         repository.TagStore
	Files        repository.FileStore
}

// GitInfrastructure provides git cloning, updating, and scanning operations.
type GitInfrastructure struct {
	Adapter git.Adapter
	Cloner  domainservice.Cloner
	Scanner domainservice.Scanner
}
