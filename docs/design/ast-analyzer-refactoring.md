# AST Analyzer Refactoring - Design Document

## Overview

This document describes the refactoring of the existing `Slicer` class to extract common AST parsing and analysis logic into a reusable `ASTAnalyzer` component. The Slicer currently uses tree-sitter for multi-language parsing, and this refactoring will make that infrastructure reusable for features like API documentation extraction and module hierarchy analysis while maintaining clean separation of concerns.

## Motivation

The current `Slicer` class (`src/kodit/infrastructure/slicing/slicer.py`) performs multiple responsibilities:
1. **Parsing**: Parse files with tree-sitter, build AST trees
2. **Definition extraction**: Walk AST using tree-sitter queries to find functions, classes, and types
3. **Call graph analysis**: Build function call relationships by traversing AST nodes
4. **Snippet generation**: Create code snippets with dependencies

**Current Implementation:**

The Slicer uses tree-sitter for language-agnostic parsing across Python, Go, Java, C/C++, Rust, JavaScript/TypeScript, C#, HTML, and CSS. It traverses ASTs using:

- `_walk_tree()`: Queue-based AST traversal that yields all nodes (src/kodit/infrastructure/slicing/slicer.py:321)
- `child_by_field_name()`: Named field access for extracting specific parts of definitions
- `LanguageConfig.CONFIGS`: Language-specific node type mappings (e.g., `function_nodes`, `method_nodes`, `call_node`)
- Pattern matching on node types to identify definitions, calls, and imports

**Problems:**

- Parsing and AST traversal logic is tightly coupled with snippet generation
- Cannot reuse parsing infrastructure for other use cases (e.g., API documentation, module hierarchy extraction)
- Adding new languages requires modifying the large Slicer class
- Testing parsing separately from snippet logic is difficult

**Solution:**
Extract parsing and definition extraction (items 1-2) into a shared `ASTAnalyzer` class. The Slicer will use ASTAnalyzer for parsing, then focus on its core responsibility: generating snippets with call graph dependencies.

## Goals

1. **Extract reusable parsing infrastructure** - Create `ASTAnalyzer` that can be used by multiple consumers
2. **Support multiple use cases** - Enable API documentation extraction, module hierarchy analysis, and future AST-based features
3. **Maintain backward compatibility** - Existing snippet extraction must continue to work without changes
4. **Improve testability** - Enable testing parsing logic independently from snippet generation
5. **Preserve language support** - All currently supported languages must continue to work
6. **Reuse existing code** - Prefer Extract Method and Extract Class refactorings from Slicer over writing new implementations

## Use Cases

The refactored `ASTAnalyzer` will enable multiple use cases beyond snippet generation:

### 1. API Documentation Extraction (Primary)

Extract public API signatures from library code to provide AI assistants with searchable interface documentation:

- **Input**: Library source files (e.g., Python package, Go module)
- **Output**: Module-level enrichments with public functions, classes, types, and their signatures
- **Usage**: `extract_module_definitions(parsed_files, include_private=False)`
- **Consumer**: New `APIDocExtractor` service

### 2. Module Hierarchy Extraction (Secondary)

Analyze project structure by extracting module organization and relationships:

- **Input**: All source files in a project
- **Output**: Tree structure showing packages/modules, their public exports, and hierarchical relationships
- **Use cases**:
  - Project structure visualization and navigation
  - Identifying public API surface area
  - Understanding module dependencies and import relationships
  - Generating architecture diagrams from code structure
  - Helping AI assistants understand codebase organization
- **Usage**: `extract_module_definitions()` to get module hierarchy with exports
- **Example output**: For a Python package `mylib/` with `processor.py`, `models.py`, and `utils/helpers.py`:
  ```
  mylib/
    ├── processor (public: process_data, DataProcessor)
    ├── models (public: User, Order, BaseModel)
    └── utils/
        └── helpers (public: format_output, validate_input)
  ```

### 3. Code Snippet Generation (Existing)

Current use case - generate code snippets with dependencies:

- **Input**: Source files to analyze
- **Output**: Snippets containing functions with their call dependencies
- **Usage**: `extract_definitions(parsed_files, include_private=True)`
- **Consumer**: Existing `Slicer` class

