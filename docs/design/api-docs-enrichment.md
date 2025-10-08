# API Documentation Enrichment - Design Document

## Overview

This document describes the design for generating API documentation enrichments in Kodit. The goal is to provide AI coding assistants with concise, searchable API signatures for libraries and frameworks, enabling them to understand and use external dependencies correctly without needing access to traditional documentation.

**Prerequisites:** This design depends on the `ASTAnalyzer` refactoring described in [ast-analyzer-refactoring.md](./ast-analyzer-refactoring.md). The AST analyzer must be implemented first, as it provides the foundation for extracting API definitions.

## Problem Statement

When an AI coding assistant uses a library or framework, it needs to understand the **public API** - the interfaces, classes, functions, and methods that are intended for external use. Currently, Kodit extracts code snippets with implementation details, but what's needed is:

1. **Public API signatures** - Function/method signatures, class definitions, interfaces
2. **Queryable documentation** - Ability to search for specific API elements
3. **Concise format** - No implementation code, just the contract/interface
4. **Separation of concerns** - Public API for users vs private API for library developers (future)

## Key Design Questions & Answers

### 1. Structure: One Big Document vs. Snippets?

**Decision: Treat as enrichment snippets at the module level**

- Generate one enrichment per module (Python file, Go package, Java namespace, etc.)
- Each enrichment is a Markdown document containing all public API elements in that module
- Store as `EnrichmentV2` entities attached to commits
- Benefits:
  - Leverages existing search infrastructure (BM25, vector search, fusion)
  - Natural unit for documentation (matches how developers think about libraries)
  - Enables generating a browsable UI listing all public modules
  - Fits existing architecture patterns

### 2. Granularity: One Enrichment Per Module

**Decision: One enrichment per logical module containing all its public APIs**

**Module definition per language:**
- **Python**: One file (`.py`)
- **Go**: One package (directory with multiple `.go` files)
- **Java/C#**: One namespace or package
- **TypeScript/JavaScript**: One file (`.ts`, `.js`) or module
- **Rust**: One module (file or `mod.rs` with submodules)
- **C/C++**: One header file (`.h`, `.hpp`)

API elements to extract per module:
- **Class definitions** - Class signature with public methods
- **Function/method signatures** - Name, parameters, return type, documentation
- **Interface/Protocol definitions** - Contract definitions
- **Type definitions** - Enums, type aliases, structs, data classes
- **Module-level exports** - Explicitly exported elements
- **Constants** - Public constants and variables

**Format per module (Markdown document):**
```markdown
# Module: fully.qualified.module.path

Brief module description if available.

## Functions

### function_name
function_name(parameter1: Type1, parameter2: Type2, ...) -> ReturnType

Documentation summary explaining what the function does.
Key information: return value, exceptions/errors.

## Classes

### ClassName
class ClassName(BaseClass)

Documentation for the class.

#### Methods

##### method_name
method_name(self, param: Type) -> ReturnType

Method documentation.

## Types

### TypeName
type TypeName = definition

Type documentation.
```

### 3. Content: What to Include vs. Exclude?

**Include:**
- Function/method signatures with type annotations
- Documentation comments (summary only, not full documentation)
- Parameter names and types
- Return types
- Exception/error declarations
- Modifiers and annotations (static, abstract, readonly, etc.)
- Inheritance and interface information for classes
- Access modifiers (public, protected, etc.)

**Exclude:**
- Implementation code (method bodies, function bodies)
- Private/internal elements (determined by language conventions)
- Internal imports
- Implementation comments
- Code examples (these are in regular snippets)
- Full detailed documentation

### 4. How to Generate?

**Approach: Extend the existing Slicer with API extraction mode**

New component: `APIDocExtractor` in `infrastructure/slicing/`

```python
class APIDocExtractor:
    """Extract API documentation from code files."""

    def extract_api_docs(
        self,
        files: list[GitFile],
        language: str,
        include_private: bool = False
    ) -> list[APIDocEnrichment]:
        """Extract API documentation enrichments from files.

        Args:
            files: List of GitFile entities to process
            language: Programming language
            include_private: Whether to include private/internal APIs

        Returns:
            List of APIDocEnrichment entities (one per module)
        """
```

**Extraction strategy (language-agnostic approach):**

