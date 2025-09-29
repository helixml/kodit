# API Documentation Extraction Design

## Overview

This design document outlines a new feature for Kodit that will expose API documentation through the MCP server. The goal is to provide AI coding assistants with contextual, hierarchical API information that goes beyond the current snippet-based approach.

## Current Architecture Analysis

### Existing Components

1. **MCP Server (`src/kodit/mcp.py`)**
   - Exposes a `search` tool for finding code snippets
   - Uses HTTP streaming, STDIO, and SSE protocols
   - Integrates with AI coding assistants like Cursor and Claude

2. **Slicer (`src/kodit/infrastructure/slicing/slicer.py`)**
   - Extracts function-level snippets using Tree-sitter parsing
   - Supports 20+ programming languages
   - Builds call graphs and dependency relationships
   - Creates qualified names like `module.function_name`

3. **Snippet System (`src/kodit/domain/entities/git.py`)**
   - Stores individual code snippets with content and metadata
   - Includes dependency relationships and usage examples
   - Currently function-focused, not API-focused

### Limitations for API Documentation

1. **No Class Hierarchy Support**: Current slicer focuses on functions but doesn't explicitly track class structures
2. **No Public/Private Distinction**: All functions are treated equally regardless of visibility
3. **Limited Contextual Information**: Snippets don't include rich API metadata like parameters, return types, docstrings
4. **Static vs Dynamic**: Current snippets are pre-generated; API docs need dynamic context-aware generation

## Proposed Solution

### Core Concept: API-Aware Snippets

Extend the existing snippet system to support a new type of snippet specifically designed for API documentation. These "API Snippets" will be generated dynamically based on user queries and provide hierarchical, contextual information about public APIs.

### Architecture Overview

```
User Query: "How to start a new model"
    ↓
MCP Tool: api_documentation
    ↓
API Documentation Service
    ↓
Enhanced Slicer → API Snippet Generator
    ↓
Hierarchical API Response
```

## Detailed Design

### 1. Enhanced Slicer for API Detection

#### New Data Structures

```python
@dataclass
class APIInfo:
    """Information about an API element (class, method, function)."""
    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str
    api_type: APIType  # CLASS, METHOD, FUNCTION, INTERFACE
    visibility: Visibility  # PUBLIC, PRIVATE, PROTECTED
    parent_class: str | None  # For methods
    package: str
    docstring: str | None
    parameters: list[ParameterInfo]
    return_type: str | None
    decorators: list[str]  # For Python @property, etc.

@dataclass
class ParameterInfo:
    """Parameter information for API methods."""
    name: str
    type_hint: str | None
    default_value: str | None
    description: str | None  # From docstring

@dataclass
class APIHierarchy:
    """Hierarchical representation of API structure."""
    package: str
    classes: dict[str, ClassInfo]
    functions: dict[str, APIInfo]

@dataclass
class ClassInfo:
    """Class-level API information."""
    api_info: APIInfo
    methods: dict[str, APIInfo]
    properties: dict[str, APIInfo]
    inheritance: list[str]  # Parent classes
```

#### Enhanced AnalyzerState

```python
@dataclass
class EnhancedAnalyzerState(AnalyzerState):
    """Extended state for API analysis."""
    # Existing fields from AnalyzerState...
    
    # New API-specific fields
    api_index: dict[str, APIInfo] = field(default_factory=dict)
    class_hierarchy: dict[str, ClassInfo] = field(default_factory=dict)
    package_structure: dict[str, APIHierarchy] = field(default_factory=dict)
    public_apis: set[str] = field(default_factory=set)
```

#### Language-Specific API Detection

Extend `LanguageConfig` to include API-specific node types:

```python
CONFIGS = {
    "python": {
        # Existing config...
        "class_nodes": ["class_definition"],
        "visibility_indicators": {
            "private": lambda name: name.startswith("_") and not name.startswith("__"),
            "dunder": lambda name: name.startswith("__") and name.endswith("__"),
            "public": lambda name: not name.startswith("_")
        },
        "docstring_nodes": ["expression_statement"],
        "decorator_nodes": ["decorator"],
        "property_decorators": ["@property", "@staticmethod", "@classmethod"]
    },
    "java": {
        # Existing config...
        "class_nodes": ["class_declaration", "interface_declaration"],
        "visibility_modifiers": ["public", "private", "protected"],
        "annotation_nodes": ["annotation"]
    },
    # Additional languages...
}
```

