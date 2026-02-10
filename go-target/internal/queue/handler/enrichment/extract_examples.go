package enrichment

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/queue"
)

// ExampleDiscoverer determines if files are example candidates.
type ExampleDiscoverer interface {
	IsExampleCandidate(path string) bool
	IsDocumentationFile(path string) bool
}

// DocumentationParser parses code blocks from documentation files.
type DocumentationParser interface {
	Parse(content string) []string
}

// ExtractExamples handles the EXTRACT_EXAMPLES_FOR_COMMIT operation.
type ExtractExamples struct {
	repoRepo        git.RepoRepository
	commitRepo      git.CommitRepository
	adapter         git.Adapter
	enrichmentRepo  enrichment.EnrichmentRepository
	associationRepo enrichment.AssociationRepository
	queryService    *enrichment.QueryService
	discoverer      ExampleDiscoverer
	trackerFactory  TrackerFactory
	logger          *slog.Logger
}

// NewExtractExamples creates a new ExtractExamples handler.
func NewExtractExamples(
	repoRepo git.RepoRepository,
	commitRepo git.CommitRepository,
	adapter git.Adapter,
	enrichmentRepo enrichment.EnrichmentRepository,
	associationRepo enrichment.AssociationRepository,
	queryService *enrichment.QueryService,
	discoverer ExampleDiscoverer,
	trackerFactory TrackerFactory,
	logger *slog.Logger,
) *ExtractExamples {
	return &ExtractExamples{
		repoRepo:        repoRepo,
		commitRepo:      commitRepo,
		adapter:         adapter,
		enrichmentRepo:  enrichmentRepo,
		associationRepo: associationRepo,
		queryService:    queryService,
		discoverer:      discoverer,
		trackerFactory:  trackerFactory,
		logger:          logger,
	}
}

// Execute processes the EXTRACT_EXAMPLES_FOR_COMMIT task.
func (h *ExtractExamples) Execute(ctx context.Context, payload map[string]any) error {
	repoID, err := extractInt64(payload, "repository_id")
	if err != nil {
		return err
	}

	commitSHA, err := extractString(payload, "commit_sha")
	if err != nil {
		return err
	}

	tracker := h.trackerFactory.ForOperation(
		queue.OperationExtractExamplesForCommit,
		domain.TrackableTypeRepository,
		repoID,
	)

	hasExamples, err := h.queryService.Exists(ctx, &enrichment.ExistsParams{CommitSHA: commitSHA, Type: enrichment.TypeDevelopment, Subtype: enrichment.SubtypeExample})
	if err != nil {
		h.logger.Error("failed to check existing examples", slog.String("error", err.Error()))
		return err
	}

	if hasExamples {
		if skipErr := tracker.Skip(ctx, "Examples already extracted for commit"); skipErr != nil {
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

	files, err := h.adapter.CommitFiles(ctx, clonedPath, commitSHA)
	if err != nil {
		return fmt.Errorf("get commit files: %w", err)
	}

	var candidates []git.FileInfo
	for _, f := range files {
		fullPath := filepath.Join(clonedPath, f.Path)
		if h.discoverer.IsExampleCandidate(fullPath) {
			candidates = append(candidates, f)
		}
	}

	if setTotalErr := tracker.SetTotal(ctx, len(candidates)); setTotalErr != nil {
		h.logger.Warn("failed to set tracker total", slog.String("error", setTotalErr.Error()))
	}

	var examples []string

	for i, file := range candidates {
		fullPath := filepath.Join(clonedPath, file.Path)
		fileName := filepath.Base(file.Path)

		if currentErr := tracker.SetCurrent(ctx, i, fmt.Sprintf("Processing %s", fileName)); currentErr != nil {
			h.logger.Warn("failed to set tracker current", slog.String("error", currentErr.Error()))
		}

		if h.discoverer.IsDocumentationFile(fullPath) {
			example := h.extractFromDocumentation(fullPath)
			if example != "" {
				examples = append(examples, example)
			}
		} else {
			example := h.extractFullFile(fullPath)
			if example != "" {
				examples = append(examples, example)
			}
		}
	}

	uniqueExamples := h.deduplicateExamples(examples)

	h.logger.Info("extracted examples",
		slog.Int("total", len(examples)),
		slog.Int("unique", len(uniqueExamples)),
		slog.String("commit", commitSHA[:8]),
	)

	for _, content := range uniqueExamples {
		sanitized := h.sanitizeContent(content)
		exampleEnrichment := enrichment.NewExample(sanitized)

		saved, err := h.enrichmentRepo.Save(ctx, exampleEnrichment)
		if err != nil {
			return fmt.Errorf("save example enrichment: %w", err)
		}

		commitAssoc := enrichment.CommitAssociation(saved.ID(), commitSHA)
		if _, err := h.associationRepo.Save(ctx, commitAssoc); err != nil {
			return fmt.Errorf("save commit association: %w", err)
		}
	}

	if completeErr := tracker.Complete(ctx); completeErr != nil {
		h.logger.Warn("failed to mark tracker as complete", slog.String("error", completeErr.Error()))
	}

	return nil
}

func (h *ExtractExamples) extractFromDocumentation(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		h.logger.Warn("failed to read file", slog.String("path", path), slog.String("error", err.Error()))
		return ""
	}

	blocks := extractCodeBlocks(string(content))
	if len(blocks) == 0 {
		return ""
	}

	return strings.Join(blocks, "\n\n")
}

func (h *ExtractExamples) extractFullFile(path string) string {
	ext := filepath.Ext(path)
	mapping := domain.LanguageMapping{}

	_, err := mapping.LanguageForExtension(ext)
	if err != nil {
		return ""
	}

	content, err := os.ReadFile(path)
	if err != nil {
		h.logger.Warn("failed to read file", slog.String("path", path), slog.String("error", err.Error()))
		return ""
	}

	return string(content)
}

func (h *ExtractExamples) deduplicateExamples(examples []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, e := range examples {
		if !seen[e] {
			seen[e] = true
			result = append(result, e)
		}
	}

	return result
}

func (h *ExtractExamples) sanitizeContent(content string) string {
	return strings.ReplaceAll(content, "\x00", "")
}

// extractCodeBlocks extracts fenced code blocks from markdown content.
func extractCodeBlocks(content string) []string {
	var blocks []string
	lines := strings.Split(content, "\n")
	inBlock := false
	var currentBlock []string

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inBlock {
				blocks = append(blocks, strings.Join(currentBlock, "\n"))
				currentBlock = nil
				inBlock = false
			} else {
				inBlock = true
			}
		} else if inBlock {
			currentBlock = append(currentBlock, line)
		}
	}

	return blocks
}

// Ensure ExtractExamples implements queue.Handler.
var _ queue.Handler = (*ExtractExamples)(nil)
