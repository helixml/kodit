package enrichment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/domain/wiki"
)

const wikiPlanSystemPrompt = `You are a technical writer planning the structure of a comprehensive wiki
for a software project. You analyze repository content and produce a structured outline.
Output valid JSON only, with no additional text.`

const wikiPlanTaskPrompt = `Based on the following repository information, plan a wiki with 5-15 pages.
Return a JSON object with a "pages" array. Each page has:
- "slug": URL-safe identifier (lowercase, hyphens)
- "title": Display title
- "sources": Array of relevant file paths or enrichment types to reference
- "children": Array of child pages (same structure, may be empty)

Repository file tree:
<file_tree>
%s
</file_tree>

README:
<readme>
%s
</readme>

Existing enrichments:
<enrichments>
%s
</enrichments>

Return ONLY the JSON object. Example structure:
{"pages":[{"slug":"overview","title":"Project Overview","sources":["README.md"],"children":[]}]}`

const wikiPageSystemPrompt = `You are a technical writer creating documentation for a software project wiki.
Write clear, well-structured Markdown content for the requested page.
Use cross-references to other wiki pages where appropriate using [link text](slug) format.

CRITICAL: Your output will be served directly as a .md file. Output raw Markdown ONLY.
NEVER wrap your response in triple-backtick code fences (e.g. ` + "```" + `markdown ... ` + "```" + `).
Start directly with the page content (e.g. a heading).`

const wikiPageTaskPrompt = `Write the content for wiki page "%s" (slug: %s).
Repository URL: %s

The full wiki has these pages (you can link to any of them):
%s

Source material:
<sources>
%s
</sources>

Write comprehensive Markdown documentation for this page. Include code examples where relevant.
Use the repository URL and name when referring to the project — never use placeholder text like "[Project Name]".`

const wikiIndexSystemPrompt = `You are a technical writer creating the home page for a software project wiki.
Write a welcoming overview that introduces the project and guides readers to the right pages.

CRITICAL: Your output will be served directly as a .md file. Output raw Markdown ONLY.
NEVER wrap your response in triple-backtick code fences (e.g. ` + "```" + `markdown ... ` + "```" + `).
Start directly with the page content (e.g. a heading).`

const wikiIndexTaskPrompt = `Write the index/home page for this project wiki.
Repository URL: %s
Current commit: %s
Generated on: %s

Available pages:
%s

README content:
<readme>
%s
</readme>

Write a concise introduction that helps readers navigate the wiki.
Use links in [title](slug) format to reference pages.
Include the current date and commit SHA on the page.
Use the repository URL and name when referring to the project — never use placeholder text like "[Project Name]".`

// WikiContextGatherer gathers context for wiki generation.
type WikiContextGatherer interface {
	Gather(ctx context.Context, repoPath string, files []repository.File, existingEnrichments []enrichment.Enrichment) (readme, fileTree, enrichments string, err error)
	FileContent(repoPath, filePath string, maxLen int) string
}

// wikiGatheredContext holds the context strings needed for wiki generation.
type wikiGatheredContext struct {
	readme      string
	fileTree    string
	enrichments string
	repoURL     string
	commitSHA   string
}

// Wiki handles the GENERATE_WIKI_FOR_COMMIT operation.
type Wiki struct {
	repoStore       repository.RepositoryStore
	commitStore     repository.CommitStore
	fileStore       repository.FileStore
	enrichCtx       handler.EnrichmentContext
	contextGatherer WikiContextGatherer
}

// NewWiki creates a new Wiki handler.
func NewWiki(
	repoStore repository.RepositoryStore,
	commitStore repository.CommitStore,
	fileStore repository.FileStore,
	enrichCtx handler.EnrichmentContext,
	contextGatherer WikiContextGatherer,
) (*Wiki, error) {
	if repoStore == nil {
		return nil, fmt.Errorf("NewWiki: nil repoStore")
	}
	if commitStore == nil {
		return nil, fmt.Errorf("NewWiki: nil commitStore")
	}
	if fileStore == nil {
		return nil, fmt.Errorf("NewWiki: nil fileStore")
	}
	if enrichCtx.Enricher == nil {
		return nil, fmt.Errorf("NewWiki: nil Enricher")
	}
	if contextGatherer == nil {
		return nil, fmt.Errorf("NewWiki: nil contextGatherer")
	}
	return &Wiki{
		repoStore:       repoStore,
		commitStore:     commitStore,
		fileStore:       fileStore,
		enrichCtx:       enrichCtx,
		contextGatherer: contextGatherer,
	}, nil
}