### 2. API Documentation Service

Create a new application service specifically for API documentation:

```python
# src/kodit/application/services/api_documentation_service.py

class APIDocumentationService:
    """Service for generating API documentation snippets."""
    
    def __init__(
        self,
        snippet_repository: SnippetRepositoryV2,
        enhanced_slicer: EnhancedSlicer,
        embedding_service: EmbeddingDomainService,
    ):
        self.snippet_repository = snippet_repository
        self.enhanced_slicer = enhanced_slicer
        self.embedding_service = embedding_service
    
    async def search_api_documentation(
        self,
        query: APIDocumentationQuery
    ) -> APIDocumentationResponse:
        """Search for API documentation based on user intent."""
        
        # 1. Parse user query to understand intent
        intent = self._parse_user_intent(query.user_intent)
        
        # 2. Find relevant API elements
        api_candidates = await self._find_api_candidates(intent, query.filters)
        
        # 3. Score and rank APIs based on relevance
        ranked_apis = self._rank_api_relevance(api_candidates, intent)
        
        # 4. Generate hierarchical context for top results
        api_docs = []
        for api_info in ranked_apis[:query.max_results]:
            doc = self._generate_api_documentation(api_info, query.context_level)
            api_docs.append(doc)
        
        return APIDocumentationResponse(
            apis=api_docs,
            total_found=len(api_candidates),
            query_intent=intent
        )
    
    def _generate_api_documentation(
        self,
        api_info: APIInfo,
        context_level: ContextLevel
    ) -> APIDocumentation:
        """Generate comprehensive API documentation with context."""
        
        doc = APIDocumentation(
            qualified_name=api_info.qualified_name,
            api_type=api_info.api_type,
            visibility=api_info.visibility,
            signature=self._extract_signature(api_info),
            docstring=api_info.docstring,
            source_location=f"{api_info.file}:{api_info.span[0]}"
        )
        
        if context_level >= ContextLevel.MODERATE:
            # Add class context if this is a method
            if api_info.parent_class:
                doc.class_context = self._get_class_context(api_info.parent_class)
            
            # Add related methods in the same class
            doc.related_methods = self._find_related_methods(api_info)
        
        if context_level >= ContextLevel.COMPREHENSIVE:
            # Add usage examples
            doc.usage_examples = self._find_usage_examples(api_info)
            
            # Add full class hierarchy if applicable
            if api_info.api_type == APIType.METHOD:
                doc.inheritance_chain = self._get_inheritance_chain(api_info.parent_class)
        
        return doc
```

### 3. New MCP Tool for API Documentation

Add a new MCP tool alongside the existing `search` tool:

```python
# In src/kodit/mcp.py

@mcp_server.tool()
async def api_documentation(
    ctx: Context,
    user_intent: Annotated[
        str,
        Field(description="What the user wants to accomplish with the API")
    ],
    api_context: Annotated[
        str,
        Field(description="Specific API, class, or module context if known")
    ] = "",
    language: Annotated[
        str | None,
        Field(description="Programming language filter")
    ] = None,
    context_level: Annotated[
        str,
        Field(description="Amount of context: 'minimal', 'moderate', 'comprehensive'")
    ] = "moderate",
    max_results: Annotated[
        int,
        Field(description="Maximum number of API elements to return")
    ] = 5,
) -> str:
    """Get API documentation for specific functionality.
    
    This tool provides structured API documentation including class hierarchies,
    method signatures, parameters, return types, and usage examples. It's
    designed to help understand how to use specific APIs rather than finding
    general code examples.
    """
    
    mcp_context: MCPContext = ctx.request_context.lifespan_context
    api_service = mcp_context.server_factory.api_documentation_service()
    
    query = APIDocumentationQuery(
        user_intent=user_intent,
        api_context=api_context,
        filters=APIFilters(language=language),
        context_level=ContextLevel.from_string(context_level),
        max_results=max_results
    )
    
    response = await api_service.search_api_documentation(query)
    
    return APIDocumentationFormatter.format_response(response)
```