The extractor uses tree-sitter to parse files and identify public API elements. Each language has specific conventions for determining visibility:

- **Access modifiers**: Languages with explicit keywords (public, private, protected, internal)
- **Naming conventions**: Languages that use naming patterns (capitalization, prefix characters)
- **Export mechanisms**: Languages with explicit export declarations (export, public modules)
- **Package visibility**: Languages with package/module-level visibility controls

**What to extract per language:**
- Function/method declarations (signatures without bodies)
- Class/struct/interface definitions (public members only)
- Type definitions (enums, type aliases, interfaces)
- Constants and public variables
- Module/package exports
- Documentation comments in language-specific format

**Tree-sitter strategy:**
1. Parse file to get AST
2. Identify top-level and class-level declarations
3. Filter by visibility according to language conventions
4. Extract signature nodes (not implementation nodes)
5. Extract associated documentation comments
6. Format as API documentation enrichment

### 5. Storage: How to Store and Query?

**Storage Model:**

```python
@dataclass
class APIDocEnrichment(CommitEnrichment):
    """API documentation enrichment for a module."""

    module_path: str  # Import path: "mylib.processor", "github.com/org/repo/pkg"

    # Inherited from EnrichmentV2:
    # - entity_id: str (commit_sha)
    # - content: str (the Markdown API doc content for the entire module)
    # - id: int | None
    # - created_at: datetime | None
    # - updated_at: datetime | None

    @property
    def type(self) -> str:
        return "api_doc"

    @property
    def subtype(self) -> str | None:
        return "public"  # or "private" for future use

    def entity_type_key(self) -> str:
        return "git_commit"
```

**Database storage:**
- Use existing `enrichments_v2` table
- `type = "api_doc"`
- `subtype = "public"` or `"private"` (for future)
- Associated with commits via `enrichment_associations`

**Searchability:**
- API docs are indexed with BM25 (keyword search on function names, signatures)
- API docs get vector embeddings (semantic search on what the API does)
- Same fusion search as regular snippets
- Can filter by `enrichment_type = "api_doc"`

### 6. Workflow Integration

**New Task Operations:**

```python
class TaskOperation(StrEnum):
    # ... existing operations ...

    # New API doc operations
    CREATE_PUBLIC_API_DOCS_FOR_COMMIT = "kodit.commit.create_public_api_docs"
    CREATE_PRIVATE_API_DOCS_FOR_COMMIT = "kodit.commit.create_private_api_docs"
```

**Integration into commit indexing pipeline:**

Current pipeline (from `PrescribedOperations.INDEX_COMMIT`):
1. `EXTRACT_SNIPPETS_FOR_COMMIT`
2. `CREATE_BM25_INDEX_FOR_COMMIT`
3. `CREATE_CODE_EMBEDDINGS_FOR_COMMIT`
4. `CREATE_SUMMARY_ENRICHMENT_FOR_COMMIT`
5. `CREATE_SUMMARY_EMBEDDINGS_FOR_COMMIT`
6. `CREATE_ARCHITECTURE_ENRICHMENT_FOR_COMMIT`

**Add new step:**
7. `CREATE_PUBLIC_API_DOCS_FOR_COMMIT` (insert after step 6)
   - Group files by module (language-specific grouping)
   - Extract all public API elements per module
   - Generate Markdown document with import path metadata
   - Create one `APIDocEnrichment` per module
   - Store in enrichments table with module_path

**Future:**
8. `CREATE_PRIVATE_API_DOCS_FOR_COMMIT` (for developers working on the library itself)

## Architecture

### Domain Layer

**New entities:**

```python
# src/kodit/domain/enrichments/api_doc_enrichment.py
@dataclass
class APIDocEnrichment(CommitEnrichment):
    """API documentation enrichment for a module.

    Stores a Markdown document containing all public API elements
    for a single module (file, package, namespace, etc.).
    """

    module_path: str  # Import path for the module

    @property
    def type(self) -> str:
        return "api_doc"

    @property
    def subtype(self) -> str | None:
        return "public"  # or "private"
```

**Value objects:**

None needed - the module path and visibility (public/private) are captured in the `APIDocEnrichment` entity directly.

### Infrastructure Layer

**Foundation: ASTAnalyzer**

