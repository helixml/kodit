# Requirements: Office Document Image Extraction Pipeline Step

## Summary

Extract embedded images from Office Open XML documents (docx, pptx, xlsx) during indexing, producing JPEG enrichments that feed into the existing vision embedding pipeline.

## User Stories

**As a user indexing repositories containing Office documents**, I want images embedded in docx/pptx/xlsx files to be extracted and indexed so that visual search can find content within these documents.

**As a developer consuming MCP resources**, I want to retrieve extracted images via `file://` URIs with `mode=image&ref=<xml-path>` so I can display them in context.

## Acceptance Criteria

1. A new `Rasterizer` implementation (`OfficeImageExtractor`) handles `.docx`, `.pptx`, and `.xlsx` extensions.
2. It walks the ZIP archive, finds images in the standard media directories (`word/media/`, `ppt/media/`, `xl/media/`), and exposes each as a "page" (1-based index).
3. All extracted images are re-encoded as JPEG at quality 80, regardless of source format (png, emf, wmf, etc.).
4. `PageCount()` returns the number of extractable images in the archive.
5. `Render(path, page)` returns the Nth image as an `image.Image`.
6. The step is registered in **both** the `default` and `rag` pipelines (same position as the existing page-image steps — it reuses them).
7. Enrichments are created with subtype `page_image` and source locations track the image index.
8. MCP resource URIs use `file://{repoID}/{blob}/{path}?page={n}&mode=raster` (same pattern as PDF pages).
9. The existing `ExtractPageImages` and `CreatePageImageEmbeddings` handlers work unchanged — the new rasterizer just adds `.docx/.pptx/.xlsx` to the registry.
10. Unsupported embedded formats (e.g., EMF/WMF vector graphics that cannot decode to `image.Image` via Go stdlib) are skipped with a warning log.

## Out of Scope

- Rendering document pages as screenshots (full-page rasterization like LibreOffice would do). This extracts only embedded image files.
- OCR of extracted images.
- Extracting images from older binary formats (.doc, .ppt, .xls).

## Notes

- **Tabula cannot be used for image extraction.** The `github.com/tsawler/tabula` library (already in go.mod) is text-extraction only. Its internal PDF image handling is private API used solely for OCR fallback. Office format readers in tabula have no image extraction methods.
- Office Open XML files are ZIP archives. Images live at predictable paths: `word/media/*`, `ppt/media/*`, `xl/media/*`. No XML relationship parsing is strictly needed to find media files — a prefix match on the zip entry path suffices.
- The user mentioned `ref=some/XML/key/to/image` in the URI. Since we're reusing the existing `mode=raster` + `page=N` URI scheme (which the MCP server already handles), the image index serves as the reference. Adding a separate `ref` query parameter would require changes to `FileURI` and the MCP handler that aren't justified — the page number uniquely identifies each image.
