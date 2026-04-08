# Implementation Tasks

## Domain Layer

- [ ] Add `SubtypePageText Subtype = "page_text"` to `domain/enrichment/enrichment.go`
- [ ] Add `NewPageText(content string) Enrichment` constructor to `domain/enrichment/development.go`
- [ ] Add `IsPageText(e Enrichment) bool` helper to `domain/enrichment/development.go`
- [ ] Add `OperationExtractPageTextForCommit` and `OperationCreatePageTextEmbeddingsForCommit` to `domain/task/operation.go`
- [ ] Add the two new operations to `PrescribedOperations.All()` (in the Vision section, alongside page images)

## OOXML Text Extractor

- [ ] Create `infrastructure/extraction/ooxml.go` with `OOXMLText` type and `PageText` struct
- [ ] Implement `Pages(path string) ([]PageText, error)` dispatcher that routes by extension
- [ ] Implement PPTX slide text extraction: open zip, parse `ppt/slides/slide*.xml`, extract `<a:t>` text elements
- [ ] Implement PPTX speaker notes extraction: parse `ppt/notesSlides/notesSlide*.xml` and correlate to slides
- [ ] Implement PPTX media reference parsing: parse `ppt/slides/_rels/slide*.xml.rels` to build slide-to-media map
- [ ] Implement DOCX page text extraction: parse `word/document.xml`, extract `<w:t>` elements, split on page break markers
- [ ] Implement XLSX sheet text extraction: parse `xl/sharedStrings.xml` and `xl/worksheets/sheet*.xml`
- [ ] Implement `PageCount(path string) (int, error)` method
- [ ] Add size limit check (100 MB, matching existing document extraction)
- [ ] Write tests for each format using small test fixtures (create minimal docx/pptx/xlsx files in `testdata/`)

## Pipeline Handlers

- [ ] Create `application/handler/indexing/extract_page_text.go` following `ExtractPageImages` pattern
- [ ] In the handler: iterate commit files, filter to `.docx`/`.pptx`/`.xlsx`, call `OOXMLText.Pages()`, create `page_text` enrichments with source locations and associations
- [ ] For PPTX: append `---\nMedia: file1.png, file2.png` footer to enrichment content
- [ ] Write tests for `ExtractPageText` handler
- [ ] Create `application/handler/indexing/create_page_text_embeddings.go` following `CreateCodeEmbeddings` pattern
- [ ] In the handler: find `page_text` enrichments, filter already-embedded, batch-embed text, save to code embedding store
- [ ] Write tests for `CreatePageTextEmbeddings` handler

## Pipeline Registration

- [ ] Register `ExtractPageText` handler in `handlers.go` with `task.OperationExtractPageTextForCommit`
- [ ] Register `CreatePageTextEmbeddings` handler in `handlers.go` with `task.OperationCreatePageTextEmbeddingsForCommit`
- [ ] Wrap both with `handler.WithCleanup` using appropriate enrichment cleanup
- [ ] Add both operations to `ragSteps()` in `application/service/pipeline.go`: `extract_page_text` depends on `extract_snippets`, `create_page_text_embeddings` depends on `extract_page_text`
- [ ] The same steps are automatically included in `defaultSteps()` since it calls `ragSteps()`

## File URI Text Mode

- [ ] Update mode validation in `internal/mcp/server.go:handleReadFile()` to accept `mode=text`
- [ ] Implement `handleTextRead()` in `internal/mcp/server.go` that calls `OOXMLText.Pages()` and returns text content
- [ ] Handle `page` param: specific page returns one page's text; no page returns all pages concatenated with separators
- [ ] Add `WithTextMode()` method to `internal/mcp/file_uri.go` that builds `?page=N&mode=text` URIs
- [ ] Update MCP instructions string to document `mode=text` alongside `mode=raster`

## Search Integration

- [ ] In `resolveFileResults()` (MCP server), check enrichment subtype: use `mode=text` URI for `page_text` enrichments instead of `mode=raster`
- [ ] Verify that `page_text` enrichments appear in semantic search results (they use the same code embedding store)

## API / Swagger

- [ ] Add swag annotation updates to any modified API handlers (if the enrichments endpoint docs need updating for the new subtype)

## Testing

- [ ] Create minimal OOXML test fixtures in a `testdata/` directory (one small docx, pptx, xlsx)
- [ ] Run `make test` to verify all tests pass
- [ ] Run `make check` to verify lint/vet pass
