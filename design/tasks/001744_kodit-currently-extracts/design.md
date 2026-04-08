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
- **Note**: The top-level `Extractor.Pages()` method only applies to PDFs. For XLSX/PPTX, the per-page options must be passed through the format-specific reader options. A clean approach is to call `tabula.Open(path).Pages(N).Text()` and let tabula handle the format dispatch — but this only works for PDF. For other formats, we need to open the reader directly.
- **Existing infrastructure**: `extraction.DocumentText` currently extracts entire documents. `rasterization.Cache` demonstrates the filesystem caching pattern (SHA256 hash-based).
- **Existing code paths**: Both `repositories.go:GetBlob()` (HTTP) and `server.go:handleReadFile()` (MCP) already parse `mode` and `page` query params and validate them. Adding `mode=text` means extending the validation and adding a new handler branch.
- **`Blobs.DiskPath()`** resolves repo+blob+path to an absolute filesystem path — already used by `mode=raster`.

## Architecture

### New Component: `extraction.PageText`

A new struct in `infrastructure/extraction/` that extracts text from a specific page of a document file. This is distinct from `DocumentText` which extracts entire documents for indexing.

```go
// PageText extracts text from individual document pages using tabula.
type PageText struct{}

func (p *PageText) PageCount(path string) (int, error)
func (p *PageText) Text(path string, page int) (string, error)
```

**Why a new type instead of extending `DocumentText`?** `DocumentText.Text()` returns the full document as a single string (designed for indexing/chunking). Per-page extraction has different semantics — it must open format-specific readers and pass page selection options. Keeping them separate follows the single-responsibility principle and avoids booleans switching behaviour.

**Implementation of `Text(path, page)`:**
1. Validate extension is a supported document format (`IsDocument(ext)`)
2. Validate file size <= 100 MB
3. Validate page >= 1
4. Dispatch by format:
   - **PDF**: `tabula.Open(path).Pages(page).ExcludeHeadersAndFooters().JoinParagraphs().Text()`
   - **XLSX**: Open `xlsx.Reader`, call `TextWithOptions(ExtractOptions{Sheets: []int{page - 1}})`
   - **PPTX**: Open `pptx.Reader`, call `TextWithOptions(ExtractOptions{SlideNumbers: []int{page - 1}, IncludeNotes: true, IncludeTitles: true})`
   - **DOCX/ODT/EPUB**: Only `page=1` is valid. Extract full text via `tabula.Open(path).ExcludeHeadersAndFooters().JoinParagraphs().Text()`

### No Caching

Unlike rasterization (which renders pixels), text extraction is fast and produces small output. No filesystem cache is needed initially. If performance becomes an issue, the rasterization cache pattern can be replicated.

### API Changes

**HTTP API** (`infrastructure/api/v1/repositories.go`):

Extend `GetBlob()`:
- Update mode validation: `mode != "" && mode != "raster" && mode != "text"` → error
- When `mode == "text"`, call new `renderTextPage()` method
- `renderTextPage()` follows the same pattern as `renderRasterPage()`:
  1. Resolve disk path via `Blobs.DiskPath()`
  2. If no `page` param: return JSON `{"page_count": N}` via `PageText.PageCount()`
  3. If `page` param: extract text via `PageText.Text()`, apply line filter if requested, write `text/plain` response

**MCP Server** (`internal/mcp/server.go`):

Extend `handleReadFile()`:
- Update mode validation to accept `"text"`
- When `mode == "text"`, call new `handleTextRead()` method
- `handleTextRead()` returns `TextResourceContents` with extracted text
- Page count returned as text: `"Page count: N"`

**MCP FileURI** (`internal/mcp/file_uri.go`):

No changes needed to `FileURI` itself. The existing `WithPage()` method builds `?page=N&mode=raster` URIs for search results, which is correct — search results should continue pointing to the raster view by default (vision embeddings are the primary use case for document pages). MCP clients that want text can replace `mode=raster` with `mode=text` in the URI.

### Wiring

`PageText` is constructed in `kodit.go` (or wherever `Client` is assembled) and passed to both the HTTP router and MCP server. It has no dependencies beyond the tabula library.

### Swagger Annotations

Update the `GetBlob` handler's `@Param mode` annotation to document `text` as a valid mode value. Add `@Param page` clarification that it works with both `raster` and `text` modes.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| New type vs extend `DocumentText` | New `PageText` type | Different responsibility (per-page vs whole-doc), avoids boolean params |
| Caching | None | Text extraction is fast; add later if needed |
| Page count endpoint | `mode=text` without `page` | Consistent with raster mode's requirement for `page`; reuses same param space |
| Line numbers/filtering | Reuse existing `LineFilter` | Already works on `[]byte` content, no changes needed |
| EPUB support | Page = chapter | Tabula treats chapters as pages via `ChapterCount()` |
| Search result links | Keep pointing to `mode=raster` | Vision embeddings are the primary indexing path for documents; text is opt-in |
| Page parameter semantics | 1-based, same as `mode=raster` | Consistent with `SourceLocation.Page()`, search result DTOs, and `FileURI` |
