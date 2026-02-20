# Plan: Wiki Generation Feature

## Problem Statement

We need a feature that generates a multi-page wiki from a repository. Two challenges:

1. **Storage**: How to represent a structured collection of pages (with hierarchy, ordering, and cross-references) within the existing data model.
2. **Generation**: Building a wiki requires iterating over all files in a repository, which could be very large. We need a strategy that stays within token budgets while producing coherent, comprehensive documentation.

---

## Part 1: Storage Design

### Approach: Wiki as a single enrichment

A wiki is conceptually the same as other enrichments — an immutable, generated document derived from repository content. The cookbook enrichment is already a multi-section structured document stored as one enrichment. The wiki follows the same pattern, just with more explicit hierarchy.

The wiki is stored as **one enrichment** in the existing `enrichments_v2` table:

```
Type:    "usage"
Subtype: "wiki"
```

The `content` field holds the complete wiki as structured JSON:

```json
{
  "pages": [
    {
      "slug": "overview",
      "title": "Project Overview",
      "position": 0,
      "content": "# Project Overview\n\nThis project...",
      "children": [
        {
          "slug": "installation",
          "title": "Installation",
          "position": 0,
          "content": "# Installation\n\nTo install..."
        }
      ]
    },
    {
      "slug": "architecture",
      "title": "Architecture",
      "position": 1,
      "content": "# Architecture\n\n...",
      "children": []
    }
  ]
}
```

Associated to the commit via the standard `enrichment_associations` table — identical to how cookbook or architecture enrichments work today.

### Why this works

1. **The wiki is generated as a complete unit** (Phase 1 plan → Phase 2 pages → Phase 3 index). It's never partially updated — it's regenerated wholesale. This matches the immutable enrichment model.

2. **The cookbook is already the same concept** — a multi-section structured document stored as a single enrichment. The wiki is just larger with more explicit hierarchy.

3. **No schema changes** — no migration, no new table, no new store. Just a new subtype constant and a domain object that parses the JSON content.

4. **All existing infrastructure works** — commit associations, deletion cascading, rescan cleanup, the enrichment service's `List`/`Count`/`DeleteBy`.

### Trade-off

Individual pages aren't queryable at the DB level — the application always fetches the full wiki JSON and filters in memory. Since wikis are rendered with a navigation sidebar (requiring the full tree anyway), this is acceptable. A single-page API endpoint just does a slug lookup in the parsed tree.

### Domain layer

```
domain/wiki/
├── wiki.go          # Wiki value object (parsed from enrichment JSON content)
└── page.go          # Page value object (slug, title, content, position, children)
```

`Wiki` is a domain object that wraps the parsed JSON structure. It is not a persistence entity — it's constructed by deserializing an enrichment's content:

```go
type Wiki struct {
    pages []Page
}

type Page struct {
    slug     string
    title    string
    content  string
    position int
    children []Page
}
```

Key methods on `Wiki`:
- `Pages()` — returns the full page tree
- `Page(slug)` — finds a single page by slug (searches tree)
- `JSON()` — serializes back to JSON for storage in enrichment content

### Enrichment integration

Add to `domain/enrichment/usage.go`:

```go
SubtypeWiki Subtype = "wiki"
```

Add a constructor:

```go
func NewWiki(content string) Enrichment {
    return NewEnrichment(TypeUsage, SubtypeWiki, EntityTypeCommit, content)
}
```

### Service layer

No new service needed. The existing `application/service/enrichment.go` handles CRUD. The API layer fetches the wiki enrichment via the enrichment service (filtering by `type=usage, subtype=wiki, commitSHA=X`) and parses the JSON into the `wiki.Wiki` domain object.

### API layer

```
GET /api/v1/repositories/{id}/wiki              # Full page tree (parsed from enrichment)
GET /api/v1/repositories/{id}/wiki/{slug}        # Single page (slug lookup in parsed tree)
POST /api/v1/repositories/{id}/wiki/generate     # Trigger wiki generation
```

These endpoints fetch the wiki enrichment, deserialize the content JSON into `wiki.Wiki`, and return the requested data. Thin layer — no new service required.

---

## Part 2: Wiki Generation Strategy

### Multi-pass approach

