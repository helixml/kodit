# Design: Text Rasterisation Mode for Read File API

## Overview

Add `mode=text` to the read file API (HTTP + MCP) that uses tabula to extract readable text from document pages. This parallels the existing `mode=raster` which renders pages as images.

## Alignment with Search/Embedding Results

The indexing pipeline stores document content in two ways, tracked by `SourceLocation`:

| Content type | Location reference | How it's indexed | How search results link back |
|---|---|---|---|
| Code chunks | `startLine` / `endLine` | BM25 + code embeddings | `?lines=L5-L20&line_numbers=true` |
| Document pages | `page` (1-based) | Vision embeddings (rasterized image) | `?page=N&mode=raster` |

Today, when a search result references a document page, the link points to `?page=N&mode=raster` — which returns a JPEG image. There is no way to get the **text** of that same page. The new `mode=text` closes this gap:

- Search result says page 3 → `?page=3&mode=raster` returns the image (existing)
- Search result says page 3 → `?page=3&mode=text` returns the extracted text (new)

**The `page` parameter is already the canonical unit for document content referencing across indexing, search results, and the read API.** The new `mode=text` reuses this same `page` parameter with the same 1-based semantics, so consumers can trivially switch between image and text views of search results.

Concretely, these are the touchpoints that use `page`:
- `SourceLocation.Page()` — stores 1-based page number per enrichment (`domain/sourcelocation/source_location.go`)
- `SnippetContentSchema.Page` — returned in HTTP search results (`infrastructure/api/v1/dto/search.go:48`)
- `fileResult.Page` — returned in MCP search results (`internal/mcp/server.go:536`)
- `FileURI.WithPage()` — builds `?page=N&mode=raster` URIs for MCP (`internal/mcp/file_uri.go:32`)
- `SnippetLinks.File` — builds `?page=N&mode=raster` links for HTTP search results (`infrastructure/api/v1/search.go:864`)

The new `mode=text` slots into all of these without changing the page-based addressing — only the rendering mode changes.

### Line numbers within text mode

Once `mode=text` extracts a page's text, the existing `lines` and `line_numbers` query parameters apply **within that page's text**. These are ephemeral line numbers (not stored in the index) and are useful for LLMs to reference specific positions within a page. This is the same pattern used for code files today.

## Key Findings from Codebase Exploration

- **Tabula** (`github.com/tsawler/tabula` v1.6.6) already supports per-page extraction:
  - **PDF**: `tabula.Open(path).Pages(N).ExcludeHeadersAndFooters().JoinParagraphs().Text()` — 1-indexed
  - **XLSX**: `xlsxReader.TextWithOptions(ExtractOptions{Sheets: []int{N}})` — 0-indexed
  - **PPTX**: `pptxReader.TextWithOptions(ExtractOptions{SlideNumbers: []int{N}})` — 0-indexed
  - **DOCX/ODT**: No per-page support — `PageCount()` returns 1, entire document is one "page"
  - **EPUB**: `PageCount()` returns chapter count, no per-chapter text extraction via reader
- **Note**: The top-level `Extractor.Pages()` method only applies to PDFs. For XLSX/PPTX, the per-page options must be passed through the format-specific reader options.
- **Existing infrastructure**: `extraction.DocumentText` currently extracts entire documents. `rasterization.Cache` demonstrates the filesystem caching pattern (SHA256 hash-based).
- **Existing code paths**: Both `repositories.go:GetBlob()` (HTTP) and `server.go:handleReadFile()` (MCP) already parse `mode` and `page` query params and validate them. Adding `mode=text` means extending the validation and adding a new handler branch.
- **`Blobs.DiskPath()`** resolves repo+blob+path to an absolute filesystem path — already used by `mode=raster`.

## Architecture

### Interface + Registry Pattern (Mirroring Rasterization)

The existing rasterization layer follows a clean pattern:

```
rasterization.Rasterizer (interface)     → PageCount, Render, Close
rasterization.PdfiumRasterizer (impl)    → PDF-specific implementation
rasterization.Registry                   → maps extensions to Rasterizer impls
```

The text extraction layer mirrors this exactly, in a new `extraction` subpackage:

