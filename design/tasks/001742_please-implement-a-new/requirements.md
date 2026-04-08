# Requirements: Document Text Extraction Pipeline Step

## Context

Kodit currently extracts text from DOCX/PPTX/XLSX files via tabula (for chunking/BM25/code embeddings) and rasterizes PDF pages to images (for vision embeddings). However, there is no way to retrieve the **extracted text** of a specific document page/slide/sheet and serve it back to LLMs. The raster mode renders pages as images, but LLMs work best with a combination of text + images, not images alone.

This feature adds a `text` rasterization mode that extracts structured text from OOXML zip archives (docx/pptx/xlsx), exposes it via `file://path?mode=text&page=N`, and creates text-based enrichments alongside the existing page image enrichments.

## User Stories

### US-1: Text extraction from OOXML documents at index time
**As** the indexing pipeline,
**I want** to extract per-page/slide/sheet text from docx/pptx/xlsx files by walking their internal XML structure,
**So that** each page's text content is stored as an enrichment and available for search and retrieval.

**Acceptance Criteria:**
- A new pipeline step `extract_page_text` runs after `extract_snippets` in both the `default` and `rag` pipelines
- For PPTX: extracts text from each slide's XML (`ppt/slides/slideN.xml`) including titles, body placeholders, and speaker notes (`ppt/notesSlides/notesSlideN.xml`)
- For DOCX: extracts text from `word/document.xml`, split into logical pages (by page break markers or section breaks; single enrichment if no breaks exist)
- For XLSX: extracts text from each sheet (`xl/worksheets/sheetN.xml`), treating each sheet as a "page"
- Creates one `page_text` enrichment per page/slide/sheet with the extracted text as content
- Associates each enrichment with the commit, file, and repository
- Stores a `SourceLocation` with the page number
- Skips files that are not OOXML formats

### US-2: Text mode in file:// URIs
**As** an MCP client (or API consumer),
**I want** to request `file://repoID/blob/path/to/slides.pptx?mode=text&page=3`,
**So that** I receive the extracted text content of slide 3 instead of a rendered image.

**Acceptance Criteria:**
- The MCP `handleReadFile` accepts `mode=text` as a valid mode alongside `mode=raster`
- When `mode=text` is used with a `page` parameter, returns the text content of that specific page
- When `mode=text` is used without a `page` parameter, returns all pages' text concatenated with page separators
- Returns `text/plain` MIME type
- Returns an error if the file is not a supported OOXML format

### US-3: Text embeddings for document pages
**As** the search system,
**I want** page text enrichments to have code embeddings,
**So that** semantic search can find relevant document pages by their text content.

**Acceptance Criteria:**
- A new pipeline step `create_page_text_embeddings` runs after `extract_page_text`
- Creates embeddings for `page_text` enrichments using the code embedding provider
- Page text enrichments appear in semantic search results alongside code snippets
- Search results for page text enrichments include the document path and page number

### US-4: Slide-scoped media references (PPTX)
**As** the retrieval system,
**I want** to know which images belong to which slide in a PPTX,
**So that** when returning context for a matched slide, I can include only the relevant images.

**Acceptance Criteria:**
- For PPTX files, parse `ppt/slides/_rels/slideN.xml.rels` to build a slide-to-media map
- Store the media references in the page text enrichment's metadata or as a separate association
- When a page text enrichment is retrieved, the associated media file paths are available

## Non-Goals

- Rendering the visual layout of slides (positioning, charts, SmartArt diagrams)
- Full chart data extraction from `ppt/charts/chartN.xml` (cached PNGs are already captured by the raster pipeline)
- Supporting non-OOXML formats (PDF text extraction remains via tabula; ODT/EPUB stay as-is)