Wiki generation is a **deterministic, multi-pass pipeline** — not an autonomous agent. This keeps token usage predictable and the process debuggable.

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
3. **Collect**: Hold the generated page content in memory.

**Token budget per page**: ~15K input context + ~2K output = ~17K tokens. For a 10-page wiki, that's ~170K tokens total.

**Cross-referencing**: The system prompt tells the LLM about all other page slugs and titles so it can create `[link text](slug)` references.

#### Phase 3: Generate index page (1 LLM call, optional)

Generate a root "index" page that serves as the wiki home. Input is the outline + first paragraph of each page. This provides a cohesive introduction.

#### Phase 4: Assemble and save (no LLM call)

Combine all generated pages into a single `wiki.Wiki` object, serialize to JSON, and save as one enrichment with a commit association. This is a single `enrichmentStore.Save()` + `associationStore.Save()` — the same pattern as every other enrichment handler.

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
- **Content-addressed dedup**: If a wiki's source enrichments haven't changed, skip regeneration. Check whether the existing wiki enrichment's commit SHA matches the current HEAD.

### Task integration

Add a new operation to the task queue:

```go
OperationGenerateWikiForCommit Operation = "kodit.commit.generate_wiki"
```

The handler (`application/handler/enrichment/wiki.go`) follows the standard pattern:
1. Check if wiki enrichment already exists for this commit (find by `type=usage, subtype=wiki, commitSHA`)
2. If so, skip
3. Otherwise, run Phase 1 → Phase 2 → Phase 3 → Phase 4
4. Save the single enrichment + commit association
5. Update tracker

This operation goes at the end of `ScanAndIndexCommit()` — it depends on other enrichments being generated first (architecture, API docs, cookbook).

### Making it deterministic (not an agent)

- **Phase 1** is deterministic: same inputs → same outline (low temperature, structured JSON output)
- **Phase 2** is embarrassingly parallel: each page is independent
- **No tool use, no loops, no branching**: The LLM just generates text given context
- **Predictable cost**: Exactly `1 + N + 1` LLM calls where N is the number of pages

The only "iterative" part is processing pages one-by-one, but this is a simple loop — not an agent deciding what to do next.

---

## Part 3: Implementation Steps

1. **Domain layer**: `domain/wiki/wiki.go` and `domain/wiki/page.go` — value objects for the parsed wiki structure, with JSON serialization/deserialization
2. **Enrichment subtype**: Add `SubtypeWiki` to `domain/enrichment/usage.go` with `NewWiki()` constructor
3. **Handler**: `application/handler/enrichment/wiki.go` — the Phase 1-4 pipeline, produces one enrichment
4. **Task operation**: `OperationGenerateWikiForCommit` in `domain/task/operation.go`, add to `ScanAndIndexCommit()`
5. **Context gatherer**: `infrastructure/enricher/wiki_context.go` — gathers file tree, reads files, collects existing enrichments for the LLM prompts
6. **API endpoints**: Wiki listing + single page + trigger generation in `infrastructure/api/v1/`
7. **Tests**: Unit tests for domain wiki parsing, handler pipeline, API endpoints

---

## Part 4: Open Questions

1. **Should the wiki be regenerated on every commit, or only on demand?** Adding it to `ScanAndIndexCommit()` means automatic regeneration. This is convenient but adds ~180K tokens per commit scan. An on-demand `POST /wiki/generate` might be more appropriate given the cost.

2. **How should "highlights" work?** Options:
   - Stored as a repo-level setting (e.g., `wiki_config` column on `git_repos`)
   - Passed as parameters to the generation endpoint
   - Inferred automatically from repo structure (e.g., directories named `docs/`, `examples/`)

3. **Should wiki content participate in search/embeddings?** The wiki is a single enrichment, so embedding the whole thing as one document would be too coarse. Options:
   - Don't embed the wiki (it synthesizes other enrichments that are already embedded)
   - Split pages into separate embedding documents at indexing time (without separate DB rows)
   - Defer this to a future iteration

4. **Wiki cross-commit stability**: When regenerated at a new commit, should slugs remain stable? The Phase 1 outline would need awareness of the previous wiki structure to maintain URL stability.