```
extraction.TextRenderer (interface)      → PageCount, Render, Close
extraction.PDFTextRenderer (impl)        → PDF via tabula Pages()
extraction.XLSXTextRenderer (impl)       → XLSX via tabula Sheet()
extraction.PPTXTextRenderer (impl)       → PPTX via tabula Slide()
extraction.SinglePageTextRenderer (impl) → DOCX/ODT/EPUB (always 1 page)
extraction.Registry                      → maps extensions to TextRenderer impls
```

#### Interface: `TextRenderer`

```go
// TextRenderer extracts text from individual document pages.
// For PDFs this means pages; for spreadsheets, sheets; for presentations, slides.
type TextRenderer interface {
    io.Closer

    // PageCount returns the number of extractable pages in the document.
    PageCount(path string) (int, error)

    // Render returns the text content of the given 1-based page.
    Render(path string, page int) (string, error)
}
```

This mirrors `rasterization.Rasterizer` exactly — same method names (`PageCount`, `Render`, `Close`), same 1-based page convention — but returns `string` instead of `image.Image`.

#### Registry: `extraction.Registry`

```go
// Registry maps file extensions to TextRenderer implementations.
type Registry struct {
    renderers map[string]TextRenderer
}

func NewRegistry() *Registry
func (r *Registry) Register(ext string, renderer TextRenderer)
func (r *Registry) For(ext string) (TextRenderer, bool)
func (r *Registry) Supports(ext string) bool
func (r *Registry) Close() error
```

Identical structure to `rasterization.Registry`.

#### Implementations

**`PDFTextRenderer`** — Extracts text from a specific PDF page:
```go
func (r *PDFTextRenderer) Render(path string, page int) (string, error) {
    text, _, err := tabula.Open(path).
        Pages(page).
        ExcludeHeadersAndFooters().
        JoinParagraphs().
        Text()
    return text, err
}
```

**`XLSXTextRenderer`** — Extracts text from a specific sheet (0-indexed internally, 1-indexed API):
```go
func (r *XLSXTextRenderer) Render(path string, page int) (string, error) {
    xr, err := xlsx.Open(path)
    // ...
    return xr.TextWithOptions(xlsx.ExtractOptions{Sheets: []int{page - 1}})
}
```

**`PPTXTextRenderer`** — Extracts text from a specific slide:
```go
func (r *PPTXTextRenderer) Render(path string, page int) (string, error) {
    pr, err := pptx.Open(path)
    // ...
    return pr.TextWithOptions(pptx.ExtractOptions{
        SlideNumbers: []int{page - 1},
        IncludeNotes: true,
        IncludeTitles: true,
    })
}
```

**`SinglePageTextRenderer`** — For DOCX, ODT, EPUB (entire document as page 1):
```go
func (r *SinglePageTextRenderer) PageCount(path string) (int, error) { return 1, nil }
func (r *SinglePageTextRenderer) Render(path string, page int) (string, error) {
    if page != 1 {
        return "", fmt.Errorf("page %d out of range (1-1)", page)
    }
    text, _, err := tabula.Open(path).
        ExcludeHeadersAndFooters().
        JoinParagraphs().
        Text()
    return text, err
}
```

#### Wiring in `kodit.go`

Mirrors how `rasterization.Registry` is set up today:

```go
// Existing (rasterization)
rasterizers := rasterization.NewRegistry()
rasterizers.Register(".pdf", pdfRast)

// New (text extraction)
textRenderers := extraction.NewRegistry()
textRenderers.Register(".pdf", extraction.NewPDFTextRenderer())
textRenderers.Register(".xlsx", extraction.NewXLSXTextRenderer())
textRenderers.Register(".pptx", extraction.NewPPTXTextRenderer())
textRenderers.Register(".docx", extraction.NewSinglePageTextRenderer())
textRenderers.Register(".odt", extraction.NewSinglePageTextRenderer())
textRenderers.Register(".epub", extraction.NewSinglePageTextRenderer())
```

The registry is passed to the HTTP router and MCP server alongside the rasterizer registry.

### No Caching

Unlike rasterization (which renders pixels), text extraction is fast and produces small output. No filesystem cache is needed initially. If performance becomes an issue, the rasterization cache pattern can be replicated.