### 4. Data Models for API Documentation

```python
# src/kodit/domain/entities/api.py

@dataclass
class APIDocumentationQuery:
    """Query for API documentation search."""
    user_intent: str
    api_context: str = ""
    filters: APIFilters = field(default_factory=APIFilters)
    context_level: ContextLevel = ContextLevel.MODERATE
    max_results: int = 5

@dataclass
class APIFilters:
    """Filters for API documentation search."""
    language: str | None = None
    package: str | None = None
    class_name: str | None = None
    visibility: Visibility | None = None
    api_type: APIType | None = None

@dataclass
class APIDocumentation:
    """Complete API documentation for a single API element."""
    qualified_name: str
    api_type: APIType
    visibility: Visibility
    signature: str
    docstring: str | None
    source_location: str
    
    # Contextual information
    class_context: ClassContext | None = None
    related_methods: list[str] = field(default_factory=list)
    usage_examples: list[UsageExample] = field(default_factory=list)
    inheritance_chain: list[str] = field(default_factory=list)
    
    # Metadata
    parameters: list[ParameterInfo] = field(default_factory=list)
    return_type: str | None = None
    decorators: list[str] = field(default_factory=list)

@dataclass
class ClassContext:
    """Context about the class containing a method."""
    class_name: str
    package: str
    docstring: str | None
    inheritance: list[str]
    public_methods: list[str]
    properties: list[str]

@dataclass
class UsageExample:
    """Example of how to use an API."""
    code: str
    description: str
    source: str  # Where this example came from
```

### 5. Integration with Existing Snippet System

The new API documentation system should coexist with the existing snippet system:

#### Snippet Type Extension

```python
# Extend existing SnippetV2 to support API documentation
class SnippetType(Enum):
    FUNCTION = "function"  # Existing
    API_DOCUMENTATION = "api_documentation"  # New

@dataclass
class EnhancedSnippetV2(SnippetV2):
    """Extended snippet with API-specific metadata."""
    snippet_type: SnippetType = SnippetType.FUNCTION
    api_metadata: APIMetadata | None = None

@dataclass
class APIMetadata:
    """Metadata specific to API documentation snippets."""
    api_hierarchy: str  # e.g., "package.Class.method"
    visibility: Visibility
    api_type: APIType
    related_apis: list[str]
    context_level: ContextLevel
```

#### Repository Extensions

The existing `SnippetRepositoryV2` can be extended or a new repository can be created for API-specific queries:

```python
class APIDocumentationRepository:
    """Repository for API documentation queries."""
    
    async def find_public_apis_by_intent(
        self,
        intent: str,
        filters: APIFilters
    ) -> list[APIInfo]:
        """Find public APIs matching user intent."""
        
    async def get_class_hierarchy(
        self,
        class_name: str
    ) -> ClassHierarchy:
        """Get complete class hierarchy for a given class."""
        
    async def find_related_methods(
        self,
        method_qualified_name: str
    ) -> list[APIInfo]:
        """Find methods related to the given method."""
```

## Implementation Strategy

### Phase 1: Enhanced Slicer for API Detection

1. Extend `AnalyzerState` with API-specific data structures
2. Add class and visibility detection to the slicer
3. Implement public/private API identification for Python and Java
4. Add docstring and parameter extraction

### Phase 2: API Documentation Service

1. Create `APIDocumentationService` with basic query handling
2. Implement API ranking and relevance scoring
3. Add hierarchical context generation
4. Create formatters for different output styles

### Phase 3: MCP Integration

1. Add new `api_documentation` MCP tool
2. Integrate with existing server infrastructure
3. Add proper error handling and validation
4. Create comprehensive tool documentation

### Phase 4: Advanced Features

1. Add inheritance chain analysis
2. Implement usage example extraction from reverse call graphs
3. Add support for more programming languages
4. Optimize performance for large codebases

## Key Design Decisions

### 1. Extending vs. Replacing Snippets

**Decision**: Extend the existing snippet system rather than replacing it.

