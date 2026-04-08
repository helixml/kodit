# Design: Office Document Image Extraction

## Architecture

This feature adds a single new `Rasterizer` implementation that registers `.docx`, `.pptx`, and `.xlsx` extensions with the existing `rasterization.Registry`. No new pipeline operations, handlers, or enrichment types are needed — the existing page-image pipeline already handles everything once a rasterizer is registered for an extension.

### Data Flow

```
File in repo (.pptx)
  → ExtractPageImages handler (existing)
    → rasterizers.For(".pptx") → OfficeImageExtractor
    → PageCount() = N images in ZIP
    → creates N enrichments (subtype=page_image) + source locations
  → CreatePageImageEmbeddings handler (existing)
    → Render(path, page) → returns decoded image.Image
    → JPEG encode at quality 80 → vision embed → store vector
  → MCP read_resource
    → file://1/main/slides.pptx?page=3&mode=raster
    → handleRasterRead() → Render() → JPEG → base64 blob
```

### New File

**`infrastructure/rasterization/office.go`** — implements `Rasterizer` interface.

```go
type OfficeImageExtractor struct{}

func (o *OfficeImageExtractor) PageCount(path string) (int, error)
func (o *OfficeImageExtractor) Render(path string, page int) (image.Image, error)
func (o *OfficeImageExtractor) Close() error  // no-op
```

**How it works:**

1. Opens the file as a ZIP archive (`archive/zip`).
2. Scans entries for paths matching `word/media/*`, `ppt/media/*`, or `xl/media/*`.
3. Filters to supported image extensions: `.png`, `.jpg`, `.jpeg`, `.gif`, `.bmp`, `.tiff`, `.tif`.
4. Sorts entries by path for deterministic ordering (so page N is stable).
5. `PageCount()` returns the count of matching entries.
6. `Render(path, page)` opens the ZIP, finds the Nth matching entry, decodes it using Go's `image` package (with format-specific imports), and returns the `image.Image`.

**Skipped formats:** EMF, WMF, SVG — Go stdlib has no decoders for these. A warning is logged but they don't count toward `PageCount()`.

### Registration

In `kodit.go`, after the PDF rasterizer registration block:

```go
officeRast := rasterization.NewOfficeImageExtractor()
for _, ext := range []string{".docx", ".pptx", ".xlsx"} {
    rasterizers.Register(ext, officeRast)
}
```

No conditional — this extractor uses only stdlib, no external dependencies.

### Pipeline Integration

No changes needed. The `.docx/.pptx/.xlsx` extensions are already in `documentExtensions` (used by text extraction). The `ExtractPageImages` handler already iterates all files and checks `rasterizers.Supports(ext)`. Registering the new extensions automatically includes them in both `default` and `rag` pipelines.

### URI Scheme

Reuses existing `mode=raster` pattern:
```
file://{repoID}/{blobName}/{path}?page={N}&mode=raster
```

Example: `file://5/main/docs/slides.pptx?page=2&mode=raster`

The MCP `handleRasterRead()` already handles this — looks up rasterizer by extension, calls `Render()`, JPEG-encodes at quality 80, returns base64 blob. No changes needed.

### Key Decisions

| Decision | Rationale |
|----------|-----------|
| Reuse `Rasterizer` interface | The existing interface (`PageCount` + `Render`) maps cleanly to "count images" + "get Nth image". No interface changes needed. |
| No XML relationship parsing | Media files are at predictable ZIP paths. Parsing `_rels/*.rels` would add complexity for no benefit — we want all images, not just referenced ones. |
| Skip EMF/WMF/SVG | Go stdlib cannot decode these formats. Adding CGO dependencies (e.g., for WMF→PNG) is out of scope. |
| Sort ZIP entries by path | Ensures deterministic image ordering so `page=N` is stable across calls. |
| Single struct, three registrations | All three formats use the same ZIP-walking logic with the same media directory prefixes. One implementation, registered for each extension. |
| No tabula usage | Tabula has no image extraction API. Its public methods are text-only. |
| Reuse `page_image` subtype | These are images from documents, same as PDF page images. Same enrichment type, same embedding flow, same MCP serving. |

### Codebase Patterns Found

- **Rasterizer registry** (`infrastructure/rasterization/rasterizer.go`): Generic interface + extension-based dispatch. The pattern is explicitly designed for extension ("extensible to spreadsheets, presentations, and other document types" per the package doc).
- **Handler reuse**: `ExtractPageImages` and `CreatePageImageEmbeddings` are extension-agnostic — they delegate to the registry. Adding formats is purely a registry operation.
- **JPEG quality 80**: Hardcoded in both `create_page_image_embeddings.go:178` and `server.go:1199`. The new extractor doesn't encode to JPEG — it returns raw `image.Image` and lets callers encode.
- **Pipeline specs** (`application/service/pipeline.go`): `ragSteps()` already includes `OperationExtractPageImagesForCommit` and `OperationCreatePageImageEmbeddingsForCommit`. Both pipelines already have these steps.
