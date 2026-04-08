# Implementation Tasks

## Domain Layer

- [ ] Add `ref` string field to `SourceLocation` domain struct with `Ref()` accessor
- [ ] Add `NewRef(enrichmentID int64, ref string) SourceLocation` constructor
- [ ] Update `Reconstruct()` to accept `ref` parameter

## Persistence Layer

- [ ] Add `Ref string` column to `SourceLocationModel` in `infrastructure/persistence/models.go`
- [ ] Update `SourceLocationMapper.ToDomain` and `ToModel` to map the `ref` field
- [ ] Verify GORM AutoMigrate picks up the new column (run `make test`)

## Image Extractor Interface + Registry

- [ ] Create `infrastructure/rasterization/image_extractor.go` with `ImageExtractor` interface (`ImageRefs(path) ([]string, error)`, `Extract(path, ref) (image.Image, error)`) and `ImageExtractorRegistry` (same pattern as `Registry`)

## Office ZIP Implementation

- [ ] Create `infrastructure/rasterization/office.go` implementing `OfficeImageExtractor`
- [ ] Implement `ImageRefs()`: open ZIP, walk entries under `ppt/media/`, `word/media/`, `xl/media/`, filter by image extensions
- [ ] Implement `Extract()`: open ZIP, find entry by ref path, decode to `image.Image`
- [ ] Write tests for `OfficeImageExtractor` with sample docx/pptx/xlsx fixtures

## Handler Changes

- [ ] Add `ImageExtractorRegistry` field to `ExtractPageImages` handler
- [ ] Extend `ExtractPageImages.Execute()`: after rasterizer check, also check image extractor; for supported extensions, call `ImageRefs()` and create enrichments with `sourcelocation.NewRef()`
- [ ] Add `ImageExtractorRegistry` field to `CreatePageImageEmbeddings` handler
- [ ] Extend `CreatePageImageEmbeddings.Execute()`: when source location has `ref != ""`, use image extractor's `Extract()` instead of rasterizer's `Render()`
- [ ] Update handler constructors in `handlers.go` to accept and pass through `ImageExtractorRegistry`

## MCP URI + Server

- [ ] Add `ref` field and `WithRef(ref string)` method to `FileURI` in `internal/mcp/file_uri.go`
- [ ] Update `FileURI.String()` to render `?mode=image&ref=...` when ref is set
- [ ] Add `mode=image` branch in MCP server resource handler (`internal/mcp/server.go`)
- [ ] Implement `handleImageRead()` in MCP server: resolve disk path, get image extractor, extract, encode JPEG q80, return blob
- [ ] Update MCP resource template description to document the new mode

## Wiring

- [ ] Create `OfficeImageExtractor` in `kodit.go` and register for `.docx`, `.pptx`, `.xlsx`
- [ ] Inject `ImageExtractorRegistry` into `ExtractPageImages` and `CreatePageImageEmbeddings` handlers
- [ ] Inject `ImageExtractorRegistry` into MCP server

## Testing

- [ ] Unit tests for `OfficeImageExtractor` (ImageRefs + Extract)
- [ ] Unit tests for extended `ExtractPageImages` handler with image extractor
- [ ] Unit tests for `FileURI.WithRef()` rendering
- [ ] Unit tests for MCP `mode=image` handler
- [ ] Run `make check` to verify everything passes