### 4. Future Use Cases (Enabled by Refactoring)

- **Code quality analysis**: Detect unused public APIs, identify large classes
- **Refactoring assistance**: Find all references to a definition across modules
- **Symbol navigation**: Build indexes for IDE-like "go to definition" features
- **Documentation completeness**: Check which public APIs lack docstrings

## Non-Goals

- Changing the snippet generation algorithm or output format
- Adding new languages (can be done after refactoring)
- Modifying the existing `LanguageConfig` structure
- Changes to public APIs or domain entities
- Implementing tree-sitter query language (continue using node traversal and type matching)

## Design

### Current Architecture

```
Slicer
├── Parse files with tree-sitter
├── Extract function/class definitions
├── Build call graphs
├── Generate snippets with dependencies
└── Return SnippetV2 entities
```

### Proposed Architecture

```
ASTAnalyzer (new)                    Slicer (refactored)
├── Parse files with tree-sitter     ├── Use ASTAnalyzer to parse
├── Extract definitions               ├── Build call graphs
├── Determine visibility              ├── Generate snippets with dependencies
└── Group by module                   └── Return SnippetV2 entities
```

### New Data Structures

```python
# src/kodit/infrastructure/slicing/ast_analyzer.py

from dataclasses import dataclass
from pathlib import Path
from tree_sitter import Node, Tree

from kodit.domain.entities.git import GitFile


@dataclass
class ParsedFile:
    """Result of parsing a single file with tree-sitter."""

    path: Path
    git_file: GitFile
    tree: Tree
    source_code: bytes


@dataclass
class FunctionDefinition:
    """Information about a function or method definition."""

    file: Path
    node: Node  # The tree-sitter AST node
    span: tuple[int, int]  # (start_byte, end_byte)
    qualified_name: str  # e.g., "module.ClassName.method_name"
    simple_name: str  # e.g., "method_name"
    is_public: bool  # Determined by language conventions
    is_method: bool  # True if inside a class
    docstring: str | None  # Extracted documentation
    parameters: list[str]  # Parameter names
    return_type: str | None  # Return type if available


@dataclass
class ClassDefinition:
    """Information about a class definition."""

    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str  # e.g., "module.ClassName"
    simple_name: str  # e.g., "ClassName"
    is_public: bool
    docstring: str | None
    methods: list[FunctionDefinition]
    base_classes: list[str]  # Names of parent classes


@dataclass
class TypeDefinition:
    """Information about a type definition (enum, interface, type alias)."""

    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str
    simple_name: str
    is_public: bool
    docstring: str | None
    kind: str  # "enum", "interface", "type_alias", "struct", etc.


@dataclass
class ModuleDefinition:
    """All definitions in a module, grouped by language conventions.

    A module is:
    - Python: one file
    - Go: one package (multiple files)
    - Java/C#: one namespace
    - TypeScript/JavaScript: one file
    - Rust: one module (file or mod.rs with submodules)
    - C/C++: one header file
    """

    module_path: str  # Import path: "mylib.processor", "github.com/org/repo/pkg"
    files: list[ParsedFile]
    functions: list[FunctionDefinition]
    classes: list[ClassDefinition]
    types: list[TypeDefinition]
    constants: list[tuple[str, Node]]  # (name, node) for public constants
    module_docstring: str | None  # Module-level documentation
```

### New ASTAnalyzer Class

