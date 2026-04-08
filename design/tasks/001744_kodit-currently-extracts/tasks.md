# Implementation Tasks

- [ ] Create `PageText` struct in `infrastructure/extraction/page_text.go` with `PageCount(path) (int, error)` and `Text(path, page) (string, error)` methods
- [ ] Write tests for `PageText` covering PDF, XLSX, PPTX, DOCX page extraction and error cases (invalid page, unsupported format, oversized file)
- [ ] Add `renderTextPage()` method to `RepositoriesRouter` in `infrastructure/api/v1/repositories.go` — handles disk path resolution, page count, text extraction, line filtering, and response writing
- [ ] Update `GetBlob()` in `repositories.go` to accept `mode=text`, route to `renderTextPage()`, and allow `page` param with `mode=text`
- [ ] Update swagger annotations on `GetBlob` for the new mode and page count response
- [ ] Add `handleTextRead()` method to MCP `Server` in `internal/mcp/server.go` — handles page count and text extraction via file URIs
- [ ] Update `handleReadFile()` in `server.go` to accept `mode=text` and route to `handleTextRead()`
- [ ] Wire `PageText` into `Client` / app startup and pass to both HTTP router and MCP server
- [ ] Write integration tests for the HTTP endpoint with `mode=text` (page extraction, page count, line numbers, line filtering, error cases)
- [ ] Write integration tests for the MCP resource with `mode=text`
