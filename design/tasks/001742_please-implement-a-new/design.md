# Design: Document Text Extraction Pipeline Step

## Codebase Patterns Discovered

- **Language:** Go monolith with domain-driven architecture (`domain/`, `application/`, `infrastructure/`)
- **Pipeline steps** are `task.Operation` constants registered in `handlers.go`, wired into the DAG in `application/service/pipeline.go` via `ragSteps()` and `defaultSteps()`
- **Rasterizer registry** (`infrastructure/rasterization/rasterizer.go`) maps extensions to `Rasterizer` implementations (currently only PDFium for `.pdf`). Interface: `PageCount(path) (int, error)`, `Render(path, page) (image.Image, error)`
- **Text extraction** uses tabula (`infrastructure/extraction/document.go`) which treats the whole file as one blob of text. No per-page/slide extraction exists
- **Enrichments** (`domain/enrichment/`) have type + subtype. Existing subtypes: `chunk`, `page_image`, `snippet`, etc. Each enrichment gets associations (commit, file, repo) and a `SourceLocation` (line range or page number)
- **File URIs** support `?mode=raster&page=N` for image rendering. The mode is validated in `internal/mcp/server.go:handleReadFile()` which dispatches to `handleRasterRead()`
- **Handler pattern:** Struct with stores + dependencies, `Execute(ctx, payload) error`. Registered in `handlers.go` with `c.registry.Register(operation, handler)`
- **Cleanup wrappers:** `handler.WithCleanup(handler, cleanup)` wraps handlers with enrichment cleanup on rescan

## Architecture

### New OOXML Text Extractor

Create `infrastructure/extraction/ooxml.go` implementing a new `OOXMLText` type that opens OOXML files as zip archives and extracts per-page text.

```
type PageText struct {
    Page    int
    Text    string
    Media   []string  // relative paths within the zip (PPTX only)
}

type OOXMLText struct{}

func (o *OOXMLText) Pages(path string) ([]PageText, error)
func (o *OOXMLText) PageCount(path string) (int, error)
```

**PPTX extraction:**
1. Open as `archive/zip`
2. List `ppt/slides/slide*.xml` entries, sorted by slide number
3. For each slide, parse XML to extract text from `<a:t>` elements (these contain all visible text in placeholders, titles, body, shapes)
4. Parse `ppt/notesSlides/notesSlide*.xml` for speaker notes (match by slide number)
5. Parse `ppt/slides/_rels/slide*.xml.rels` to find referenced media files (`image*.png`, `image*.jpg`, etc.)
6. Return one `PageText` per slide

**DOCX extraction:**
1. Open as `archive/zip`
2. Parse `word/document.xml`
3. Extract text from `<w:t>` elements
4. Split on `<w:lastRenderedPageBreak/>` and `<w:br w:type="page"/>` markers
5. If no page breaks, return one `PageText` for the whole document
6. Speaker notes / media: not applicable for DOCX (the chunk pipeline already handles DOCX text well; page text is supplementary)

**XLSX extraction:**
1. Open as `archive/zip`
2. Parse `xl/sharedStrings.xml` to build the shared strings table
3. For each `xl/worksheets/sheet*.xml`, extract cell values by resolving shared string references
4. Each sheet becomes one `PageText`

**Decision:** Use Go's `archive/zip` + `encoding/xml` directly rather than a third-party OOXML library. The extraction is read-only and only needs text content, not formatting. This avoids a new dependency and keeps the extractor under our control. The XML structures for text content are well-documented and stable across Office versions.

### New Enrichment Subtype

Add `SubtypePageText Subtype = "page_text"` to `domain/enrichment/enrichment.go`.

Add constructor `NewPageText(content string) Enrichment` to `domain/enrichment/development.go`.

Unlike `page_image` enrichments (which store empty content and render on demand), `page_text` enrichments store the extracted text as their `content` field. This allows:
- Direct semantic search over the text
- Returning text via the API without re-extracting from the file

### New Pipeline Operations

Add two new operations to `domain/task/operation.go`:
```go
OperationExtractPageTextForCommit            Operation = "kodit.commit.extract_page_text"
OperationCreatePageTextEmbeddingsForCommit   Operation = "kodit.commit.create_page_text_embeddings"
```

