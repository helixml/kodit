package enrichment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
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
	repoStore  repository.RepositoryStore
	enrichCtx  handler.EnrichmentContext
	discoverer SchemaDiscoverer
}

// NewDatabaseSchema creates a new DatabaseSchema handler.
func NewDatabaseSchema(
	repoStore repository.RepositoryStore,
	enrichCtx handler.EnrichmentContext,
	discoverer SchemaDiscoverer,
) (*DatabaseSchema, error) {
	if repoStore == nil {
		return nil, fmt.Errorf("NewDatabaseSchema: nil repoStore")
	}
	if enrichCtx.Enricher == nil {
		return nil, fmt.Errorf("NewDatabaseSchema: nil Enricher")
	}
	if discoverer == nil {
		return nil, fmt.Errorf("NewDatabaseSchema: nil discoverer")
	}
	return &DatabaseSchema{
		repoStore:  repoStore,
		enrichCtx:  enrichCtx,
		discoverer: discoverer,
	}, nil
}

// Execute processes the CREATE_DATABASE_SCHEMA_FOR_COMMIT task.
func (h *DatabaseSchema) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationCreateDatabaseSchemaForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	count, err := h.enrichCtx.Enrichments.Count(ctx, enrichment.WithCommitSHA(cp.CommitSHA()), enrichment.WithType(enrichment.TypeArchitecture), enrichment.WithSubtype(enrichment.SubtypeDatabaseSchema))
	if err != nil {
		h.enrichCtx.Logger.Error("failed to check existing database schema", slog.String("error", err.Error()))
		return err
	}

	if count > 0 {
		tracker.Skip(ctx, "Database schema already exists for commit")
		return nil
	}

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(cp.RepoID()))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", cp.RepoID())
	}

	tracker.SetTotal(ctx, 3)
	tracker.SetCurrent(ctx, 1, "Discovering database schemas")

	schemaReport, err := h.discoverer.Discover(ctx, clonedPath)
	if err != nil {
		return fmt.Errorf("discover schemas: %w", err)
	}

	if strings.Contains(schemaReport, "No database schemas detected") {
		tracker.Skip(ctx, "No database schemas found in repository")
		return nil
	}

	tracker.SetCurrent(ctx, 2, "Enriching schema documentation with LLM")

	taskPrompt := fmt.Sprintf(databaseSchemaTaskPrompt, schemaReport)
	requests := []domainservice.EnrichmentRequest{
		domainservice.NewEnrichmentRequest(cp.CommitSHA(), taskPrompt, databaseSchemaSystemPrompt),
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return fmt.Errorf("enrich database schema: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("no enrichment response for commit %s", cp.CommitSHA())
	}

	schemaEnrichment := enrichment.NewEnrichment(
		enrichment.TypeArchitecture,
		enrichment.SubtypeDatabaseSchema,
		enrichment.EntityTypeCommit,
		responses[0].Text(),
	)
	saved, err := h.enrichCtx.Enrichments.Save(ctx, schemaEnrichment)
	if err != nil {
		return fmt.Errorf("save database schema enrichment: %w", err)
	}

	commitAssoc := enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())
	if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	tracker.SetCurrent(ctx, 3, "Database schema enrichment completed")

	return nil
}

// Ensure DatabaseSchema implements handler.Handler.
var _ handler.Handler = (*DatabaseSchema)(nil)