The API documentation extractor is built on top of the `ASTAnalyzer` component (see [ast-analyzer-refactoring.md](./ast-analyzer-refactoring.md) for full details).

ASTAnalyzer provides:

1. Parsing files with tree-sitter
2. Extracting function, class, and type definitions
3. Determining visibility (public/private) based on language conventions
4. Grouping files by module
5. Extracting documentation comments

**Key data structures from ASTAnalyzer:**

- `ParsedFile` - Parsed file with AST tree
- `FunctionDefinition` - Function info with visibility and docstring
- `ClassDefinition` - Class with methods, base classes, and docstring
- `TypeDefinition` - Type info (enum, interface, type alias)
- `ModuleDefinition` - Complete module with all definitions grouped

**Key methods from ASTAnalyzer:**

- `parse_files(files)` - Parse files into AST trees
- `extract_module_definitions(parsed_files, include_private)` - Extract all definitions grouped by module

See [ast-analyzer-refactoring.md](./ast-analyzer-refactoring.md) for complete implementation details.

**APIDocExtractor (built on ASTAnalyzer):**

```python
# src/kodit/infrastructure/slicing/api_doc_extractor.py
class APIDocExtractor:
    """Extract API documentation from code.

    Uses ASTAnalyzer to parse files and extract definitions,
    then formats them as Markdown API documentation.
    """

    def __init__(self):
        self.log = structlog.get_logger(__name__)

    def extract_api_docs(
        self,
        files: list[GitFile],
        language: str,
        include_private: bool = False
    ) -> list[APIDocEnrichment]:
        """Extract API documentation from files.

        Process:
        1. Use ASTAnalyzer to parse files and extract module definitions
        2. For each module, generate Markdown document
        3. Create APIDocEnrichment entities
        """
        if not files:
            return []

        try:
            # Use shared analyzer to extract definitions
            analyzer = ASTAnalyzer(language)
            parsed_files = analyzer.parse_files(files)
            modules = analyzer.extract_module_definitions(
                parsed_files, include_private
            )
        except ValueError:
            # Unsupported language
            self.log.debug("Unsupported language", language=language)
            return []

        # Generate API docs for each module
        enrichments = []
        for module in modules:
            markdown_content = self._generate_markdown(module)

            enrichment = APIDocEnrichment(
                entity_id=module.files[0].git_file.blob_sha,  # Use first file's SHA
                module_path=module.module_path,
                content=markdown_content,
            )
            enrichments.append(enrichment)

        return enrichments

    def _generate_markdown(self, module: ModuleDefinition) -> str:
        """Generate Markdown document for a module."""
        lines = [f"# Module: {module.module_path}", ""]

        if module.module_docstring:
            lines.append(module.module_docstring)
            lines.append("")

        # Functions section
        if module.functions:
            lines.append("## Functions")
            lines.append("")
            for func in module.functions:
                lines.extend(self._format_function(func))
                lines.append("")

        # Classes section
        if module.classes:
            lines.append("## Classes")
            lines.append("")
            for cls in module.classes:
                lines.extend(self._format_class(cls))
                lines.append("")

        # Types section
        if module.types:
            lines.append("## Types")
            lines.append("")
            for typ in module.types:
                lines.extend(self._format_type(typ))
                lines.append("")

        # Constants section
        if module.constants:
            lines.append("## Constants")
            lines.append("")
            for name, node in module.constants:
                lines.append(f"### {name}")
                lines.append("")

        return "\n".join(lines)

    def _format_function(self, func: FunctionDefinition) -> list[str]:
        """Format a function as Markdown."""
        lines = [f"### {func.qualified_name.split('.')[-1]}"]

        # Extract signature (without implementation)
        signature = self._extract_signature(func.node)
        lines.append(f"```{self.language}")
        lines.append(signature)
        lines.append("```")
        lines.append("")

        if func.docstring:
            lines.append(func.docstring)
            lines.append("")

        return lines

    def _format_class(self, cls: ClassDefinition) -> list[str]:
        """Format a class with methods as Markdown."""
        lines = [f"### {cls.qualified_name.split('.')[-1]}"]

        # Class signature
        signature = self._extract_signature(cls.node)
        lines.append(f"```{self.language}")
        lines.append(signature)
        lines.append("```")
        lines.append("")

        if cls.docstring:
            lines.append(cls.docstring)
            lines.append("")

        # Methods subsection
        if cls.methods:
            lines.append("#### Methods")
            lines.append("")
            for method in cls.methods:
                lines.extend(self._format_method(method))
                lines.append("")

        return lines

    def _extract_signature(self, node: Node) -> str:
        """Extract just the signature (no body) from a definition node."""
        # This is language-specific:
        # - Python: extract everything before the colon
        # - Go: extract the func declaration line
        # - TypeScript: extract the function/method signature
        # - etc.
        pass
