package kodit

import (
	"log/slog"

	"github.com/helixml/kodit/application/handler"
	commithandler "github.com/helixml/kodit/application/handler/commit"
	enrichmenthandler "github.com/helixml/kodit/application/handler/enrichment"
	indexinghandler "github.com/helixml/kodit/application/handler/indexing"
	repohandler "github.com/helixml/kodit/application/handler/repository"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/tracking"
)

// registerHandlers registers all task handlers with the worker registry.
func (c *Client) registerHandlers() {
	// Repository handlers (always registered)
	c.registry.Register(task.OperationCloneRepository, repohandler.NewClone(
		c.repoStores.Repositories, c.gitInfra.Cloner, c.queue, c.enrichCtx.Tracker, c.logger,
	))
	c.registry.Register(task.OperationSyncRepository, repohandler.NewSync(
		c.repoStores.Repositories, c.repoStores.Branches, c.gitInfra.Cloner, c.gitInfra.Scanner, c.queue, c.enrichCtx.Tracker, c.logger,
	))
	c.registry.Register(task.OperationDeleteRepository, repohandler.NewDelete(
		c.repoStores, c.snippetStore, c.enrichCtx.Tracker, c.logger,
	))
	c.registry.Register(task.OperationScanCommit, commithandler.NewScan(
		c.repoStores.Repositories, c.repoStores.Commits, c.repoStores.Files, c.gitInfra.Scanner, c.enrichCtx.Tracker, c.logger,
	))
	c.registry.Register(task.OperationRescanCommit, commithandler.NewRescan(
		c.snippetStore, c.enrichCtx.Associations, c.enrichCtx.Tracker, c.logger,
	))

	// Indexing handlers (always registered for snippet extraction)
	c.registry.Register(task.OperationExtractSnippetsForCommit, indexinghandler.NewExtractSnippets(
		c.repoStores.Repositories, c.snippetStore, c.repoStores.Files, c.slicer, c.enrichCtx.Tracker, c.logger,
	))

	// BM25 index handler
	c.registry.Register(task.OperationCreateBM25IndexForCommit, indexinghandler.NewCreateBM25Index(
		c.bm25Service, c.snippetStore, c.enrichCtx.Tracker, c.logger,
	))

	// Code embeddings for snippets
	c.registry.Register(task.OperationCreateCodeEmbeddingsForCommit, indexinghandler.NewCreateCodeEmbeddings(
		c.codeIndex, c.snippetStore, c.enrichCtx.Tracker, c.logger,
	))

	// Example code embeddings (enrichment content from extracted examples)
	c.registry.Register(task.OperationCreateExampleCodeEmbeddingsForCommit, indexinghandler.NewCreateExampleCodeEmbeddings(
		c.codeIndex, c.enrichCtx.Query, c.enrichCtx.Tracker, c.logger,
	))

	// Summary embeddings (enrichment content from snippet summaries)
	c.registry.Register(task.OperationCreateSummaryEmbeddingsForCommit, indexinghandler.NewCreateSummaryEmbeddings(
		c.textIndex, c.enrichCtx.Query, c.enrichCtx.Associations, c.enrichCtx.Tracker, c.logger,
	))

	// Example summary embeddings (enrichment content from example summaries)
	c.registry.Register(task.OperationCreateExampleSummaryEmbeddingsForCommit, indexinghandler.NewCreateExampleSummaryEmbeddings(
		c.textIndex, c.enrichCtx.Query, c.enrichCtx.Tracker, c.logger,
	))

	// Enrichment handlers
	// Summary enrichment
	c.registry.Register(task.OperationCreateSummaryEnrichmentForCommit, enrichmenthandler.NewCreateSummary(
		c.snippetStore, c.enrichCtx,
	))

	// Commit description
	c.registry.Register(task.OperationCreateCommitDescriptionForCommit, enrichmenthandler.NewCommitDescription(
		c.repoStores.Repositories, c.enrichCtx, c.gitInfra.Adapter,
	))

	// Architecture discovery
	c.registry.Register(task.OperationCreateArchitectureEnrichmentForCommit, enrichmenthandler.NewArchitectureDiscovery(
		c.repoStores.Repositories, c.enrichCtx, c.archDiscoverer,
	))

	// Example summary
	c.registry.Register(task.OperationCreateExampleSummaryForCommit, enrichmenthandler.NewExampleSummary(
		c.enrichCtx,
	))

	// Database schema enrichment
	c.registry.Register(task.OperationCreateDatabaseSchemaForCommit, enrichmenthandler.NewDatabaseSchema(
		c.repoStores.Repositories, c.enrichCtx, c.schemaDiscoverer,
	))

	// Cookbook enrichment
	c.registry.Register(task.OperationCreateCookbookForCommit, enrichmenthandler.NewCookbook(
		c.repoStores.Repositories, c.repoStores.Files, c.enrichCtx, c.cookbookContext,
	))

	// API docs enrichment
	c.registry.Register(task.OperationCreatePublicAPIDocsForCommit, enrichmenthandler.NewAPIDocs(
		c.repoStores.Repositories, c.repoStores.Files, c.enrichCtx, c.apiDocService,
	))

	// Example extraction handler
	c.registry.Register(task.OperationExtractExamplesForCommit, enrichmenthandler.NewExtractExamples(
		c.repoStores.Repositories, c.repoStores.Commits, c.gitInfra.Adapter, c.enrichCtx, c.exampleDiscoverer,
	))

	c.logger.Info("registered task handlers", slog.Int("count", len(c.registry.Operations())))
}

// buildDatabaseURL constructs the database URL from configuration.
func buildDatabaseURL(cfg *clientConfig) (string, error) {
	switch cfg.database {
	case databaseSQLite:
		return "sqlite:///" + cfg.dbPath, nil
	case databasePostgres, databasePostgresPgvector, databasePostgresVectorchord:
		return cfg.dbDSN, nil
	default:
		return "", ErrNoDatabase
	}
}

// trackerFactoryImpl implements handler.TrackerFactory for progress reporting.
type trackerFactoryImpl struct {
	reporters []tracking.Reporter
	logger    *slog.Logger
}

// ForOperation creates a Tracker for the given operation.
func (f *trackerFactoryImpl) ForOperation(operation task.Operation, trackableType task.TrackableType, trackableID int64) handler.Tracker {
	tracker := tracking.TrackerForOperation(operation, f.logger, trackableType, trackableID)
	for _, reporter := range f.reporters {
		tracker.Subscribe(reporter)
	}
	return tracker
}
