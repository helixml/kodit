package enrichment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/queue"
)

const databaseSchemaSystemPrompt = `
You are an expert database architect and documentation specialist.
Your task is to create clear, visual documentation of database schemas.
`

const databaseSchemaTaskPrompt = `
You will be provided with a database schema discovery report.
Please create comprehensive database schema documentation.

<schema_report>
%s
</schema_report>

**Return the following:**

## Entity List

For each table/entity, write one line:
- **[Table Name]**: [brief description of what it stores]

## Mermaid ERD

Create a Mermaid Entity Relationship Diagram showing:
- All entities (tables)
- Key relationships between entities (if apparent from names or common patterns)
- Use standard ERD notation

Example format:
` + "```mermaid" + `
erDiagram
    User ||--o{ Order : places
    User {
        int id PK
        string email
        string name
    }
    Order {
        int id PK
        int user_id FK
        datetime created_at
    }
` + "```" + `

If specific field details aren't available, show just the entity boxes and
relationships.

## Key Observations

Answer these questions in 1-2 sentences each:
1. What is the primary data model pattern (e.g., user-centric,
   event-sourced, multi-tenant)?
2. What migration strategy is being used?
3. Are there any notable database design patterns or concerns?

## Rules:
- Be concise and focus on the high-level structure
- Infer reasonable relationships from table names when explicit information
  isn't available
- If no database schema is found, state that clearly
- Keep entity descriptions to 10 words or less
`

// SchemaDiscoverer detects database schemas from a repository.
type SchemaDiscoverer interface {
	Discover(ctx context.Context, repoPath string) (string, error)
}

// DatabaseSchema handles the CREATE_DATABASE_SCHEMA_FOR_COMMIT operation.
type DatabaseSchema struct {
	repoRepo        git.RepoRepository
	enrichmentRepo  enrichment.EnrichmentRepository
	associationRepo enrichment.AssociationRepository
	queryService    *enrichment.QueryService
	discoverer      SchemaDiscoverer
	enricher        enrichment.Enricher
	trackerFactory  TrackerFactory
	logger          *slog.Logger
}

// NewDatabaseSchema creates a new DatabaseSchema handler.
func NewDatabaseSchema(
	repoRepo git.RepoRepository,
	enrichmentRepo enrichment.EnrichmentRepository,
	associationRepo enrichment.AssociationRepository,
	queryService *enrichment.QueryService,
	discoverer SchemaDiscoverer,
	enricher enrichment.Enricher,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *DatabaseSchema {
	return &DatabaseSchema{
		repoRepo:        repoRepo,
		enrichmentRepo:  enrichmentRepo,
		associationRepo: associationRepo,
		queryService:    queryService,
		discoverer:      discoverer,
		enricher:        enricher,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}
}

// Execute processes the CREATE_DATABASE_SCHEMA_FOR_COMMIT task.
func (h *DatabaseSchema) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := extractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationCreateDatabaseSchemaForCommit,
		domain.TrackableTypeRepository,
		repoID,
	)

	hasSchema, err := h.queryService.Exists(ctx, &enrichment.ExistsParams{CommitSHA: commitSHA, Type: enrichment.TypeArchitecture, Subtype: enrichment.SubtypeDatabaseSchema})
	if err != nil {
		h.logger.Error("failed to check existing database schema", slog.String("error", err.Error()))
		return err
	}

	if hasSchema {
		if skipErr := tracker.Skip(ctx, "Database schema already exists for commit"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	repo, err := h.repoRepo.Get(ctx, repoID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", repoID)
	}

	if setTotalErr := tracker.SetTotal(ctx, 3); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	if currentErr := tracker.SetCurrent(ctx, 1, "Discovering database schemas"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	schemaReport, err := h.discoverer.Discover(ctx, clonedPath)
	if err != nil {
		return fmt.Errorf("discover schemas: %w", err)
	}

	if strings.Contains(schemaReport, "No database schemas detected") {
		if skipErr := tracker.Skip(ctx, "No database schemas found in repository"); skipErr != nil {
			h.logger.Warn("failed to mark tracker as skipped", slog.String("error", skipErr.Error()))
		}
		return nil
	}

	if currentErr := tracker.SetCurrent(ctx, 2, "Enriching schema documentation with LLM"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	taskPrompt := fmt.Sprintf(databaseSchemaTaskPrompt, schemaReport)
	requests := []enrichment.Request{
		enrichment.NewRequest(commitSHA, taskPrompt, databaseSchemaSystemPrompt),
	}

	responses, err := h.enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich database schema: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("no enrichment response for commit %s", commitSHA)
	}

	schemaEnrichment := enrichment.NewDatabaseSchema(responses[0].Text())
	saved, err := h.enrichmentRepo.Save(ctx, schemaEnrichment)
	if err != nil {
		return fmt.Errorf("save database schema enrichment: %w", err)
	}

	commitAssoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
	if _, err := h.associationRepo.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	if currentErr := tracker.SetCurrent(ctx, 3, "Database schema enrichment completed"); currentErr != nil {
		h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

// Ensure DatabaseSchema implements queue.Handler.
var _ queue.Handler = (*DatabaseSchema)(nil)