### API Changes

**HTTP API** (`infrastructure/api/v1/repositories.go`):

Extend `GetBlob()`:
- Update mode validation: `mode != "" && mode != "raster" && mode != "text"` → error
- When `mode == "text"`, call new `renderTextPage()` method
- `renderTextPage()` follows the same pattern as `renderRasterPage()`:
  1. Look up `TextRenderer` from registry by file extension
  2. Resolve disk path via `Blobs.DiskPath()`
  3. If no `page` param: return JSON `{"page_count": N}` via `renderer.PageCount()`
  4. If `page` param: extract text via `renderer.Render()`, apply line filter if requested, write `text/plain` response

**MCP Server** (`internal/mcp/server.go`):

Extend `handleReadFile()`:
- Update mode validation to accept `"text"`
- When `mode == "text"`, call new `handleTextRead()` method
- `handleTextRead()` looks up `TextRenderer` from registry, returns `TextResourceContents`
- Page count returned as text: `"Page count: N"`

**MCP FileURI** (`internal/mcp/file_uri.go`):

No changes needed. The existing `WithPage()` builds `?page=N&mode=raster` for search results. MCP clients that want text can replace `mode=raster` with `mode=text`.

### Swagger Annotations

Update the `GetBlob` handler's `@Param mode` annotation to document `text` as a valid mode value. Add `@Param page` clarification that it works with both `raster` and `text` modes.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Architecture pattern | Interface + Registry, mirroring `rasterization.Rasterizer` | Consistent with existing codebase; extensible for new formats |
| Interface name | `TextRenderer` with `Render()` returning `string` | Mirrors `Rasterizer.Render()` returning `image.Image` |
| Package location | `infrastructure/extraction/` | Alongside existing `DocumentText`; extraction is the domain |
| Implementations | 4 types: PDF, XLSX, PPTX, SinglePage | Format-specific tabula APIs require separate implementations |
| Caching | None | Text extraction is fast; add later if needed |
| Page count endpoint | `mode=text` without `page` | Consistent with raster mode's requirement for `page`; reuses same param space |
| Line numbers/filtering | Reuse existing `LineFilter` | Already works on `[]byte` content, no changes needed |
| Search result links | Keep pointing to `mode=raster` | Vision embeddings are the primary indexing path for documents; text is opt-in |
| Page parameter semantics | 1-based, same as `mode=raster` | Consistent with `SourceLocation.Page()`, search result DTOs, and `FileURI` |

## Implementation Notes

- **File layout**: Each TextRenderer implementation is in its own file: `pdf_text_renderer.go`, `xlsx_text_renderer.go`, `pptx_text_renderer.go`, `single_page_text_renderer.go`. The interface and registry are in `text_renderer.go`.
- **Shared validation**: `validateDocumentPath()` is defined in `pdf_text_renderer.go` and reused by all implementations (checks file exists and size <= 100MB).
- **Wiring pattern**: Followed the exact same pattern as rasterizers — `TextRendererRegistry` created in `kodit.go:New()`, stored on the `Client` struct, exposed via `TextRenderers()` accessor, and passed to MCP server via `WithTextRendering()` option.
- **HTTP API**: `renderTextPage()` in `repositories.go` mirrors `renderRasterPage()` — same disk path resolution, same line filter reuse. Without `page` param it returns JSON `{"page_count":N}`; with `page` it returns `text/plain`.
- **MCP server**: `handleTextRead()` in `server.go` mirrors `handleRasterRead()`. Without `page` it returns `"Page count: N"`; with `page` it returns text content. Both support `lines` and `line_numbers` query params.
- **CGO limitation**: The CI/test environment lacks `gcc`, so `CGO_ENABLED=0 go build/test` was used. The `infrastructure/api/v1` tests require sqlite (CGO) and were verified to have pre-existing failures unrelated to this change.
- **No test fixtures for real documents**: Following the existing pattern in `document_test.go`, tests cover error paths (missing file, oversized, invalid page) using fakes. MCP tests use `fakeTextRenderer` and `fakeDiskPathResolver` to test the full request flow.