```python
# src/kodit/infrastructure/slicing/ast_analyzer.py

import structlog
from tree_sitter import Node, Parser
from tree_sitter_language_pack import get_language

from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.slicer import LanguageConfig


class ASTAnalyzer:
    """Language-agnostic AST analyzer.

    Parses files with tree-sitter and extracts structured information about
    definitions (functions, classes, types). Used by both Slicer (for code
    snippets) and other consumers (e.g., API documentation extraction, module
    hierarchy analysis).

    Responsibilities:
    - Parse files into AST trees
    - Extract function, class, and type definitions
    - Determine visibility (public/private) based on language conventions
    - Group files by module
    - Extract documentation comments

    Non-responsibilities (left to consumers):
    - Call graph analysis
    - Dependency tracking
    - Snippet generation
    - Output formatting

    Implementation strategy:
    - Extract methods from existing Slicer code using Martin Fowler refactorings
    - Reuse _walk_tree() (slicer.py:321), _extract_function_name() (slicer.py:349),
      and _is_function_definition() (slicer.py:341)
    - Preserve existing LanguageConfig structure (slicer.py:46-143)
    - No new tree-sitter query language - continue using imperative node traversal
    """

    def __init__(self, language: str):
        """Initialize analyzer for a specific language.

        Args:
            language: Programming language (e.g., "python", "go", "typescript")

        Raises:
            ValueError: If language is not supported
        """
        self.language = language.lower()
        self.config = LanguageConfig.CONFIGS.get(self.language)
        if not self.config:
            raise ValueError(f"Unsupported language: {language}")

        ts_language = get_language(self._get_tree_sitter_name())
        self.parser = Parser(ts_language)
        self.log = structlog.get_logger(__name__)

    def parse_files(self, files: list[GitFile]) -> list[ParsedFile]:
        """Parse files into AST trees.

        Args:
            files: List of GitFile entities to parse

        Returns:
            List of ParsedFile objects containing trees and source code

        Note:
            Files that don't exist or can't be parsed are skipped with logging.
        """
        parsed = []
        for git_file in files:
            path = Path(git_file.path)
            if not path.exists():
                self.log.debug("Skipping non-existent file", path=str(path))
                continue

            try:
                with path.open("rb") as f:
                    source_code = f.read()

                tree = self.parser.parse(source_code)
                parsed.append(ParsedFile(
                    path=path,
                    git_file=git_file,
                    tree=tree,
                    source_code=source_code
                ))
            except OSError as e:
                self.log.warning("Failed to parse file", path=str(path), error=str(e))
                continue

        return parsed

    def extract_definitions(
        self,
        parsed_files: list[ParsedFile],
        include_private: bool = True
    ) -> tuple[list[FunctionDefinition], list[ClassDefinition], list[TypeDefinition]]:
        """Extract all definitions from parsed files.

        This is the main method for consumers that want flat lists of definitions
        (e.g., Slicer for building call graphs).

        Args:
            parsed_files: Files parsed by parse_files()
            include_private: Whether to include private/internal definitions

        Returns:
            Tuple of (functions, classes, types)
        """
        functions = []
        classes = []
        types = []

        for parsed in parsed_files:
            functions.extend(self._extract_functions(parsed, include_private))
            classes.extend(self._extract_classes(parsed, include_private))
            types.extend(self._extract_types(parsed, include_private))

        return functions, classes, types

    def extract_module_definitions(
        self,
        parsed_files: list[ParsedFile],
        include_private: bool = False
    ) -> list[ModuleDefinition]:
        """Extract definitions grouped by module.

        This is the main method for consumers that need module-level organization
        (e.g., API documentation extraction).

        Args:
            parsed_files: Files parsed by parse_files()
            include_private: Whether to include private/internal definitions

        Returns:
            List of ModuleDefinition objects, one per module
        """
        # Group files by module (language-specific)
        modules = self._group_by_module(parsed_files)

        result = []
        for module_path, module_files in modules.items():
            # Extract definitions from all files in the module
            functions = []
            classes = []
            types = []
            constants = []

            for parsed in module_files:
                functions.extend(self._extract_functions(parsed, include_private))
                classes.extend(self._extract_classes(parsed, include_private))
                types.extend(self._extract_types(parsed, include_private))
                constants.extend(self._extract_constants(parsed, include_private))

            module_doc = self._extract_module_docstring(module_files)

            result.append(ModuleDefinition(
                module_path=module_path,
                files=module_files,
                functions=functions,
                classes=classes,
                types=types,
                constants=constants,
                module_docstring=module_doc
            ))

        return result

    # Private methods for implementation

    def _group_by_module(
        self, parsed_files: list[ParsedFile]
    ) -> dict[str, list[ParsedFile]]:
        """Group files by module based on language conventions.

        Uses tree-sitter to extract module/package declarations:
        - Python: one file = one module, derive from file path
        - Go: parse package declarations (package statement), group by package name
        - Java/C#: parse package/namespace declarations, group by namespace
        - TypeScript/JavaScript: one file = one module, handle export statements
        - Rust: parse mod declarations, handle mod.rs hierarchy
        - C/C++: one header = one module

        For languages like Go and Java, this walks the AST to find package/namespace
        declaration nodes using tree-sitter node type checks.
        """
        # Implementation depends on language
        pass

    def _extract_functions(
        self, parsed: ParsedFile, include_private: bool
    ) -> list[FunctionDefinition]:
        """Extract function definitions from a parsed file.

        Walks the AST and finds nodes matching function_nodes or method_nodes
        from LanguageConfig. Uses tree-sitter node traversal and field access:
        - _walk_tree() to iterate over all nodes (extracted from slicer.py:321)
        - node.type checks against config["function_nodes"] (slicer.py:341)
        - child_by_field_name() for language-specific field extraction
        - Pattern matching on node types to identify parameters, return types

        Implementation approach:
        - Extract Method refactoring on Slicer._build_definition_and_import_indexes()
          (slicer.py:279-305)
        - Reuse Slicer._extract_function_name() (slicer.py:349) and language-specific
          extractors (_extract_go_method_name, _extract_c_cpp_function_name, etc.)
        - Reuse Slicer._qualify_name() (slicer.py:538) for building qualified names
        """
        pass

    def _extract_classes(
        self, parsed: ParsedFile, include_private: bool
    ) -> list[ClassDefinition]:
        """Extract class definitions with their methods.

        Finds class nodes, extracts class-level information, then recursively
        extracts methods within each class.
        """
        pass

    def _extract_types(
        self, parsed: ParsedFile, include_private: bool
    ) -> list[TypeDefinition]:
        """Extract type definitions (enums, interfaces, type aliases, structs).

        Language-specific handling:
        - Python: TypedDict, Enum, Protocol
        - Go: struct, interface, type alias
        - TypeScript: interface, type, enum
        - etc.
        """
        pass

    def _extract_constants(
        self, parsed: ParsedFile, include_private: bool
    ) -> list[tuple[str, Node]]:
        """Extract public constants.

        Language-specific handling:
        - Python: module-level assignments with UPPERCASE names
        - Go: const declarations
        - TypeScript: const exports
        - etc.
        """
        pass

    def _is_public(self, node: Node, name: str) -> bool:
        """Determine if a definition is public based on language conventions.

        Language-specific visibility rules:
        - Python: no leading underscore, or in __all__
        - Go: capitalized first letter
        - Java/C#: check for public/protected modifiers via child nodes
        - TypeScript/JavaScript: check for export keyword via parent/sibling nodes
        - Rust: check for pub modifier via child nodes
        - C/C++: assume public (slicer currently doesn't filter C/C++)

        Implementation: Walk node children and check for modifier keywords based
        on language. For Python and Go, use simple name pattern matching.
        """
        pass

    def _extract_docstring(self, node: Node) -> str | None:
        """Extract documentation comment for a definition.

        Uses tree-sitter node traversal to find documentation:
        - Walks children to find comment or string_literal nodes
        - Checks node positions relative to definition (before or after)
        - Filters by comment type (line vs block, doc vs regular)

        Language-specific formats:
        - Python: string_literal immediately after def/class (first statement)
        - Go: comment nodes immediately before declaration
        - Java: Javadoc comments (/** */) before declaration
        - TypeScript: JSDoc comments (/** */) before declaration
        - Rust: doc comment nodes (///) before declaration
        - C/C++: Doxygen-style comments before declaration
        """
        pass

    def _extract_module_docstring(self, module_files: list[ParsedFile]) -> str | None:
        """Extract module-level documentation.

        Usually from the first file in the module:
        - Python: module docstring (first string in file)
        - Go: package comment
        - etc.
        """
        pass

    def _get_tree_sitter_name(self) -> str:
        """Map language name to tree-sitter language name."""
        mapping = {
            "c++": "cpp",
            "c#": "csharp",
            "cs": "csharp",
            "js": "javascript",
            "ts": "typescript",
        }
        return mapping.get(self.language, self.language)

    def _walk_tree(self, node: Node) -> Generator[Node, None, None]:
        """Walk the AST tree, yielding all nodes.

        Uses queue-based traversal to avoid recursion issues.
        This method will be extracted from Slicer._walk_tree() using
        the Extract Method refactoring.
        """
        queue = [node]
        visited: set[int] = set()

        while queue:
            current = queue.pop(0)
            node_id = id(current)
            if node_id in visited:
                continue
            visited.add(node_id)

            yield current
            queue.extend(current.children)
```

