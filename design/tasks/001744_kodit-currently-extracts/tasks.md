# Implementation Tasks

- [ ] Create `TextRenderer` interface in `infrastructure/extraction/text_renderer.go` with `PageCount(path) (int, error)`, `Render(path, page) (string, error)`, and `io.Closer` — mirroring `rasterization.Rasterizer`
- [ ] Create `extraction.Registry` in `infrastructure/extraction/text_registry.go` mapping extensions to `TextRenderer` implementations — mirroring `rasterization.Registry`
- [ ] Implement `PDFTextRenderer` using `tabula.Open(path).Pages(page).ExcludeHeadersAndFooters().JoinParagraphs().Text()`
- [ ] Implement `XLSXTextRenderer` using `xlsx.Reader.TextWithOptions(ExtractOptions{Sheets: []int{page - 1}})`
- [ ] Implement `PPTXTextRenderer` using `pptx.Reader.TextWithOptions(ExtractOptions{SlideNumbers: []int{page - 1}})`
- [ ] Implement `SinglePageTextRenderer` for DOCX/ODT/EPUB (always 1 page, full document text)
- [ ] Write tests for all `TextRenderer` implementations covering page extraction, page count, out-of-range pages, and error cases
- [ ] Wire up `extraction.Registry` in `kodit.go` — register all implementations by extension, pass to HTTP router and MCP server
- [ ] Add `renderTextPage()` method to `RepositoriesRouter` in `infrastructure/api/v1/repositories.go` — look up renderer from registry, handle page count and text extraction with line filtering
- [ ] Update `GetBlob()` in `repositories.go` to accept `mode=text`, route to `renderTextPage()`, and allow `page` param with `mode=text`
- [ ] Update swagger annotations on `GetBlob` for the new mode and page count response
- [ ] Add `handleTextRead()` method to MCP `Server` in `internal/mcp/server.go` — look up renderer from registry, handle page count and text extraction
- [ ] Update `handleReadFile()` in `server.go` to accept `mode=text` and route to `handleTextRead()`
- [ ] Write integration tests for the HTTP endpoint with `mode=text` (page extraction, page count, line numbers, line filtering, error cases)
- [ ] Write integration tests for the MCP resource with `mode=text`