### Pipeline Step Placement

In `application/service/pipeline.go`, add to `ragSteps()`:
```
extract_page_text       depends on: extract_snippets
create_page_text_embeddings  depends on: extract_page_text
```

This mirrors the existing vision pipeline (`extract_page_images` -> `create_page_image_embeddings`) and runs in parallel with the vision steps since both depend on `extract_snippets`.

### Handler: ExtractPageText

New file: `application/handler/indexing/extract_page_text.go`

Follows the same pattern as `ExtractPageImages`:
1. Check for existing `page_text` enrichments for the commit (skip if already done)
2. Iterate commit files, filter to OOXML extensions
3. For each file, call `OOXMLText.Pages()` to get per-page text
4. Create one `page_text` enrichment per page with the text as content
5. Save `SourceLocation` with page number
6. Save commit, file, and repository associations

For PPTX, store slide-scoped media paths in the enrichment content as a structured footer:
```
[slide text content here]

---
Media: image1.png, image3.png
```

This keeps the media references queryable without a separate store. The `---\nMedia:` line is a convention the retrieval layer can parse.

### Handler: CreatePageTextEmbeddings

New file: `application/handler/indexing/create_page_text_embeddings.go`

Follows the pattern of `CreateCodeEmbeddings`:
1. Find all `page_text` enrichments for the commit
2. Filter out enrichments that already have embeddings
3. Batch-embed the text content using the code embedding provider
4. Save embeddings to the code embedding store (same store as snippets/chunks)

These embeddings land in the same vector space as code snippet embeddings, so semantic search automatically picks them up.

### Text Mode in File URIs

In `internal/mcp/server.go`:

1. Update the mode validation to accept `"text"` alongside `"raster"`:
   ```go
   if mode != "" && mode != "raster" && mode != "text" {
       return nil, fmt.Errorf("unsupported mode %q, valid modes: raster, text", mode)
   }
   ```

2. Add `handleTextRead()` method that:
   - Resolves the disk path
   - Calls `OOXMLText.Pages()` to extract text
   - If `page` param is set, returns that specific page's text
   - If `page` param is omitted, returns all pages concatenated with `\n--- Page N ---\n` separators
   - Returns as `TextResourceContents` with `text/plain` MIME type

3. Update `FileURI.String()` to support `mode=text`:
   - Add `WithTextMode()` method that builds `?page=N&mode=text` URIs
   - Used by search result formatting to point to text content

### Search Result Integration

When a semantic search hit is a `page_text` enrichment, the result URI should use `mode=text` instead of `mode=raster`. The `resolveFileResults` method in the MCP server already resolves enrichments to file URIs; it needs to check the enrichment subtype and use `WithTextMode()` for `page_text` results.

### API Endpoint

The existing `GET /api/v1/enrichments` endpoint already supports filtering by `enrichment_subtype=page_text`. No new endpoints needed. The enrichment content contains the extracted text directly.

## Constraints and Gotchas

- **Page breaks in DOCX are unreliable.** Word inserts `<w:lastRenderedPageBreak/>` based on the *last render*, which depends on fonts, margins, and printer settings that may not match the extraction environment. Document this limitation: page numbers in DOCX are approximate.
- **Charts and SmartArt in PPTX** are stored as separate XML (`ppt/charts/chart1.xml`, SmartArt XML). Text within these is extractable from the XML, but the visual representation may not match. The cached PNG (if present) is already handled by the raster pipeline. We extract text from chart/SmartArt XML on a best-effort basis.
- **XLSX cell formatting** (dates, currency, percentages) is stored as format codes in `xl/styles.xml`. We extract raw cell values without applying formatting. Formatted display values aren't stored in the XML unless the file was saved with `calcOnSave`.
- **Large files:** The existing 100 MB limit from `DocumentText` applies. Very large Excel files with millions of rows could produce enormous text; consider a per-page text size cap (e.g. 50KB per sheet).
- **Existing tabula extraction** for BM25/chunks continues unchanged. The new OOXML extractor provides *per-page* text that tabula cannot, and serves a different purpose (retrieval context for LLMs, not chunking).