// Execute processes the GENERATE_WIKI_FOR_COMMIT task.
func (h *Wiki) Execute(ctx context.Context, payload map[string]any) error {
	cp, err := handler.ExtractCommitPayload(payload)
	if err != nil {
		return err
	}

	tracker := h.enrichCtx.Tracker.ForOperation(
		task.OperationGenerateWikiForCommit,
		task.TrackableTypeRepository,
		cp.RepoID(),
	)

	// Delete any existing wiki for this repository so each repo has at most one.
	if err := h.deleteExistingWiki(ctx, cp.RepoID()); err != nil {
		return fmt.Errorf("delete existing wiki: %w", err)
	}

	repo, err := h.repoStore.FindOne(ctx, repository.WithID(cp.RepoID()))
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	clonedPath := repo.WorkingCopy().Path()
	if clonedPath == "" {
		return fmt.Errorf("repository %d has never been cloned", cp.RepoID())
	}

	files, err := h.fileStore.Find(ctx, repository.WithCommitSHA(cp.CommitSHA()))
	if err != nil {
		return fmt.Errorf("get files: %w", err)
	}
	if len(files) == 0 {
		tracker.Skip(ctx, "No files to generate wiki from")
		return nil
	}

	// Get existing enrichments to synthesize.
	existingEnrichments, err := h.enrichCtx.Enrichments.Find(ctx,
		enrichment.WithCommitSHA(cp.CommitSHA()),
	)
	if err != nil {
		return fmt.Errorf("get existing enrichments: %w", err)
	}

	// Filter to only architecture, API docs, and cookbook.
	var relevantEnrichments []enrichment.Enrichment
	for _, e := range existingEnrichments {
		if enrichment.IsArchitectureEnrichment(e) || enrichment.IsAPIDocs(e) || enrichment.IsCookbook(e) {
			relevantEnrichments = append(relevantEnrichments, e)
		}
	}

	readme, fileTree, enrichmentText, err := h.contextGatherer.Gather(ctx, clonedPath, files, relevantEnrichments)
	if err != nil {
		return fmt.Errorf("gather wiki context: %w", err)
	}

	wikiCtx := wikiGatheredContext{
		readme:      readme,
		fileTree:    fileTree,
		enrichments: enrichmentText,
		repoURL:     repo.RemoteURL(),
		commitSHA:   cp.CommitSHA(),
	}

	// Phase 1: Plan the wiki.
	tracker.SetTotal(ctx, 3)
	tracker.SetCurrent(ctx, 0, "Planning wiki structure")

	outline, err := h.planWiki(ctx, wikiCtx)
	if err != nil {
		return fmt.Errorf("plan wiki: %w", err)
	}

	if len(outline.Pages) == 0 {
		tracker.Skip(ctx, "Wiki plan produced no pages")
		return nil
	}

	// Phase 2: Generate each page.
	tracker.SetTotal(ctx, len(outline.Pages)+2) // +2 for plan and index phases
	tracker.SetCurrent(ctx, 1, "Generating wiki pages")

	pages, err := h.generatePages(ctx, tracker, outline, wikiCtx, clonedPath)
	if err != nil {
		return fmt.Errorf("generate pages: %w", err)
	}

	// Phase 3: Generate index page.
	tracker.SetCurrent(ctx, len(outline.Pages)+1, "Generating wiki index")

	indexPage, err := h.generateIndex(ctx, outline, wikiCtx)
	if err != nil {
		return fmt.Errorf("generate index: %w", err)
	}

	// Prepend index to pages.
	allPages := append([]wiki.Page{indexPage}, pages...)

	// Phase 4: Assemble and save.
	tracker.SetCurrent(ctx, len(outline.Pages)+2, "Saving wiki")

	w := wiki.NewWiki(allPages)
	content, err := w.JSON()
	if err != nil {
		return fmt.Errorf("serialize wiki: %w", err)
	}

	wikiEnrichment := enrichment.NewWiki(content)
	saved, err := h.enrichCtx.Enrichments.Save(ctx, wikiEnrichment)
	if err != nil {
		return fmt.Errorf("save wiki enrichment: %w", err)
	}

	commitAssoc := enrichment.CommitAssociation(saved.ID(), cp.CommitSHA())
	if _, err := h.enrichCtx.Associations.Save(ctx, commitAssoc); err != nil {
		return fmt.Errorf("save commit association: %w", err)
	}

	return nil
}

