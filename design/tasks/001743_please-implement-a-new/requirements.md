# Requirements: Office ZIP Image Extraction Pipeline Step

## Summary

Extract embedded images from Office Open XML files (docx, pptx, xlsx) during pipeline indexing. These formats are ZIP archives containing media files at known paths (e.g., `ppt/media/image1.png`). Extracted images are converted to JPEG quality 80, indexed for vision search, and served via MCP.

## User Stories

### US-1: Image extraction from Office documents
**As** a user indexing a repository containing Office documents,
**I want** embedded images (diagrams, charts, photos) automatically extracted and indexed,
**So that** I can find and reference visual content inside docx/pptx/xlsx files.

**Acceptance Criteria:**
- [ ] Pipeline walks ZIP contents of `.docx`, `.pptx`, `.xlsx` files
- [ ] Images found under `word/media/`, `ppt/media/`, `xl/media/` (and similar paths) are extracted
- [ ] Each extracted image is converted to JPEG at quality 80
- [ ] One enrichment record is created per extracted image
- [ ] Vision embeddings are generated for each extracted image
- [ ] Works with both `default` (all) and `rag` pipelines

### US-2: Image reference via MCP URI
**As** an MCP client,
**I want** to retrieve an extracted image using a file URI with `mode=image&ref=<zip-path>`,
**So that** I can display specific images from Office documents.

**Acceptance Criteria:**
- [ ] URI format: `file://{repoID}/{blob}/{path/to/file.pptx}?mode=image&ref=ppt/media/image1.png`
- [ ] Returns the image as a base64-encoded JPEG blob (quality 80)
- [ ] Returns appropriate error if ref path does not exist in the ZIP

### US-3: Source location tracking
**As** the system,
**I want** each extracted image's enrichment to store its ZIP-internal path as a `ref`,
**So that** the image can be resolved back to its origin inside the document.

**Acceptance Criteria:**
- [ ] `SourceLocation` model gains a `ref` string field for the internal ZIP path
- [ ] The ref is persisted via GORM AutoMigrate (no SQL migration files)
- [ ] `FileURI` can render `mode=image&ref=...` URIs
