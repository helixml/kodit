# Requirements Document: Example Enrichments

## Overview

Add a new enrichment type called "examples" to Kodit. Examples are code samples and demonstrations discovered from special directories and documentation files within indexed repositories. Like snippets, examples require enrichment with embeddings and AI-generated summaries to enable semantic search.

## Problem Statement

Users need to find usage examples and code samples within repositories. Kodit currently only indexes production code via AST-based snippet extraction. Example code often resides in:

- Dedicated example directories (`examples/`, `samples/`, `demos/`)
- Documentation files (`.md`, `.rst`, `.adoc`)
- Code blocks within documentation
- Tutorial/cookbook directories

These examples are valuable for understanding how to use a codebase but are not captured by the current snippet extraction process.

## Goals

1. Automatically identify and extract examples from repositories
2. Follow existing enrichment patterns (consistent with snippets)
3. Make examples searchable via semantic and keyword search
4. Support both standalone files and embedded code blocks
5. Associate examples with their source location and purpose

## Relevant Context from Codebase

### Existing Similar Feature: Snippets

**Location:** [src/kodit/domain/enrichments/development/snippet/](../../src/kodit/domain/enrichments/development/snippet/)

Snippets are the closest analog to examples:

- Type: `development`, Subtype: `snippet`
- Uses AST-based slicing to extract code snippets
- Processing: [src/kodit/application/handlers/commit/extract_snippets.py](../../src/kodit/application/handlers/commit/extract_snippets.py)
- Requires summary enrichment (`snippet_summary` subtype)
- Requires embeddings (code + summary)

**Key Difference:** Snippets are decomposed function-by-function using AST analysis. Examples should be extracted as complete, runnable units.

### Enrichment System Architecture

**Three-level taxonomy:**

- Level 1: Type (`development`, `usage`, `architecture`, `history`)
- Level 2: Subtype (e.g., `snippet`, `snippet_summary`, `api_docs`)
- Level 3: Concrete classes (e.g., `SnippetEnrichment`)

**Standard enrichment pipeline:**

1. Extract primary enrichment (create enrichment entity)
2. Create AI-generated summary (LLM)
3. Create code embeddings (vector DB)
4. Create summary embeddings (vector DB)

**Storage:**

- Table: `enrichments_v2` (no schema changes needed)
- Associations: `enrichment_associations` (links to commits, other enrichments)
- Embeddings: `embeddings` table (separate)

**Task System:**

- Operations defined in `TaskOperation` enum ([src/kodit/domain/value_objects.py](../../src/kodit/domain/value_objects.py))
- Handlers registered in application factory
- Pipeline: `SCAN_AND_INDEX_COMMIT` prescribed operation sequence

## Requirements

### 1. Example Discovery

#### 1.1 Directory-Based Discovery (Priority: P0)

Identify files within directories matching example-related patterns:

- Exact matches (case-insensitive): `examples/`, `example/`, `samples/`, `sample/`, `demos/`, `demo/`, `tutorials/`, `tutorial/`
- Support nested directories: `docs/examples/`, `src/samples/`, `examples/auth/basic/`
- Respect `.gitignore` patterns (already handled by git file listing)

**Acceptance Criteria:**

- All files in matching directories are candidates
- Support arbitrary nesting depth
- Case-insensitive matching

#### 1.2 Documentation-Based Discovery (Priority: P0)

Extract code blocks from documentation files:

