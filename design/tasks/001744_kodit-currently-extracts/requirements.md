# Requirements: Text Rasterisation Mode for Read File API

## Context: Alignment with Indexing and Search

Documents are indexed per-page: each page/slide/sheet gets a `SourceLocation` with a 1-based `page` number. Search results return this `page` number and link to `?page=N&mode=raster` for the rendered image. The new `mode=text` uses the **same page parameter** so that consumers can read the text of any page returned by search — no translation between coordinate systems is needed.

## User Stories

### US-1: Read document text via API
As an API consumer, I want to read the extracted text of a specific document page/slide/sheet so that I can pass readable content to LLMs without needing vision capabilities.

**Acceptance Criteria:**
- `GET /api/v1/repositories/{id}/blob/{blob_name}/{path}?mode=text&page=1` returns plain text extracted from the specified page
- Supported formats: PDF, DOCX, ODT, XLSX, PPTX, EPUB
- Page numbering is 1-indexed, consistent with `mode=raster` and `SourceLocation.Page()`
- For single-page formats (DOCX, ODT), `page=1` returns the full document text; other page numbers return an error
- Response Content-Type is `text/plain; charset=utf-8`
- Response includes `X-Commit-SHA` header

### US-2: Read document text via MCP
As an MCP client (e.g., Claude Code), I want to read document text using the file resource URI so that I can understand document content without vision.

**Acceptance Criteria:**
- `file://{id}/{blob_name}/{path}?mode=text&page=1` returns a `TextResourceContents` with extracted text
- Same format support and page semantics as the HTTP API
- A search result returning `page=3` can be read via `?mode=text&page=3` (same page number, different mode)

### US-3: Line numbers and line filtering
As an API/MCP consumer, I want to combine `mode=text` with `line_numbers=true` and `lines=L5-L20` so that I can reference specific sections of extracted text.

**Acceptance Criteria:**
- `?mode=text&page=1&line_numbers=true` prefixes each line with its 1-based line number
- `?mode=text&page=1&lines=L5-L20` returns only lines 5-20 of the extracted page text
- Both parameters can be combined
- Uses the existing `service.NewLineFilter` implementation
- Line numbers are relative to the page text (not the whole document)

### US-4: Page count discovery
As an API/MCP consumer, I want to know how many pages a document has so that I can iterate through all pages.

**Acceptance Criteria:**
- `?mode=text` without a `page` parameter returns the total page count as a JSON object: `{"page_count": N}`
- MCP returns: `"Page count: N"` as text content

### US-5: Error handling
As a consumer, I expect clear errors when I make invalid requests.

**Acceptance Criteria:**
- `mode=text` on a non-document file (e.g., `.go`, `.png`) returns 400 with "text extraction not supported for .ext files"
- `page=0` or negative page returns 400 with "page must be a positive integer"
- `page` exceeding document page count returns 400 with "page N out of range (1-M)"
- Files exceeding 100 MB return 400 with a size limit error
