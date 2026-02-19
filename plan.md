# Plan: Wiki Generation Feature

## Problem Statement

We need a feature that generates a multi-page wiki from a repository. Two challenges:

1. **Storage**: Enrichments are single documents (`enrichments_v2` table with a single `content` TEXT column). A wiki is a structured collection of pages with hierarchy, ordering, and cross-references.
2. **Generation**: Building a wiki requires iterating over all files in a repository, which could be very large. We need a strategy that stays within token budgets while producing coherent, comprehensive documentation.

---

## Part 1: Storage Design

### Why enrichments don't work for wikis

The current `Enrichment` model (`domain/enrichment/enrichment.go`) is a single immutable document:

```
enrichments_v2: id | type | subtype | content | language | created_at | updated_at
```

A wiki page needs:
- A **title** and **slug** (for URL/navigation)
- **Ordering** (position within the wiki)
- **Hierarchy** (parent-child pages, sections)
- **Repository-level association** (enrichments only link to commits)

Storing wiki pages as enrichments would require abusing the `content` field with JSON metadata, using enrichment-to-enrichment associations for hierarchy, and losing the ability to query by title/slug efficiently. This would couple two distinct domain concepts.

### Proposed: New `WikiPage` domain entity

Introduce a first-class `WikiPage` domain object and `wiki_pages` database table:

```
wiki_pages
├── id              BIGINT PRIMARY KEY AUTO_INCREMENT
├── repo_id         BIGINT NOT NULL (FK → git_repos.id, ON DELETE CASCADE)
├── commit_sha      VARCHAR(64) NOT NULL (FK → git_commits.commit_sha)
├── slug            VARCHAR(255) NOT NULL
├── title           VARCHAR(512) NOT NULL
├── content         TEXT NOT NULL
├── position        INT NOT NULL DEFAULT 0
├── parent_id       BIGINT (FK → wiki_pages.id, ON DELETE CASCADE, nullable)
├── created_at      TIMESTAMP NOT NULL
├── updated_at      TIMESTAMP NOT NULL
├── UNIQUE INDEX    idx_wiki_repo_commit_slug (repo_id, commit_sha, slug)
```

Key design decisions:

- **Direct FK to repository**: Unlike enrichments (which go through commits via associations), wiki pages belong to a repository at a specific commit. This makes querying simple: `SELECT * FROM wiki_pages WHERE repo_id = ? AND commit_sha = ? ORDER BY position`.
- **Hierarchical structure via `parent_id`**: Top-level pages have `parent_id = NULL`. Sub-sections reference their parent. This gives us a tree without needing a separate table.
- **`slug` for stable references**: Pages can cross-reference each other by slug (e.g., `[see Architecture](architecture)`). The `(repo_id, commit_sha, slug)` unique index prevents duplicate slugs.
- **`position` for ordering**: Integer ordering within the same parent. The wiki table of contents is reconstructed by querying all pages for a repo/commit and sorting by parent_id + position.
- **`commit_sha` for versioning**: When a repo is re-scanned at a new commit, the wiki is regenerated. Old wiki pages for the previous commit remain until explicitly cleaned up (or cascaded when the commit is deleted).

### Domain layer

```
domain/wiki/
├── page.go          # WikiPage immutable value object
├── store.go         # WikiPageStore interface
└── options.go       # WithRepoID, WithCommitSHA, WithSlug, WithParentID, etc.
```

`WikiPage` follows the same immutable pattern as other domain objects:

```go
type Page struct {
    id        int64
    repoID    int64
    commitSHA string
    slug      string
    title     string
    content   string
    position  int
    parentID  int64   // 0 = top-level
    createdAt time.Time
    updatedAt time.Time
}
```

### Persistence layer

```
infrastructure/persistence/
├── wiki_store.go    # WikiPageStore implementation (embeds database.Repository[wiki.Page, WikiPageModel])
├── models.go        # WikiPageModel added
├── mappers.go       # WikiPageMapper added
├── db.go            # AutoMigrate updated
```

### Service layer

```
application/service/wiki.go
```

Service methods:
- `Generate(ctx, repoID)` — enqueues wiki generation task
- `Pages(ctx, repoID, commitSHA)` — returns full page tree
- `Page(ctx, repoID, commitSHA, slug)` — returns single page

### API layer

```
GET /api/v1/repositories/{id}/wiki              # List all pages (tree structure)
GET /api/v1/repositories/{id}/wiki/{slug}        # Single page
POST /api/v1/repositories/{id}/wiki/generate     # Trigger wiki generation
```

---

## Part 2: Wiki Generation Strategy

### Multi-pass approach

Wiki generation should be a **deterministic, multi-pass pipeline** — not an autonomous agent. This keeps token usage predictable and the process debuggable.

#### Phase 1: Plan the wiki (1 LLM call)

**Input**: Repository metadata, file tree, README, existing enrichments (architecture, API docs, cookbook).

**Output**: A structured JSON outline — the table of contents with page slugs, titles, and which files/enrichments are relevant to each page.

```json
{
  "pages": [
    {
      "slug": "overview",
      "title": "Project Overview",
      "sources": ["README.md", "architecture_enrichment"],
      "children": []
    },
    {
      "slug": "getting-started",
      "title": "Getting Started",
      "sources": ["README.md", "docs/setup.md", "Makefile"],
      "children": [
        {
          "slug": "installation",
          "title": "Installation",
          "sources": ["docs/install.md", "Dockerfile"]
        }
      ]
    },
    {
      "slug": "architecture",
      "title": "Architecture",
      "sources": ["architecture_enrichment", "docker-compose.yml"],
      "children": []
    },
    {
      "slug": "api-reference",
      "title": "API Reference",
      "sources": ["api_docs_enrichment", "src/api/"],
      "children": []
    }
  ]
}
```

