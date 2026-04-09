# Add mode=text to read file API for document text extraction

## Summary
Adds a new `mode=text` rasterisation mode to the read file API (HTTP and MCP) that extracts readable text from document pages using tabula. This allows LLMs and API consumers to read document content without requiring vision capabilities, using the same page-based addressing already used by search results.

## Changes
- New `TextRenderer` interface and `TextRendererRegistry` in `infrastructure/extraction/`, mirroring the existing `Rasterizer` pattern
- Four implementations: `PDFTextRenderer`, `XLSXTextRenderer`, `PPTXTextRenderer`, `SinglePageTextRenderer` (DOCX/ODT/EPUB)
- HTTP API: `GET /repositories/{id}/blob/{blob_name}/{path}?mode=text&page=N` returns extracted text; without `page` returns `{"page_count":N}`
- MCP: `file://{id}/{blob_name}/{path}?mode=text&page=N` returns text content via `kodit_read_resource`
- Line numbers and line filtering (`lines`, `line_numbers` params) work with text mode
- Wired into `kodit.go`, HTTP router, and MCP server following existing rasterizer patterns

## Testing
- Unit tests for all TextRenderer implementations (error paths, registry operations)
- MCP integration tests for text mode (page count, page extraction, line numbers, unsupported extensions, invalid modes)
- All existing tests pass without regressions
