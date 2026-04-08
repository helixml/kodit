# Extract images from Office documents (docx/pptx/xlsx)

## Summary
Add image extraction from Office Open XML documents by implementing a new `OfficeImageExtractor` that plugs into the existing rasterization registry. This enables vision search over images embedded in Word, PowerPoint, and Excel files.

## Changes
- New `infrastructure/rasterization/office.go` — walks ZIP archives to find and decode embedded images from `word/media/`, `ppt/media/`, `xl/media/` directories
- Register `.docx`, `.pptx`, `.xlsx` extensions in the rasterizer registry (`kodit.go`)
- 10 unit tests covering page count, rendering, deterministic ordering, out-of-range errors, unsupported formats (EMF/WMF/SVG), and invalid input

## Testing
- All 15 rasterization tests pass (`make test PKG=./infrastructure/rasterization/...`)
- `go vet` clean
- No new pipeline operations or handlers — reuses existing `ExtractPageImages` and `CreatePageImageEmbeddings` handlers unchanged