### Refactored Slicer

```python
# src/kodit/infrastructure/slicing/slicer.py

class Slicer:
    """Slicer that extracts code snippets from files.

    Uses ASTAnalyzer for parsing and definition extraction,
    then builds call graphs and generates snippets with dependencies.
    """

    def __init__(self) -> None:
        """Initialize an empty slicer."""
        self.log = structlog.get_logger(__name__)

    def extract_snippets_from_git_files(
        self, files: list[GitFile], language: str = "python"
    ) -> list[SnippetV2]:
        """Extract code snippets from a list of files.

        Args:
            files: List of domain File objects to analyze
            language: Programming language for analysis

        Returns:
            List of extracted code snippets as domain entities

        Raises:
            ValueError: If no files provided or language unsupported
            FileNotFoundError: If any file doesn't exist
        """
        if not files:
            raise ValueError("No files provided")

        language = language.lower()

        # Step 1: Use ASTAnalyzer to parse files and extract definitions
        try:
            analyzer = ASTAnalyzer(language)
        except ValueError:
            self.log.debug("Skipping unsupported language", language=language)
            return []

        parsed_files = analyzer.parse_files(files)
        if not parsed_files:
            return []

        functions, classes, types = analyzer.extract_definitions(
            parsed_files, include_private=True
        )

        # Step 2: Build state for call graph analysis (snippet-specific)
        state = self._build_analyzer_state(
            parsed_files, functions, classes, analyzer.config
        )

        # Step 3: Build call graphs (snippet-specific)
        self._build_call_graph(state, analyzer.config)
        self._build_reverse_call_graph(state)

        # Step 4: Create mapping from Paths to File objects
        path_to_file_map: dict[Path, GitFile] = {
            parsed.path: parsed.git_file for parsed in parsed_files
        }

        # Step 5: Generate snippets with dependencies (snippet-specific)
        snippets: list[SnippetV2] = []
        file_contents: dict[Path, str] = {}

        for qualified_name in state.def_index:
            snippet_content = self._get_snippet(
                qualified_name,
                state,
                file_contents,
                {"max_depth": 2, "max_functions": 8},
            )
            if "not found" not in snippet_content:
                snippet = self._create_snippet_entity_from_git_files(
                    qualified_name, snippet_content, language, state, path_to_file_map
                )
                snippets.append(snippet)

        return snippets

    def _build_analyzer_state(
        self,
        parsed_files: list[ParsedFile],
        functions: list[FunctionDefinition],
        classes: list[ClassDefinition],
        config: dict[str, Any]
    ) -> AnalyzerState:
        """Build the state object needed for call graph analysis.

        Converts ASTAnalyzer results into the format expected by
        existing call graph code.
        """
        state = AnalyzerState(parser=None)  # Parser not needed anymore
        state.files = [p.path for p in parsed_files]
        state.asts = {p.path: p.tree for p in parsed_files}

        # Build def_index from extracted definitions
        for func in functions:
            state.def_index[func.qualified_name] = FunctionInfo(
                file=func.file,
                node=func.node,
                span=func.span,
                qualified_name=func.qualified_name
            )

        # Add methods from classes
        for cls in classes:
            for method in cls.methods:
                state.def_index[method.qualified_name] = FunctionInfo(
                    file=method.file,
                    node=method.node,
                    span=method.span,
                    qualified_name=method.qualified_name
                )

        # Build import map (keep existing logic)
        for parsed in parsed_files:
            file_imports = {}
            for node in self._walk_tree(parsed.tree.root_node):
                if self._is_import_statement(node, config):
                    imports = self._extract_imports(node)
                    file_imports.update(imports)
            state.imports[parsed.path] = file_imports

        return state

    # Rest of the methods remain the same:
    # - _build_call_graph
    # - _build_reverse_call_graph
    # - _get_snippet
    # - _create_snippet_entity_from_git_files
    # - etc.
```

