# API Documentation MCP Tool Design

## Overview

This design document outlines a new MCP tool for providing contextual API documentation through the Kodit server. The goal is to leverage the existing slicer infrastructure to extract public API methods and provide them to AI assistants with proper context and hierarchy information, enabling more accurate API usage guidance.

## Problem Statement

Currently, the Kodit MCP server provides a `search()` tool that returns code snippets based on user intent and keywords. However, for API documentation purposes, we need:

1. **Structured API Discovery**: Extract public API methods with their class/package context
2. **Hierarchical Context**: Understand the relationship between classes, methods, and packages
3. **Related Method Resolution**: Find related methods based on API requests (e.g., "how to start a new model" should return initialization methods plus related configuration methods)
4. **Live/Dynamic Views**: Generate contextual API views based on specific requests rather than static documentation

## Current Architecture Analysis

### MCP Infrastructure
- **FastMCP Server**: Exposes tools via HTTP streaming, STDIO, or SSE
- **Current Tools**: `search()` and `get_version()`
- **Search Flow**: User intent → MultiSearchRequest → BM25/Embedding search → SnippetV2 results

### Slicer Capabilities
- **Tree-sitter Based**: Parses multiple languages (Python, Java, Go, Rust, C/C++, JS, etc.)
- **Function Extraction**: Currently extracts functions/methods and builds call graphs
- **Dependency Analysis**: Tracks function calls and can build dependency trees
- **Snippet Generation**: Creates code snippets with dependencies and imports

### Current Limitations for API Documentation
- **No Class Hierarchy Tracking**: Slicer focuses on functions, not class structures
- **No Visibility Analysis**: Doesn't distinguish between public/private methods
- **No Package/Module Context**: Limited understanding of namespace hierarchies
- **Function-Centric**: Not optimized for API surface area extraction

## Proposed Solution

### 1. Enhanced Slicer for API Extraction

#### New API-Focused Analyzer
Create a new `APIAnalyzer` class that extends the current slicer capabilities:

```python
@dataclass
class APIMethodInfo:
    """Information about an API method."""
    name: str
    qualified_name: str  # e.g., "com.example.UserService.createUser"
    visibility: Visibility  # PUBLIC, PRIVATE, PROTECTED, PACKAGE
    method_type: MethodType  # INSTANCE, STATIC, CONSTRUCTOR
    parameters: list[Parameter]
    return_type: str | None
    documentation: str | None
    file_path: Path
    line_number: int
    class_context: ClassInfo | None
    package_context: str | None

@dataclass 
class ClassInfo:
    """Information about a class."""
    name: str
    qualified_name: str
    visibility: Visibility
    superclass: str | None
    interfaces: list[str]
    methods: list[APIMethodInfo]
    file_path: Path
    package_context: str | None

@dataclass
class APIHierarchy:
    """Represents the API structure of a codebase."""
    packages: dict[str, PackageInfo]
    classes: dict[str, ClassInfo] 
    public_methods: dict[str, APIMethodInfo]
    method_relationships: dict[str, list[str]]  # method -> related methods
```

#### Language-Specific API Extraction
Extend `LanguageConfig` to support API-specific node types:

```python
"python": {
    "function_nodes": ["function_definition"],
    "method_nodes": ["function_definition"],  # Within class_definition
    "class_nodes": ["class_definition"],
    "module_nodes": ["module"],
    "visibility_indicators": ["def __", "def _"],  # Private method patterns
    # ... existing config
},
"java": {
    "function_nodes": ["method_declaration"],
    "class_nodes": ["class_declaration", "interface_declaration"],
    "package_nodes": ["package_declaration"],
    "visibility_modifiers": ["public", "private", "protected"],
    # ... existing config
}
```

### 2. New Domain Entities

#### APISnippet Entity
Extend or create alongside `SnippetV2` for API-specific information:

```python
class APISnippet(BaseModel):
    """API-focused snippet with hierarchical context."""
    
    # Core snippet info (similar to SnippetV2)
    sha: str
    content: str
    extension: str
    derives_from: list[GitFile]
    
    # API-specific metadata
    api_method: APIMethodInfo
    related_methods: list[APIMethodInfo]
    usage_examples: list[str] = []
    hierarchical_context: APIContext
    
@dataclass
class APIContext:
    """Hierarchical context for an API method."""
    package_path: str | None
    class_name: str | None
    method_name: str
    full_qualified_name: str
    import_statements: list[str]
    related_types: list[str]  # Types used in parameters/return
```

### 3. Enhanced Search with API Context

#### New MCP Tool: `api_documentation()`

```python
@mcp_server.tool()
async def api_documentation(
    ctx: Context,
    api_request: Annotated[str, Field(description="What the user wants to accomplish with the API")],
    specific_methods: Annotated[list[str], Field(description="Specific method names if known")] = [],
    class_filter: Annotated[str | None, Field(description="Filter to specific class or package")] = None,
    include_related: Annotated[bool, Field(description="Include related methods and examples")] = True,
    language: Annotated[str | None, Field(description="Programming language filter")] = None,
) -> str:
    """Get API documentation and usage examples for specific functionality.
    
    This tool provides structured API documentation with proper context,
    including class hierarchies, method signatures, and usage examples.
    """
```

#### API Discovery Service
New application service for API-specific searches:

```python
class APIDocumentationService:
    """Service for API documentation and discovery."""
    
    async def discover_api_methods(
        self, 
        intent: str, 
        filters: APISearchFilters
    ) -> list[APISnippet]:
        """Discover relevant API methods based on user intent."""
        
    async def get_method_context(
        self, 
        method_name: str
    ) -> APIContext:
        """Get full hierarchical context for a method."""
        
    async def find_related_methods(
        self, 
        method: APIMethodInfo
    ) -> list[APIMethodInfo]:
        """Find methods related to the given method."""
```

