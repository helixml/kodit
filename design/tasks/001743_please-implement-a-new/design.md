# Design: Office ZIP Image Extraction Pipeline Step

## Architecture Overview

This feature adds a parallel track to the existing PDF page rasterization pipeline. Where the PDF path uses a `Rasterizer` interface (PageCount + Render by page number), Office documents need a different abstraction — they contain embedded images at known ZIP paths that must be extracted, not rendered.

## Codebase Patterns Discovered

- **Rasterizer registry** (`infrastructure/rasterization/rasterizer.go`): Maps file extensions to `Rasterizer` implementations. PDF is registered in `kodit.go:422-431`.
- **ExtractPageImages handler** (`application/handler/indexing/extract_page_images.go`): Iterates files, checks `rasterizers.Supports(ext)`, creates one enrichment per page with `enrichment.NewPageImage()`, stores `sourcelocation.NewPage(id, pageNum)`.
- **CreatePageImageEmbeddings handler** (`application/handler/indexing/create_page_image_embeddings.go`): Resolves source locations, calls `rast.Render()`, encodes to JPEG quality 80, embeds via vision embedder. Already uses JPEG q80.
- **Pipeline steps** (`application/service/pipeline.go:123-140`): `ragSteps()` includes `ExtractPageImagesForCommit` → `CreatePageImageEmbeddingsForCommit`. Both RAG and default pipelines share these steps.
- **MCP URI** (`internal/mcp/file_uri.go`): `mode=raster&page=N` for PDF pages. Handler at `internal/mcp/server.go:1168-1210` resolves disk path, renders, returns JPEG blob.
- **SourceLocation** (`domain/sourcelocation/source_location.go`): Has `page`, `startLine`, `endLine` fields. DB model at `infrastructure/persistence/models.go:143-150`.
- **Document extensions** (`infrastructure/extraction/document.go:16-23`): `.docx`, `.pptx`, `.xlsx` already recognized as document types (for text extraction via tabula).

## Key Design Decisions

### Decision 1: New `ImageExtractor` interface (not reusing `Rasterizer`)

The `Rasterizer` interface models page-indexed rendering (`PageCount` + `Render(page int)`). Office ZIP images are path-indexed, not page-indexed. Forcing them into the page model would require a fragile mapping between integer indices and ZIP paths, and the user explicitly wants `ref=some/XML/key` in URIs, not `page=N`.

**New interface** in `infrastructure/rasterization/`:
```go
type ImageExtractor interface {
    io.Closer
    ImageRefs(path string) ([]string, error)       // list internal ZIP paths to images
    Extract(path string, ref string) (image.Image, error) // extract one image by ref
}
```

A separate `ImageExtractorRegistry` (same pattern as `Registry`) maps extensions to implementations.

### Decision 2: Add `ref` field to SourceLocation

The `source_locations` table gains a `ref` column (`string`, default empty). GORM AutoMigrate handles the schema change. The domain `SourceLocation` struct gets a `Ref() string` accessor and a `NewRef(enrichmentID int64, ref string)` constructor.

This keeps the existing `page`-based flow untouched and cleanly separates the two use cases.

### Decision 3: Reuse existing pipeline operations (extend, don't duplicate)

Rather than creating entirely new operations (`ExtractZipImagesForCommit`), the existing `ExtractPageImagesForCommit` and `CreatePageImageEmbeddingsForCommit` handlers will be extended to also check the `ImageExtractorRegistry`. This means:

- No new pipeline steps needed — the same two operations handle both PDFs and Office ZIPs.
- No pipeline configuration changes.
- The handlers already iterate all files and check extension support; they just need to also check the image extractor registry.

**Rationale:** The user said "Copy the page rasterizing mode" — same pipeline position, same enrichment type (`page_image`), same embedding flow. The only difference is the extraction mechanism and source location type.

### Decision 4: ZIP image extraction implementation

A single `OfficeImageExtractor` struct handles all three formats. The implementation:

1. Opens the file as a ZIP archive (`archive/zip`)
2. Walks entries looking for image files under known media directories:
   - PPTX: `ppt/media/`
   - DOCX: `word/media/`
   - XLSX: `xl/media/`
3. Filters by image content type (png, jpg, gif, bmp, tiff, wmf, emf)
4. `Extract()` opens the specific ZIP entry and decodes to `image.Image`
5. Caller (handler) converts to JPEG q80

### Decision 5: MCP URI scheme

Add `mode=image` alongside existing `mode=raster`:

```
file://{repoID}/{blob}/{path}?mode=image&ref=ppt/media/image1.png
```

`FileURI` gets a `WithRef(ref string)` method. The MCP server's resource handler adds a branch for `mode=image` that:
1. Resolves disk path
2. Gets `ImageExtractor` for the extension
3. Calls `Extract(diskPath, ref)`
4. Encodes to JPEG q80 and returns as base64 blob

## Component Changes

| File | Change |
|---|---|
| `domain/sourcelocation/source_location.go` | Add `ref` field, `Ref()` accessor, `NewRef()` constructor |
| `infrastructure/persistence/models.go` | Add `Ref string` to `SourceLocationModel` |
| `infrastructure/persistence/mappers.go` | Map `ref` field in `SourceLocationMapper` |
| `infrastructure/rasterization/image_extractor.go` | New `ImageExtractor` interface + `ImageExtractorRegistry` |
| `infrastructure/rasterization/office.go` | New `OfficeImageExtractor` implementation |
| `application/handler/indexing/extract_page_images.go` | Extend to check `ImageExtractorRegistry` alongside `Rasterizer` registry |
| `application/handler/indexing/create_page_image_embeddings.go` | Extend to use `ImageExtractor` when source location has a `ref` instead of `page` |
| `internal/mcp/file_uri.go` | Add `ref` field, `WithRef()`, render `mode=image&ref=...` |
| `internal/mcp/server.go` | Add `mode=image` handler branch |
| `kodit.go` | Create `OfficeImageExtractor`, register for `.docx`, `.pptx`, `.xlsx`, inject into handlers |