- File extensions: `.md`, `.markdown`, `.rst`, `.adoc`, `.asciidoc`
- Code block formats:
  - Markdown fenced code blocks (` ```python `)
  - reStructuredText code blocks (`.. code-block:: python`)
- Extract from anywhere in repository

**Acceptance Criteria:**

- Each code block = separate example
- Preserve language identifier
- Capture surrounding context (heading, preceding paragraph)
- Handle multiple blocks per file

#### 1.3 File Type Filtering (Priority: P0)

Only process files with recognized extensions:

- Code files: Use existing `LanguageMapping` (`.py`, `.js`, `.go`, etc.)
- Documentation files: `.md`, `.rst`, `.adoc`
- Skip: Binary files, images, lock files

### 2. Example Extraction

#### 2.1 Full-File Extraction (Priority: P0)

For code files in example directories:

- Extract entire file content as single example
- Preserve all file structure and imports
- No AST-based decomposition

**Rationale:** Examples are meant to be complete, runnable units.

**Acceptance Criteria:**

- Read full file content
- Detect language from extension
- One enrichment per file
- Associate with source `GitFile`

#### 2.2 Code Block Extraction (Priority: P0)

For documentation files:

- Parse Markdown/RST/AsciiDoc
- Extract each fenced code block
- Capture metadata:
  - Language identifier
  - Surrounding heading (if within 10 lines)
  - Preceding paragraph (if within 3 lines)
  - Line number range

**Acceptance Criteria:**

- Support standard Markdown/RST syntax
- Preserve code indentation
- Handle blocks without language identifiers

#### 2.3 Content Deduplication (Priority: P0)

Examples may appear in multiple commits:

- Use content-based addressing (SHA-256 hash)
- Store unique content once
- Create multiple associations via `EnrichmentAssociation`

### 3. Enrichment Type Definition

#### 3.1 Create Example Enrichment (Priority: P0)

New enrichment with:

- Type: `development` (same as snippets)
- Subtype: `example` (new)
- Extends `DevelopmentEnrichment`
- Fields: `content`, `id`, timestamps (inherited)

#### 3.2 Create Example Summary Enrichment (Priority: P0)

New enrichment with:

- Type: `development`
- Subtype: `example_summary` (new)
- Extends `DevelopmentEnrichment`
- Generated via LLM enricher
- Associates with parent example + commit

**Suggested summary prompt:**

```
You are a professional software developer. You will be given an example code snippet.

Please provide a concise explanation (2-4 sentences) that covers:
1. What this example demonstrates
2. Key concepts or patterns shown
3. When you would use this approach
```

### 4. Pipeline Integration

#### 4.1 Add Task Operations (Priority: P0)

Add to `TaskOperation` enum:

- `EXTRACT_EXAMPLES_FOR_COMMIT`
- `CREATE_EXAMPLE_SUMMARY_FOR_COMMIT`
- `CREATE_EXAMPLE_CODE_EMBEDDINGS_FOR_COMMIT`
- `CREATE_EXAMPLE_SUMMARY_EMBEDDINGS_FOR_COMMIT`

#### 4.2 Update Prescribed Operations (Priority: P0)

Add to `SCAN_AND_INDEX_COMMIT` sequence:

- Extract examples (after extract snippets)
- Create example embeddings (with other embeddings)
- Create example summaries (with other summaries)
- Create example summary embeddings (with other summary embeddings)

### 5. Handlers

Create handlers following existing snippet patterns:

#### 5.1 Extract Examples Handler (Priority: P0)

Similar to `ExtractSnippetsHandler` ([src/kodit/application/handlers/commit/extract_snippets.py](../../src/kodit/application/handlers/commit/extract_snippets.py))

Steps:

1. Check if examples already exist for commit (skip if yes)
2. Load commit files
3. Filter to example candidates (directories + docs)
4. Extract full files from example directories
5. Extract code blocks from documentation files
6. Deduplicate by content hash
7. Save as enrichment entities
8. Create associations to commit

**Acceptance Criteria:**

- Idempotent (skip if exists)
- Report progress via `ProgressTracker`
- Handle errors gracefully
- Log statistics

#### 5.2 Create Example Summary Handler (Priority: P0)

Similar to `CreateSummaryEnrichmentHandler`

Steps:

1. Check if summaries already exist (skip if yes)
2. Get all examples for commit
3. Create enrichment requests with example-specific prompt
4. Stream responses from enricher
5. Save as summary enrichments
6. Associate with parent example + commit

#### 5.3 Create Embeddings Handlers (Priority: P0)

Two handlers, similar to existing embedding handlers:

1. Code embeddings (for example content, type: `CODE`)
2. Summary embeddings (for summary content, type: `TEXT`)

### 7. Documentation Parsers

#### 7.1 Markdown Parser (Priority: P0)

Extract fenced code blocks from Markdown:

- Support ` ```language ` and ` ~~~language ` syntax
- Extract language identifier
- Capture line numbers
- Find nearest heading (H1-H6)
- Find preceding paragraph

#### 7.2 RST Parser (Priority: P0)

Extract code blocks from reStructuredText:

- Support `.. code-block:: language` directive
- Support `.. code:: language` shorthand
- Handle indented code content
- Extract language from directive

#### 7.3 Parser Factory (Priority: P0)

Create parser based on file extension:

- `.md`, `.markdown` → Markdown parser
- `.rst` → RST parser
- Unsupported → return None
- Extensible for future formats

### 8. Example Metadata (Priority: P1)

Track example provenance:

- File path (relative within repo)
- Source type (`directory` or `documentation`)
- Detected language
- For code blocks:
  - Host file path
  - Line number range
  - Surrounding context (heading, description)

**Implementation approach:** Store metadata as structured comment/prefix in content field (no schema changes needed)

## Data Model

**No schema changes required.** Uses existing tables:

1. **enrichments_v2**: Store example content
   - `type`: "development"
   - `subtype`: "example" or "example_summary"

2. **enrichment_associations**: Link examples to commits and summaries to examples
   - Examples → commits
   - Summaries → examples
   - Summaries → commits

3. **embeddings**: Store vector embeddings
   - Link via `snippet_id` (enrichment ID)
   - Types: `CODE` (example), `TEXT` (summary)

## Appendix: Discovery Patterns

**Directory patterns (case-insensitive):**

```
examples/, example/, samples/, sample/, demos/, demo/
tutorials/, tutorial/, docs/examples/, src/samples/
```

**Documentation files:**

```
**/*.md, **/*.markdown, **/*.rst, **/*.adoc
```

**Exclusions:**

```
**/node_modules/**, **/vendor/**, **/.git/**
**/dist/**, **/build/**, **/__pycache__/**, **/venv/**
```

## Appendix: Test Scenarios

**Scenario 1:** File `examples/hello.py` → One example enrichment, Python, associated with commit

**Scenario 2:** Markdown file with code block → One example, language from fence, context from heading

**Scenario 3:** Documentation file with 3 blocks → Three separate examples

**Scenario 4:** Same example in 2 commits → One enrichment, two associations

**Scenario 5:** Nested `examples/auth/basic.py` → Recognized as example
