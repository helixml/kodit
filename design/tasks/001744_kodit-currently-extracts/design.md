# Design: Text Rasterisation Mode for Read File API

## Overview

Add `mode=text` to the read file API (HTTP + MCP) that uses tabula to extract readable text from document pages. This parallels the existing `mode=raster` which renders pages as images.

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
2. Validate file size ≤ 100 MB
3. Validate page ≥ 1
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