```

### Application Layer

**New service:**

```python
# src/kodit/application/services/api_doc_application_service.py
class APIDocApplicationService:
    """Application service for API documentation extraction."""

    def __init__(
        self,
        api_doc_extractor: APIDocExtractor,
        enrichment_repository: EnrichmentV2Repository,
        snippet_repository: SnippetRepositoryV2,
    ):
        self.api_doc_extractor = api_doc_extractor
        self.enrichment_repository = enrichment_repository
        self.snippet_repository = snippet_repository

    async def create_api_docs_for_commit(
        self,
        commit_sha: str,
        include_private: bool = False
    ) -> list[APIDocEnrichment]:
        """Create API documentation for a commit.

        Process:
        1. Get all files for commit
        2. Group files by module (language-specific grouping):
           - Python: one file = one module
           - Go: one package = one module
           - Java/C#: one namespace = one module
        3. For each module:
           - Extract all public API elements
           - Determine import path
           - Generate Markdown document
           - Create APIDocEnrichment with module_path
        4. Save enrichments
        5. Return created enrichments
        """
```

### Search Integration

**Filtering API docs in search:**

Update `MultiSearchRequest`:

```python
@dataclass
class MultiSearchRequest:
    top_k: int = 10
    text_query: str | None = None
    code_query: str | None = None
    keywords: list[str] | None = None
    filters: SnippetSearchFilters | None = None

    # New field:
    enrichment_type: str | None = None  # "api_doc" to filter for API docs
```

**MCP tool update:**

Add parameter to search tool:

```python
@mcp_server.tool()
async def search(
    # ... existing parameters ...
    include_api_docs: Annotated[
        bool,
        Field(
            description="Include API documentation in results. "
            "Set to true when looking for library/framework usage examples."
        )
    ] = False,
) -> str:
    """Search for code examples and API documentation."""
```

## Implementation Plan

### Phase 0: Prerequisites
- [ ] Complete ASTAnalyzer refactoring (see [ast-analyzer-refactoring.md](./ast-analyzer-refactoring.md))
- [ ] Verify all existing Slicer tests pass with ASTAnalyzer
- [ ] Verify ASTAnalyzer works for all supported languages

### Phase 1: Foundation
- [ ] Create `APIDocEnrichment` entity with `module_path` field
- [ ] Add `CREATE_PUBLIC_API_DOCS_FOR_COMMIT` task operation
- [ ] Add tests for entity

### Phase 2: API Doc Extraction
- [ ] Implement `APIDocExtractor` using `ASTAnalyzer`
  - [ ] `extract_api_docs()` - Main entry point
  - [ ] `_generate_markdown()` - Generate Markdown from module definitions
  - [ ] `_format_function()` - Format function as Markdown
  - [ ] `_format_class()` - Format class with methods as Markdown
  - [ ] `_format_type()` - Format type definition as Markdown
  - [ ] `_extract_signature()` - Extract signature without implementation
- [ ] Start with 2-3 primary languages
- [ ] Add tests for API doc extraction

### Phase 3: Application Service
- [ ] Create `APIDocApplicationService`
- [ ] Integrate with commit indexing pipeline
- [ ] Add task status reporting
- [ ] Add tests for service layer

### Phase 4: Search Integration
- [ ] Update search to include API docs
- [ ] Add filtering by enrichment type
- [ ] Update MCP tool with API doc parameter
- [ ] Test end-to-end search flow

### Phase 5: Expand Language Coverage
- [ ] Add API extraction for remaining languages supported by Slicer
- [ ] Ensure consistent API doc format across all languages
- [ ] Add language-specific visibility detection
- [ ] Test API extraction across full language matrix

### Phase 6: Private API (Future)
- [ ] Add `CREATE_PRIVATE_API_DOCS_FOR_COMMIT` operation
- [ ] Implement private API extraction (include internal/private elements)
- [ ] Add access control to distinguish public vs private searches

## Example Output

### Example 1: Python Library

**Input file: `mylib/processor.py`**
```python
def _internal_validate(data: str) -> bool:
    """Internal validation logic."""
    return len(data) > 0