// deleteExistingWiki removes any previous wiki enrichments for the repository.
func (h *Wiki) deleteExistingWiki(ctx context.Context, repoID int64) error {
	commits, err := h.commitStore.Find(ctx, repository.WithRepoID(repoID), repository.WithLimit(100))
	if err != nil {
		return fmt.Errorf("find commits: %w", err)
	}

	if len(commits) == 0 {
		return nil
	}

	shas := make([]string, 0, len(commits))
	for _, c := range commits {
		shas = append(shas, c.SHA())
	}

	existing, err := h.enrichCtx.Enrichments.Find(ctx,
		enrichment.WithCommitSHAs(shas),
		enrichment.WithType(enrichment.TypeUsage),
		enrichment.WithSubtype(enrichment.SubtypeWiki),
	)
	if err != nil {
		return fmt.Errorf("find existing wikis: %w", err)
	}

	if len(existing) == 0 {
		return nil
	}

	ids := make([]int64, len(existing))
	for i, e := range existing {
		ids[i] = e.ID()
	}

	if err := h.enrichCtx.Enrichments.DeleteBy(ctx, repository.WithIDIn(ids)); err != nil {
		return fmt.Errorf("delete existing wikis: %w", err)
	}

	return nil
}

// planWiki calls the LLM to produce a structured wiki outline (Phase 1).
func (h *Wiki) planWiki(ctx context.Context, wikiCtx wikiGatheredContext) (wikiOutline, error) {
	prompt := fmt.Sprintf(wikiPlanTaskPrompt, wikiCtx.fileTree, wikiCtx.readme, wikiCtx.enrichments)

	requests := []domainservice.EnrichmentRequest{
		domainservice.NewEnrichmentRequest("wiki-plan", prompt, wikiPlanSystemPrompt),
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return wikiOutline{}, fmt.Errorf("enrich wiki plan: %w", err)
	}
	if len(responses) == 0 {
		return wikiOutline{}, fmt.Errorf("no response for wiki plan")
	}

	text := responses[0].Text()
	text = extractJSON(text)

	var outline wikiOutline
	if err := json.Unmarshal([]byte(text), &outline); err != nil {
		return wikiOutline{}, fmt.Errorf("parse wiki plan: %w", err)
	}

	return outline, nil
}

// generatePages generates content for each page in the outline (Phase 2).
func (h *Wiki) generatePages(
	ctx context.Context,
	tracker handler.Tracker,
	outline wikiOutline,
	wikiCtx wikiGatheredContext,
	repoPath string,
) ([]wiki.Page, error) {
	pageListing := h.pageListingText(outline)

	var pages []wiki.Page
	flatPages := outline.flatten()

	for i, entry := range flatPages {
		tracker.SetCurrent(ctx, i+2, fmt.Sprintf("Generating page: %s", entry.Title))

		sources := h.gatherSources(entry, wikiCtx, repoPath)
		prompt := fmt.Sprintf(wikiPageTaskPrompt, entry.Title, entry.Slug, wikiCtx.repoURL, pageListing, sources)

		requests := []domainservice.EnrichmentRequest{
			domainservice.NewEnrichmentRequest(entry.Slug, prompt, wikiPageSystemPrompt),
		}

		responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
		if err != nil {
			return nil, fmt.Errorf("generate page %s: %w", entry.Slug, err)
		}
		if len(responses) == 0 {
			continue
		}

		pages = append(pages, wiki.NewPage(entry.Slug, entry.Title, stripCodeFence(responses[0].Text()), i+1, nil))
	}

	// Rebuild the tree structure from the flat list.
	return h.rebuildTree(outline, pages), nil
}

// generateIndex generates the wiki home page (Phase 3).
func (h *Wiki) generateIndex(ctx context.Context, outline wikiOutline, wikiCtx wikiGatheredContext) (wiki.Page, error) {
	pageListing := h.pageListingText(outline)
	now := time.Now().UTC().Format("2006-01-02")
	prompt := fmt.Sprintf(wikiIndexTaskPrompt, wikiCtx.repoURL, wikiCtx.commitSHA, now, pageListing, wikiCtx.readme)

	requests := []domainservice.EnrichmentRequest{
		domainservice.NewEnrichmentRequest("index", prompt, wikiIndexSystemPrompt),
	}

	responses, err := h.enrichCtx.Enricher.Enrich(ctx, requests)
	if err != nil {
		return wiki.Page{}, fmt.Errorf("generate index: %w", err)
	}
	if len(responses) == 0 {
		return wiki.Page{}, fmt.Errorf("no response for wiki index")
	}

	return wiki.NewPage("index", "Home", stripCodeFence(responses[0].Text()), 0, nil), nil
}