**Token budget**: ~2-4K input (file tree + README excerpt + enrichment summaries), ~2K output. Very cheap.

**Highlighting specific things**: The user can pass a list of "focus areas" or "highlights" when triggering generation. These get included in the Phase 1 prompt: "Pay special attention to: [user-provided highlights]". The LLM then allocates pages to those topics.

#### Phase 2: Generate each page (N LLM calls, one per page)

For each page in the outline:

1. **Gather context**: Read the specific files listed in `sources` (truncated to ~3K chars each, max ~15K total per page). If an enrichment is listed as a source, include its content directly.
2. **Generate**: One LLM call per page with a focused system prompt and the gathered context.
3. **Save**: Store as a `WikiPage` with the slug, title, position, and parent_id from the outline.

**Token budget per page**: ~15K input context + ~2K output = ~17K tokens. For a 10-page wiki, that's ~170K tokens total.

**Cross-referencing**: The system prompt tells the LLM about all other page slugs and titles so it can create `[link text](slug)` references.

#### Phase 3: Generate index page (1 LLM call, optional)

Generate a root "index" page that serves as the wiki home. Input is the outline + first paragraph of each page. This provides a cohesive introduction.

### Token budget summary

| Phase | Calls | Input tokens/call | Output tokens/call | Total tokens |
|-------|-------|-------------------|-------------------|-------------|
| Plan  | 1     | ~4K               | ~2K               | ~6K         |
| Pages | ~10   | ~15K              | ~2K               | ~170K       |
| Index | 1     | ~5K               | ~1K               | ~6K         |
| **Total** |   |                   |                   | **~182K**   |

This is manageable. For large repos with many pages (say 20), it would be ~350K tokens. Still reasonable — comparable to a single long agent conversation.

### Reducing token cost further

- **Leverage existing enrichments heavily**: Architecture, API docs, cookbook, and snippet summaries are already generated. The wiki can synthesize these rather than re-analyzing raw files. This means Phase 2 often just reformats and connects existing enrichment content.
- **Smart file selection**: Phase 1 picks which files matter per page. Phase 2 only reads those files, not the whole repo.
- **Content-addressed dedup**: If a wiki page's source files haven't changed (same blob SHAs), skip regeneration. Compare `wiki_pages.commit_sha` with the current HEAD to determine staleness.

### Task integration

Add a new operation to the task queue:

```go
OperationGenerateWikiForCommit Operation = "kodit.commit.generate_wiki"
```

The handler (`application/handler/enrichment/wiki.go`) follows the standard pattern:
1. Check if wiki already exists for this commit
2. If so, skip
3. Otherwise, run Phase 1 → Phase 2 → Phase 3
4. Save all pages in a transaction
5. Update tracker

This operation goes at the end of `ScanAndIndexCommit()` — it depends on other enrichments being generated first (architecture, API docs, cookbook).

### Making it deterministic (not an agent)

The user is right to want a deterministic pipeline rather than an autonomous agent:

- **Phase 1** is deterministic: same inputs → same outline (low temperature, structured JSON output)
- **Phase 2** is embarrassingly parallel: each page is independent
- **No tool use, no loops, no branching**: The LLM just generates text given context
- **Predictable cost**: Exactly `1 + N + 1` LLM calls where N is the number of pages

The only "iterative" part is processing pages one-by-one, but this is a simple loop — not an agent deciding what to do next.

---

## Part 3: Implementation Steps

1. **Domain layer**: `domain/wiki/page.go`, `store.go`, `options.go`
2. **Persistence layer**: `WikiPageModel`, `WikiPageStore`, mapper, AutoMigrate
3. **Service layer**: `application/service/wiki.go`
4. **Handler**: `application/handler/enrichment/wiki.go` (Phase 1-3 pipeline)
5. **Task operation**: `OperationGenerateWikiForCommit` + add to `ScanAndIndexCommit()`
6. **Context gatherer**: `infrastructure/enricher/wiki_context.go` (gathers file tree, reads files, collects existing enrichments)
7. **API endpoints**: Wiki listing + single page + trigger generation
8. **Tests**: Unit tests for domain, store, handler; integration test for full pipeline

---

## Part 4: Open Questions

1. **Should the wiki be regenerated on every commit, or only on demand?** Adding it to `ScanAndIndexCommit()` means automatic regeneration. This is convenient but adds ~180K tokens per commit scan. An on-demand `POST /wiki/generate` might be more appropriate given the cost.

2. **How should "highlights" work?** Options:
   - Stored as a repo-level setting (e.g., `wiki_config` column on `git_repos`)
   - Passed as parameters to the generation endpoint
   - Inferred automatically from repo structure (e.g., directories named `docs/`, `examples/`)

3. **Should wiki pages participate in search/embeddings?** If yes, we'd need to create embeddings for wiki page content — similar to how snippet summaries get embedded. This would make wiki content searchable via the existing search API.

4. **Wiki page cross-commit stability**: When regenerated at a new commit, should slugs remain stable? The Phase 1 outline would need awareness of the previous wiki structure to maintain URL stability.
