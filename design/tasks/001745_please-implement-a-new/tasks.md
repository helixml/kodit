# Implementation Tasks

- [x] Create `infrastructure/rasterization/office.go` implementing the `Rasterizer` interface with `OfficeImageExtractor` struct
  - `PageCount(path)`: open ZIP, count entries matching `word/media/*`, `ppt/media/*`, `xl/media/*` with supported image extensions (.png, .jpg, .jpeg, .gif, .bmp, .tiff, .tif), sorted by path
  - `Render(path, page)`: open ZIP, find Nth matching entry (1-based), decode with `image.Decode()`, return `image.Image`
  - `Close()`: no-op (no persistent state)
  - Log warnings for skipped formats (EMF, WMF, SVG)
- [~] Register `OfficeImageExtractor` in `kodit.go` for `.docx`, `.pptx`, `.xlsx` extensions (after PDF rasterizer block, ~line 435)
- [ ] Create `infrastructure/rasterization/office_test.go` with tests:
  - Test `PageCount` returns correct count for a sample docx/pptx/xlsx with known images
  - Test `Render` returns a valid image for a known entry
  - Test that EMF/WMF/SVG entries are excluded from count
  - Test page out-of-range returns error
  - Test non-ZIP file returns error
  - Create minimal test fixtures (small ZIP files with embedded images) in a `testdata/` directory
- [ ] Verify end-to-end: run `make check` to confirm all existing tests pass with the new registration
