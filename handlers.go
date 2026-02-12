package kodit

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/helixml/kodit/application/handler"
	commithandler "github.com/helixml/kodit/application/handler/commit"
	enrichmenthandler "github.com/helixml/kodit/application/handler/enrichment"
	indexinghandler "github.com/helixml/kodit/application/handler/indexing"
	repohandler "github.com/helixml/kodit/application/handler/repository"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/tracking"
)

// registerHandlers registers all task handlers with the worker registry.
func (c *Client) registerHandlers() error {
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

	// Code embedding handlers — only if embedding provider configured
	if c.codeIndex.Store != nil {
		h, err := indexinghandler.NewCreateCodeEmbeddings(c.codeIndex, c.snippetStore, c.enrichCtx.Tracker, c.logger)
		if err != nil {
			return fmt.Errorf("create code embeddings handler: %w", err)
		}
		c.registry.Register(task.OperationCreateCodeEmbeddingsForCommit, h)

		h2, err := indexinghandler.NewCreateExampleCodeEmbeddings(c.codeIndex, c.enrichCtx.Query, c.enrichCtx.Tracker, c.logger)
		if err != nil {
			return fmt.Errorf("create example code embeddings handler: %w", err)
		}
		c.registry.Register(task.OperationCreateExampleCodeEmbeddingsForCommit, h2)
	}

	// Text embedding handlers — only if text embedding provider configured
	if c.textIndex.Store != nil {
		h, err := indexinghandler.NewCreateSummaryEmbeddings(c.textIndex, c.enrichCtx.Query, c.enrichCtx.Associations, c.enrichCtx.Tracker, c.logger)
		if err != nil {
			return fmt.Errorf("create summary embeddings handler: %w", err)
		}
		c.registry.Register(task.OperationCreateSummaryEmbeddingsForCommit, h)

		h2, err := indexinghandler.NewCreateExampleSummaryEmbeddings(c.textIndex, c.enrichCtx.Query, c.enrichCtx.Tracker, c.logger)
		if err != nil {
			return fmt.Errorf("create example summary embeddings handler: %w", err)
		}
		c.registry.Register(task.OperationCreateExampleSummaryEmbeddingsForCommit, h2)
	}

	// Enrichment handlers that call Enricher — only if text provider configured
	if c.enrichCtx.Enricher != nil {
		h, err := enrichmenthandler.NewCreateSummary(c.snippetStore, c.enrichCtx)
		if err != nil {
			return fmt.Errorf("create summary handler: %w", err)
		}
		c.registry.Register(task.OperationCreateSummaryEnrichmentForCommit, h)

		h2, err := enrichmenthandler.NewCommitDescription(c.repoStores.Repositories, c.enrichCtx, c.gitInfra.Adapter)
		if err != nil {
			return fmt.Errorf("commit description handler: %w", err)
		}
		c.registry.Register(task.OperationCreateCommitDescriptionForCommit, h2)

		h3, err := enrichmenthandler.NewArchitectureDiscovery(c.repoStores.Repositories, c.enrichCtx, c.archDiscoverer)
		if err != nil {
			return fmt.Errorf("architecture discovery handler: %w", err)
		}
		c.registry.Register(task.OperationCreateArchitectureEnrichmentForCommit, h3)

		h4, err := enrichmenthandler.NewExampleSummary(c.enrichCtx)
		if err != nil {
			return fmt.Errorf("example summary handler: %w", err)
		}
		c.registry.Register(task.OperationCreateExampleSummaryForCommit, h4)

		h5, err := enrichmenthandler.NewDatabaseSchema(c.repoStores.Repositories, c.enrichCtx, c.schemaDiscoverer)
		if err != nil {
			return fmt.Errorf("database schema handler: %w", err)
		}
		c.registry.Register(task.OperationCreateDatabaseSchemaForCommit, h5)

		h6, err := enrichmenthandler.NewCookbook(c.repoStores.Repositories, c.repoStores.Files, c.enrichCtx, c.cookbookContext)
		if err != nil {
			return fmt.Errorf("cookbook handler: %w", err)
		}
		c.registry.Register(task.OperationCreateCookbookForCommit, h6)
	}

	// API docs enrichment (AST-based, no LLM dependency)
	c.registry.Register(task.OperationCreatePublicAPIDocsForCommit, enrichmenthandler.NewAPIDocs(
		c.repoStores.Repositories, c.repoStores.Files, c.enrichCtx, c.apiDocService,
	))

	// Example extraction handler (no LLM dependency)
	c.registry.Register(task.OperationExtractExamplesForCommit, enrichmenthandler.NewExtractExamples(
		c.repoStores.Repositories, c.repoStores.Commits, c.gitInfra.Adapter, c.enrichCtx, c.exampleDiscoverer,
	))

	c.logger.Info("registered task handlers", slog.Int("count", len(c.registry.Operations())))
	return nil
}

// validateHandlers checks that every prescribed operation has a registered handler.
// Returns an error listing missing operations and which provider to configure.
func (c *Client) validateHandlers() error {
	var missing []string
	for _, op := range (task.PrescribedOperations{}).All() {
		if !c.registry.HasHandler(op) {
			missing = append(missing, op.String())
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"missing handlers for operations: [%s] — configure a text provider (WithOpenAI, WithAnthropic) and an embedding provider (WithOpenAI, WithOllama) or set SKIP_PROVIDER_VALIDATION=true to start without them",
		strings.Join(missing, ", "),
	)
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

// workerTrackerAdapter adapts trackerFactoryImpl to service.WorkerTrackerFactory.
type workerTrackerAdapter struct {
	factory *trackerFactoryImpl
}

// ForOperation creates a WorkerTracker for the given operation.
func (a *workerTrackerAdapter) ForOperation(operation task.Operation, trackableType task.TrackableType, trackableID int64) service.WorkerTracker {
	return a.factory.ForOperation(operation, trackableType, trackableID)
}