### 4. Live API View Generation

#### Dynamic Context Assembly
Instead of pre-generating all possible API documentation snippets, generate them dynamically:

1. **Query Analysis**: Parse user intent to identify:
   - Target functionality (e.g., "start a new model")
   - Preferred patterns (e.g., builder pattern, factory pattern)
   - Context requirements (e.g., initialization, configuration)

2. **Method Discovery**: Use enhanced search to find:
   - Primary methods matching the intent
   - Related initialization methods
   - Configuration methods
   - Error handling patterns

3. **Context Assembly**: Build hierarchical view:
   - Package/namespace imports needed
   - Class instantiation patterns
   - Method call sequences
   - Parameter documentation

4. **Related Method Resolution**: Identify related methods through:
   - Call graph analysis (what methods call/are called by target)
   - Parameter type analysis (methods using same types)
   - Pattern analysis (methods following similar naming conventions)
   - Documentation analysis (methods mentioned together in comments)

### 5. Integration with Existing Infrastructure

#### Extend Snippet Repository
Add API-specific repository methods:

```python
class SnippetRepositoryV2(ABC):
    # ... existing methods ...
    
    @abstractmethod
    async def search_api_methods(
        self, request: APISearchRequest
    ) -> list[APISnippet]:
        """Search for API methods with hierarchical context."""
        
    @abstractmethod  
    async def get_method_hierarchy(
        self, method_id: str
    ) -> APIHierarchy:
        """Get the full hierarchy context for a method."""
```

#### Enrichment for API Context
Extend the enrichment system to include API-specific metadata:

```python
class APIEnrichmentType(str, Enum):
    """API-specific enrichment types."""
    
    CLASS_HIERARCHY = "class_hierarchy"
    METHOD_SIGNATURE = "method_signature" 
    USAGE_PATTERN = "usage_pattern"
    RELATED_METHODS = "related_methods"
    PARAMETER_DOCUMENTATION = "parameter_docs"
```

### 6. Implementation Strategy

#### Phase 1: Enhanced Slicer
1. Extend `LanguageConfig` for class/visibility detection
2. Implement `APIAnalyzer` with class hierarchy extraction
3. Add visibility detection for common languages (Python, Java, Go)
4. Build API method index alongside function index

#### Phase 2: API-Specific Domain Layer
1. Create `APIMethodInfo` and related domain entities
2. Implement `APISnippet` extending current snippet model
3. Add API-specific search and filtering capabilities
4. Create `APIDocumentationService` application service

#### Phase 3: MCP Tool Integration
1. Implement `api_documentation()` MCP tool
2. Integrate with existing search infrastructure
3. Add API-specific result formatting
4. Implement dynamic context assembly

#### Phase 4: Advanced Features
1. Related method discovery algorithms
2. Usage pattern detection
3. API change detection across commits
4. Integration examples generation

## Questions and Considerations

### Compatibility with Existing Snippets
**Question**: Should API snippets use the same `SnippetV2` infrastructure or be separate?

**Recommendation**: Extend `SnippetV2` with optional API metadata rather than creating a separate system. This allows:
- Reuse of existing search infrastructure
- Unified storage and indexing
- Gradual migration path
- Compatibility with existing tools

```python
class SnippetV2(BaseModel):
    # ... existing fields ...
    api_metadata: APIMetadata | None = None  # Optional API-specific data

class APIMetadata(BaseModel):
    method_info: APIMethodInfo
    hierarchical_context: APIContext
    related_methods: list[str]  # References to other snippet IDs
```

### Live vs. Static Generation
**Question**: Should API documentation be pre-generated or dynamically assembled?

**Recommendation**: Hybrid approach:
- **Static Indexing**: Pre-extract and index API methods, classes, and basic relationships during commit processing
- **Dynamic Assembly**: Generate contextual views and related method collections at query time
- **Caching**: Cache frequently requested API contexts to improve performance

### Handling Multiple Languages
**Question**: How to handle different API patterns across languages?

**Recommendation**: Language-specific strategies within a common framework:
- **Python**: Class methods, module functions, `__init__` patterns
- **Java**: Class methods, interfaces, package structure, annotations
- **Go**: Package functions, struct methods, interface implementations
- **JavaScript**: Class methods, module exports, prototype methods

### Related Method Discovery
**Question**: How to effectively find related methods?

**Recommendation**: Multi-factor approach:
1. **Structural Relationships**: Same class, inheritance hierarchy, interface implementations
2. **Call Graph Analysis**: Methods that call each other or are commonly called together
3. **Type Relationships**: Methods using same parameter/return types
4. **Naming Patterns**: Methods with similar prefixes/suffixes (get/set, create/delete)
5. **Documentation Analysis**: Methods mentioned together in comments/docstrings
6. **Usage Patterns**: Methods frequently used together in examples

## Success Metrics

1. **API Coverage**: Percentage of public API methods successfully extracted and indexed
2. **Context Accuracy**: Quality of hierarchical context (package, class, method relationships)
3. **Related Method Relevance**: Accuracy of related method suggestions
4. **Response Time**: Performance of dynamic API context assembly
5. **User Satisfaction**: Effectiveness for AI assistant API guidance tasks

## Next Steps

1. **Prototype Phase**: Build a minimal API analyzer for Python to validate the approach
2. **Integration Testing**: Test with existing MCP infrastructure
3. **Evaluation**: Compare API documentation quality with current search results
4. **Iteration**: Refine based on real-world usage patterns
5. **Multi-language Support**: Extend to Java, Go, and other priority languages

This design leverages Kodit's existing strengths in code analysis and search while adding the structured API discovery capabilities needed for effective API documentation and guidance.