**Rationale**: 
- The existing snippet system works well for general code search
- API documentation has different requirements but overlapping data
- Coexistence allows users to choose the appropriate tool for their needs

### 2. Dynamic vs. Pre-generated API Documentation

**Decision**: Generate API documentation dynamically based on user queries.

**Rationale**:
- API documentation needs to be contextual to the user's specific intent
- Pre-generating all possible API contexts would be too expensive
- Dynamic generation allows for more relevant and focused results
- The hierarchy and related methods depend on what the user is trying to accomplish

### 3. Separate MCP Tool vs. Enhanced Search

**Decision**: Create a separate `api_documentation` MCP tool.

**Rationale**:
- Different use cases: general code search vs. specific API documentation
- Different parameters and output formats
- Allows for specialized prompt engineering for each use case
- Clearer tool purpose for AI assistants

### 4. Language Support Strategy

**Decision**: Start with Python and Java, then expand to other languages.

**Rationale**:
- Python and Java have clear visibility concepts and class hierarchies
- Tree-sitter parsers are well-established for these languages
- Covers a large portion of API documentation use cases
- Pattern can be extended to other object-oriented languages

## Example Usage Scenarios

### Scenario 1: Starting a New Model

**User Query**: "How do I start a new machine learning model?"

**API Documentation Response**:
```
Package: sklearn.linear_model
Class: LinearRegression

PUBLIC METHOD: __init__(self, fit_intercept=True, normalize=False, copy_X=True, n_jobs=None)
  Purpose: Initialize a linear regression model
  Parameters:
    - fit_intercept (bool, default=True): Whether to calculate intercept
    - normalize (bool, default=False): Whether to normalize features
  
  Related Methods in LinearRegression:
    - fit(X, y): Train the model on data
    - predict(X): Make predictions
    - score(X, y): Return coefficient of determination
  
  Usage Example:
    model = LinearRegression(fit_intercept=True)
    model.fit(X_train, y_train)
    predictions = model.predict(X_test)

Class Hierarchy: sklearn.base.BaseEstimator → LinearRegression
Package Context: sklearn.linear_model (Linear models for regression and classification)
```

### Scenario 2: Authentication Methods

**User Query**: "How to authenticate users in this framework?"

**API Documentation Response**:
```
Package: django.contrib.auth
Class: User

PUBLIC METHOD: authenticate(request=None, **credentials)
  Purpose: Verify user credentials and return User object if valid
  Parameters:
    - request (HttpRequest, optional): Current request object
    - **credentials: Username/password or other auth credentials
  Returns: User object if authentication succeeds, None otherwise
  
  Related Authentication Methods:
    - login(request, user): Log user into session
    - logout(request): Log user out of session
    - get_user(request): Get currently authenticated user
  
  Class Context: django.contrib.auth.models.User
    - Public properties: username, email, first_name, last_name, is_active
    - Public methods: check_password(), set_password(), get_full_name()
  
  Usage Example:
    from django.contrib.auth import authenticate, login
    user = authenticate(request, username='john', password='secret')
    if user is not None:
        login(request, user)
```

## Performance Considerations

1. **Indexing Strategy**: API metadata should be indexed separately from general snippets for efficient querying
2. **Caching**: Frequently requested API hierarchies should be cached
3. **Lazy Loading**: Related methods and usage examples should be loaded on-demand based on context level
4. **Language Parsing**: Tree-sitter parsing should be optimized for API extraction patterns

## Future Enhancements

1. **Cross-Language API Mapping**: Link equivalent APIs across different languages
2. **API Version Support**: Track API changes across different versions of libraries
3. **Community Examples**: Integrate with Stack Overflow or GitHub examples
4. **Interactive Documentation**: Generate interactive API explorers
5. **Deprecation Tracking**: Identify and warn about deprecated APIs

## Conclusion

This design provides a comprehensive approach to API documentation that leverages Kodit's existing infrastructure while adding the specialized capabilities needed for hierarchical, contextual API information. The phased implementation approach allows for iterative development and validation of the concept before full deployment.

The key innovation is the dynamic, context-aware generation of API documentation that understands not just what APIs exist, but how they relate to each other and how they should be used in the context of the user's specific intent.