def process_data(
    input: str,
    options: dict[str, Any],
    *,
    timeout: int = 30
) -> ProcessResult:
    """Process input data with specified options.

    Returns ProcessResult containing processed output.
    Raises ProcessingError if input is invalid.
    """
    # ... implementation excluded ...

class DataProcessor:
    """Main processor class for handling data transformations."""

    def __init__(self, config: ProcessorConfig):
        """Initialize processor with configuration."""
        self._config = config

    def transform(self, data: Any) -> Any:
        """Transform data according to configuration."""
        # ... implementation excluded ...
```

**API Doc Enrichment (Markdown):**

```markdown
# Module: mylib.processor

## Functions

### process_data
def process_data(
    input: str,
    options: dict[str, Any],
    *,
    timeout: int = 30
) -> ProcessResult

Process input data with specified options.

Returns ProcessResult containing processed output.
Raises ProcessingError if input is invalid.

## Classes

### DataProcessor
class DataProcessor

Main processor class for handling data transformations.

#### Methods

##### __init__
def __init__(self, config: ProcessorConfig)

Initialize processor with configuration.

##### transform
def transform(self, data: Any) -> Any

Transform data according to configuration.
```

**Metadata:**
- `module_path`: `"mylib.processor"`
- `type`: `"api_doc"`
- `subtype`: `"public"`
- `entity_id`: commit SHA

**Note:** Internal validate function is excluded (private by naming convention)

---

### Example 2: Go Package

**Input files in `github.com/org/mylib/processor` package:**
- `processor.go`
- `types.go`

**processor.go:**
```go
package processor

// internalValidate checks data validity
func internalValidate(data string) bool {
    return len(data) > 0
}

// ProcessData processes input data with the given options.
// Returns ProcessResult or error if input is invalid.
func ProcessData(input string, options map[string]interface{}, timeout int) (*ProcessResult, error) {
    // ... implementation excluded ...
}
```

**types.go:**
```go
package processor

// DataProcessor handles data transformations
type DataProcessor struct {
    config ProcessorConfig
}

// NewDataProcessor creates a new DataProcessor
func NewDataProcessor(config ProcessorConfig) *DataProcessor {
    // ... implementation excluded ...
}

// Transform applies transformation to data
func (p *DataProcessor) Transform(data interface{}) (interface{}, error) {
    // ... implementation excluded ...
}
```

**API Doc Enrichment (Markdown):**

```markdown
# Module: github.com/org/mylib/processor

Package processor provides data processing utilities.

## Functions

### ProcessData
func ProcessData(input string, options map[string]interface{}, timeout int) (*ProcessResult, error)

ProcessData processes input data with the given options.
Returns ProcessResult or error if input is invalid.

## Types

### DataProcessor
type DataProcessor struct

DataProcessor handles data transformations.

#### Functions

##### NewDataProcessor
func NewDataProcessor(config ProcessorConfig) *DataProcessor

NewDataProcessor creates a new DataProcessor.

#### Methods

##### Transform
func (p *DataProcessor) Transform(data interface{}) (interface{}, error)

Transform applies transformation to data.
```

**Metadata:**
- `module_path`: `"github.com/org/mylib/processor"`
- `type`: `"api_doc"`
- `subtype`: `"public"`
- `entity_id`: commit SHA

**Note:** internalValidate is excluded (private by capitalization convention), multiple files combined into one module enrichment

---

### Example 3: TypeScript Module

**Input file: `mylib/processor.ts`**
```typescript
function internalValidate(data: string): boolean {
    return data.length > 0;
}

/**
 * Process input data with specified options
 * @throws ProcessingError if input is invalid
 */
export function processData(
    input: string,
    options: Record<string, any>,
    timeout: number = 30
): ProcessResult {
    // ... implementation excluded ...
}