// stripCodeFence removes an outer ```markdown ... ``` wrapper that LLMs
// sometimes add despite being told not to.
func stripCodeFence(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "```") {
		return text
	}
	// Remove opening fence line.
	firstNewline := strings.Index(trimmed, "\n")
	if firstNewline == -1 {
		return text
	}
	inner := trimmed[firstNewline+1:]
	// Remove closing fence.
	if idx := strings.LastIndex(inner, "```"); idx != -1 {
		inner = inner[:idx]
	}
	return strings.TrimSpace(inner)
}

// pageListingText builds a text listing of all pages for cross-referencing.
func (h *Wiki) pageListingText(outline wikiOutline) string {
	var lines []string
	for _, p := range outline.flatten() {
		lines = append(lines, fmt.Sprintf("- [%s](%s)", p.Title, p.Slug))
	}
	return strings.Join(lines, "\n")
}

// gatherSources reads the source files and enrichments referenced by a page entry.
func (h *Wiki) gatherSources(entry outlinePage, wikiCtx wikiGatheredContext, repoPath string) string {
	var parts []string

	for _, source := range entry.Sources {
		if strings.HasSuffix(source, "_enrichment") || !strings.Contains(source, "/") {
			// Might be an enrichment reference; include enrichment context.
			if wikiCtx.enrichments != "" {
				parts = append(parts, wikiCtx.enrichments)
			}
			continue
		}
		// It's a file path; read it.
		content := h.contextGatherer.FileContent(repoPath, source, 3000)
		if content != "" {
			parts = append(parts, fmt.Sprintf("### %s\n```\n%s\n```", source, content))
		}
	}

	// If no sources were found, provide README and file tree as fallback.
	if len(parts) == 0 {
		if wikiCtx.readme != "" {
			parts = append(parts, "### README\n"+wikiCtx.readme)
		}
		if wikiCtx.fileTree != "" {
			parts = append(parts, "### File Tree\n"+wikiCtx.fileTree)
		}
	}

	return strings.Join(parts, "\n\n")
}

// rebuildTree takes a flat list of generated pages and restructures them
// according to the original outline hierarchy.
func (h *Wiki) rebuildTree(outline wikiOutline, flatPages []wiki.Page) []wiki.Page {
	pageMap := make(map[string]wiki.Page, len(flatPages))
	for _, p := range flatPages {
		pageMap[p.Slug()] = p
	}

	return h.buildPagesFromOutline(outline.Pages, pageMap)
}

func (h *Wiki) buildPagesFromOutline(entries []outlinePage, pageMap map[string]wiki.Page) []wiki.Page {
	var result []wiki.Page
	for i, entry := range entries {
		p, ok := pageMap[entry.Slug]
		if !ok {
			continue
		}
		children := h.buildPagesFromOutline(entry.Children, pageMap)
		result = append(result, wiki.NewPage(p.Slug(), p.Title(), p.Content(), i, children))
	}
	return result
}

// extractJSON extracts the first JSON object from a string that may contain
// surrounding text (markdown fences, explanatory text, etc.).
func extractJSON(text string) string {
	start := strings.Index(text, "{")
	if start == -1 {
		return text
	}
	// Find the matching closing brace.
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return text[start:]
}

// JSON types for wiki outline (internal to handler).

type wikiOutline struct {
	Pages []outlinePage `json:"pages"`
}

func (o wikiOutline) flatten() []outlinePage {
	var result []outlinePage
	for _, p := range o.Pages {
		result = append(result, p)
		result = append(result, p.flatChildren()...)
	}
	return result
}

type outlinePage struct {
	Slug     string        `json:"slug"`
	Title    string        `json:"title"`
	Sources  []string      `json:"sources"`
	Children []outlinePage `json:"children"`
}

func (p outlinePage) flatChildren() []outlinePage {
	var result []outlinePage
	for _, child := range p.Children {
		result = append(result, child)
		result = append(result, child.flatChildren()...)
	}
	return result
}

// Ensure Wiki implements handler.Handler.
var _ handler.Handler = (*Wiki)(nil)