## Implementation Plan

### Phase 1: Create Data Structures
- [ ] Create `ast_analyzer.py` file
- [ ] Define `ParsedFile` dataclass
- [ ] Define `FunctionDefinition` dataclass
- [ ] Define `ClassDefinition` dataclass
- [ ] Define `TypeDefinition` dataclass
- [ ] Define `ModuleDefinition` dataclass
- [ ] Add tests for data structures

### Phase 2: Implement ASTAnalyzer Core
- [ ] Implement `ASTAnalyzer.__init__` (language initialization)
- [ ] Implement `parse_files()` method (extract from Slicer.extract_snippets_from_git_files)
- [ ] Implement `_get_tree_sitter_name()` helper (extract from Slicer)
- [ ] Implement `_walk_tree()` helper (extract from Slicer using Extract Method refactoring)
- [ ] Add tests for parsing

### Phase 3: Implement Definition Extraction
- [ ] Implement `_extract_functions()` by extracting logic from Slicer._build_definition_and_import_indexes (slicer.py:279-305)
- [ ] Move Slicer._extract_function_name() and related methods to ASTAnalyzer using Move Method refactoring
- [ ] Implement `_extract_classes()` using tree-sitter node type checks for class definitions
- [ ] Implement `_extract_types()` for enum, interface, struct definitions
- [ ] Implement `_is_public()` with language-specific visibility rules
- [ ] Implement `_extract_docstring()` by walking AST for documentation nodes
- [ ] Implement `extract_definitions()` public method
- [ ] Add tests for all supported languages (Python, Go, Java, C/C++, Rust, JS/TS, C#)

### Phase 4: Implement Module Grouping
- [ ] Implement `_group_by_module()` with language-specific grouping logic:
  - Python: one file = one module (derive from file path)
  - Go: parse package declarations and group by package name
  - Java/C#: parse namespace declarations
  - TypeScript/JavaScript: one file = one module
  - Rust: parse mod declarations
  - C/C++: one header file = one module
- [ ] Implement `_extract_constants()` for module-level constants
- [ ] Implement `_extract_module_docstring()` for module documentation
- [ ] Implement `extract_module_definitions()` public method
- [ ] Add tests for module hierarchy extraction across languages

### Phase 5: Refactor Slicer
- [ ] Update Slicer to use ASTAnalyzer for parsing
- [ ] Implement `_build_analyzer_state()` conversion method
- [ ] Remove parsing code from Slicer (moved to ASTAnalyzer)
- [ ] Ensure all existing Slicer tests pass
- [ ] Run integration tests for snippet extraction

### Phase 6: Extend Language Support
- [ ] Verify ASTAnalyzer works for all existing languages (Python, Go, Java, C/C++, Rust, JS/TS, C#, HTML, CSS)
- [ ] Add language-specific definition extraction where needed (classes, types, constants)
- [ ] Implement language-specific module grouping (Go packages, Java namespaces, etc.)
- [ ] Add comprehensive tests for each language
- [ ] Ensure all existing Slicer language tests pass

### Phase 7: Documentation and Cleanup
- [ ] Add docstrings to all ASTAnalyzer methods
- [ ] Create usage examples in docstrings
- [ ] Update architecture documentation
- [ ] Remove any dead code from Slicer

## Testing Strategy

### Unit Tests

**ASTAnalyzer tests (`tests/kodit/infrastructure/slicing/ast_analyzer_test.py`):**
- Test parsing valid files
- Test parsing invalid files (graceful failure)
- Test function extraction with public/private filtering
- Test class extraction with methods
- Test type extraction
- Test docstring extraction
- Test module grouping per language
- Test visibility detection per language

**Slicer tests (existing):**
- All existing tests must continue to pass
- No changes to snippet output format
- No changes to call graph behavior

### Integration Tests

- End-to-end snippet extraction for all supported languages
- Verify snippet quality hasn't degraded
- Verify performance hasn't degraded significantly

## Migration Strategy

1. **Create ASTAnalyzer alongside existing Slicer** - No disruption to existing code
2. **Test ASTAnalyzer thoroughly in isolation** - Ensure correctness before integration
3. **Refactor Slicer to use ASTAnalyzer** - Single atomic change
4. **Run full test suite** - Verify backward compatibility
5. **Monitor in production** - Ensure no regressions in snippet quality

## Success Criteria

- [ ] All existing Slicer tests pass without modification
- [ ] No changes to snippet output format or quality
- [ ] ASTAnalyzer has >90% test coverage
- [ ] All supported languages work with ASTAnalyzer
- [ ] Performance within 10% of original implementation
- [ ] Code is easier to understand and maintain
- [ ] API documentation feature can now be built on top of ASTAnalyzer

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking existing snippet extraction | Comprehensive test coverage, gradual rollout |
| Performance regression | Benchmark before/after, optimize if needed |
| Language-specific edge cases | Thorough testing per language, incremental approach |
| Increased complexity | Clear separation of concerns, good documentation |

## Tree-Sitter Usage Patterns

The `ASTAnalyzer` uses tree-sitter's Python bindings to parse and traverse ASTs across all supported languages (Python, Go, Java, C/C++, Rust, JavaScript/TypeScript, C#, HTML, CSS). Key patterns used:

### Parsing
```python
parser = Parser(get_language("python"))
tree = parser.parse(source_bytes)
```

### AST Traversal
```python
# Queue-based traversal (existing pattern from Slicer)
for node in self._walk_tree(tree.root_node):
    if node.type == "function_definition":
        # Process function node
```

### Node Inspection
```python
# Check node type
if node.type in config["function_nodes"]:

# Access named fields (language-specific)
name_node = node.child_by_field_name("name")
params_node = node.child_by_field_name("parameters")

# Iterate children
for child in node.children:
    if child.type == "identifier":
        name = child.text.decode("utf-8")

# Get source location
start_byte, end_byte = node.start_byte, node.end_byte
```

### Pattern Matching
```python
# Language-agnostic pattern using LanguageConfig
if node.type in config["function_nodes"] + config["method_nodes"]:
    # Extract function regardless of language
```

**Note**: We do NOT use tree-sitter's query language (S-expressions). Instead, we use imperative node traversal and pattern matching, as this approach:
- Works with the existing LanguageConfig structure (slicer.py:46-143)
- Is easier to debug and test
- Provides more control over extraction logic
- Matches the current implementation patterns in Slicer
- Allows reuse of existing code through standard refactoring techniques

## Future Enhancements (Not in Scope)

After this refactoring is complete, ASTAnalyzer will enable:

- **API documentation extraction** (see [api-docs-enrichment.md](./api-docs-enrichment.md)) - Extract public API signatures from library code for AI assistant search
- **Module hierarchy extraction** - Visualize project structure, understand module organization, identify public API surface
- **Code quality analysis** - Detect unused public APIs, overly complex functions, missing documentation
- **Automated refactoring tools** - Find all references to a definition, rename symbols across files
- **Symbol navigation features** - Build indexes for IDE-like "go to definition" and "find references"
- **Dependency analysis** - Understand import relationships and module coupling
- **Other AST-based features** - Any feature requiring structured code analysis across languages

## References

- **Existing Slicer implementation**: `src/kodit/infrastructure/slicing/slicer.py`
  - Key methods to extract/reuse: `_walk_tree()` (line 321), `_extract_function_name()` (line 349), `_build_definition_and_import_indexes()` (line 279)
- **LanguageConfig**: `src/kodit/infrastructure/slicing/slicer.py:46-143` - Multi-language configuration for all 9 supported languages
- **Tree-sitter documentation**: <https://tree-sitter.github.io/tree-sitter/> - Parser library documentation
- **API documentation design**: `docs/design/api-docs-enrichment.md` - Primary use case for this refactoring
- **Martin Fowler's Refactoring Catalog**: Reference for Extract Method, Move Method, Extract Class refactorings