/**
 * Main processor class for handling data transformations
 */
export class DataProcessor {
    private config: ProcessorConfig;

    constructor(config: ProcessorConfig) {
        // ... implementation excluded ...
    }

    /**
     * Transform data according to configuration
     */
    public transform(data: any): any {
        // ... implementation excluded ...
    }
}
```

**API Doc Enrichment (Markdown):**

```markdown
# Module: mylib/processor

## Functions

### processData
export function processData(
    input: string,
    options: Record<string, any>,
    timeout: number = 30
): ProcessResult

Process input data with specified options.

Throws ProcessingError if input is invalid.

## Classes

### DataProcessor
export class DataProcessor

Main processor class for handling data transformations.

#### Methods

##### constructor
constructor(config: ProcessorConfig)

##### transform
public transform(data: any): any

Transform data according to configuration.
```

**Metadata:**
- `module_path`: `"mylib/processor"`
- `type`: `"api_doc"`
- `subtype`: `"public"`
- `entity_id`: commit SHA

**Note:** internalValidate and private config are excluded (not exported / private)

### Search Usage

**User query:** "How do I use the data processing library?"

**Search request:**
- `text_query`: "process data"
- `keywords`: ["process", "data", "processor"]
- `include_api_docs`: `true`

**Results returned:**
1. API doc enrichment for `mylib.processor` module (Markdown document showing all public APIs)
2. Regular snippets showing usage examples (from calling code)

**UI Display:**
The UI can list all modules with their import paths:
- `mylib.processor` - Data processing utilities
- `mylib.utils` - Helper functions
- `mylib.config` - Configuration management

Clicking on a module shows the full Markdown API documentation.

## Benefits

1. **Focused API information** - AI assistants get clean API signatures without implementation noise
2. **Searchable** - Leverages existing hybrid search (BM25 + vector + fusion)
3. **Language agnostic** - Extensible to any language with tree-sitter support
4. **Module-level organization** - Natural unit matching how developers think about libraries
5. **Browsable UI** - Import paths enable listing all public modules in a repository
6. **Complete context** - Each module enrichment has full context of all related APIs
7. **Future-proof** - Can add private API docs for library developers later
8. **Minimal changes** - Reuses existing enrichment infrastructure
9. **Shared parsing infrastructure** - ASTAnalyzer benefits both snippets and API docs
10. **Clean separation** - Slicer focuses on call graphs and dependencies, APIDocExtractor focuses on signatures and formatting

## Open Questions

1. **Documentation format** - Should we normalize documentation comments across languages?
   - **Answer:** Keep original format, just extract the summary. Normalization can be added later.

2. **Type annotation extraction** - For languages with optional type systems, should we infer types from documentation?
   - **Answer:** Start with native type annotations only. Documentation inference is a future enhancement.

3. **Deprecation markers** - Should we include deprecation annotations/decorators?
   - **Answer:** Yes, include deprecation markers in the signature.

4. **Versioning** - Should API docs track version changes?
   - **Answer:** Not in MVP. They're already commit-scoped, which provides versioning.

5. **Visibility detection** - How to handle languages with complex visibility rules?
   - **Answer:** Use language conventions (modifiers, naming, exports). Default to conservative (only clearly public APIs).

6. **Module path extraction** - How to determine the import path for each module?
   - **Answer:** Language-specific logic:
     - Python: Use file path relative to project root, convert to dot notation
     - Go: Parse `package` declaration and directory structure
     - TypeScript/JavaScript: Use file path or `export` statements
     - Java/C#: Parse namespace/package declarations

## Success Metrics

1. **Coverage** - % of public API elements extracted per repository
2. **Search relevance** - Do API docs appear in top results for API queries?
3. **Assistant accuracy** - Does access to API docs reduce hallucinations?
4. **Performance** - Extraction time per commit

## References

- **AST Analyzer refactoring**: [ast-analyzer-refactoring.md](./ast-analyzer-refactoring.md) (prerequisite)
- Existing enrichment architecture: `src/kodit/domain/enrichments/`
- Slicer implementation: `src/kodit/infrastructure/slicing/slicer.py`
- Commit indexing pipeline: `src/kodit/domain/value_objects.py` (`PrescribedOperations`)
- MCP search tool: `src/kodit/mcp.py`
