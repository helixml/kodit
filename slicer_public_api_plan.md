# Slicer Public API Enhancement Plan

## Goal
Extend the existing slicer to extract only public APIs with proper context (class, module, package) while maintaining backward compatibility.

## Current Slicer Analysis

**Existing Structure:**
- `FunctionInfo`: Contains file, node, span, qualified_name
- `LanguageConfig`: Language-specific node types and patterns
- `AnalyzerState`: Tracks functions, call graphs, imports
- `extract_snippets_from_git_files()`: Main extraction method

**Current Behavior:**
- Extracts ALL functions/methods from files
- Builds call dependency graphs
- Generates SnippetV2 for each function
- Supports 10+ programming languages

## Enhancement Plan

### 1. Extend FunctionInfo Structure

```python
@dataclass
class FunctionInfo:
    """Information about a function definition."""
    
    # Existing fields
    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str
    
    # New fields for public API support
    visibility: Visibility
    context: CodeContext
    is_public_api: bool

@dataclass
class CodeContext:
    """Context information for a function."""
    
    class_name: str | None = None
    module_name: str | None = None  
    package_name: str | None = None
    namespace: str | None = None

class Visibility(str, Enum):
    """Code visibility levels."""
    
    PUBLIC = "public"
    PRIVATE = "private" 
    PROTECTED = "protected"
    PACKAGE = "package"  # Java package-private
    INTERNAL = "internal"  # Some languages
    UNKNOWN = "unknown"
```

### 2. Extend LanguageConfig for Public API Detection

Add public API detection rules to each language configuration:

```python
CONFIGS: ClassVar[dict[str, dict[str, Any]]] = {
    "python": {
        # Existing config...
        "function_nodes": ["function_definition"],
        "class_nodes": ["class_definition"],
        "module_nodes": ["module"],
        
        # New public API rules
        "public_patterns": {
            "function": lambda name: not name.startswith("_"),
            "class": lambda name: not name.startswith("_"),
            "method": lambda name: not name.startswith("_")
        },
        "context_extractors": {
            "class": "class_definition",
            "module": "module"
        }
    },
    "java": {
        # Existing config...
        "function_nodes": ["method_declaration"],
        "class_nodes": ["class_declaration", "interface_declaration"],
        "package_nodes": ["package_declaration"],
        
        # New public API rules  
        "visibility_modifiers": ["public", "private", "protected"],
        "public_indicators": ["public"],
        "context_extractors": {
            "class": "class_declaration",
            "package": "package_declaration"
        }
    },
    "go": {
        # Existing config...
        "function_nodes": ["function_declaration"],
        "method_nodes": ["method_declaration"],
        "type_nodes": ["type_declaration"],
        
        # New public API rules
        "public_patterns": {
            "function": lambda name: name[0].isupper(),
            "type": lambda name: name[0].isupper(),
            "method": lambda name: name[0].isupper()
        },
        "context_extractors": {
            "package": "package_clause",
            "receiver": "parameter_list"  # For methods
        }
    },
    "javascript": {
        # Existing config...
        "function_nodes": ["function_declaration", "method_definition"],
        "class_nodes": ["class_declaration"],
        "export_nodes": ["export_statement"],
        
        # New public API rules
        "public_indicators": ["export"],
        "context_extractors": {
            "class": "class_declaration",
            "module": "program"
        }
    }
}
```

### 3. Add Context Extraction Methods

New methods in the Slicer class:

```python
class Slicer:
    # ... existing methods ...
    
    def _extract_code_context(
        self, 
        node: Node, 
        file_path: Path, 
        language: str
    ) -> CodeContext:
        """Extract class/module/package context for a node."""
        
    def _determine_visibility(
        self, 
        node: Node, 
        name: str, 
        language: str
    ) -> Visibility:
        """Determine if a function/method is public based on language rules."""
        
    def _is_public_api(
        self, 
        function_info: FunctionInfo, 
        language: str
    ) -> bool:
        """Determine if a function represents a public API."""
        
    def _find_parent_class(self, node: Node) -> Node | None:
        """Find the parent class node if the function is a method."""
        
    def _extract_package_info(self, tree: Tree, language: str) -> str | None:
        """Extract package/module information from the file."""
```

### 4. Update Main Extraction Logic

Modify `extract_snippets_from_git_files()` to filter for public APIs:

```python
def extract_snippets_from_git_files(
    self, 
    files: list[GitFile], 
    language: str = "python",
    public_apis_only: bool = False  # New parameter
) -> list[SnippetV2]:
    """Extract code snippets from files.
    
    Args:
        files: List of files to analyze
        language: Programming language
        public_apis_only: If True, only extract public API methods
    """
    
    # ... existing parsing logic ...
    
    # Build enhanced function index with context
    self._build_enhanced_definition_index(state, config, language)
    
    # Filter for public APIs if requested
    if public_apis_only:
        public_functions = {
            name: info for name, info in state.def_index.items()
            if info.is_public_api
        }
        functions_to_process = public_functions
    else:
        functions_to_process = state.def_index
    
    # Generate snippets only for selected functions
    snippets: list[SnippetV2] = []
    for qualified_name in functions_to_process:
        # ... existing snippet generation ...
```

### 5. Enhanced Function Index Building

Replace `_build_definition_and_import_indexes()` with enhanced version:

```python
def _build_enhanced_definition_index(
    self, 
    state: AnalyzerState, 
    config: dict[str, Any], 
    language: str
) -> None:
    """Build function definition index with context and visibility."""
    
    for file_path, tree in state.asts.items():
        # Extract package/module context for the entire file
        file_context = self._extract_file_context(tree, language)
        
        for node in self._walk_tree(tree.root_node):
            if self._is_function_definition(node, config):
                func_name = self._extract_function_name(node, config, language)
                if func_name:
                    # Extract context (class, module, etc.)
                    context = self._extract_code_context(node, file_path, language)
                    context.module_name = file_context.get("module")
                    context.package_name = file_context.get("package")
                    
                    # Determine visibility
                    visibility = self._determine_visibility(node, func_name, language)
                    
                    # Create enhanced function info
                    qualified_name = self._build_qualified_name(func_name, context)
                    function_info = FunctionInfo(
                        file=file_path,
                        node=node,
                        span=(node.start_byte, node.end_byte),
                        qualified_name=qualified_name,
                        visibility=visibility,
                        context=context,
                        is_public_api=self._is_public_api_by_rules(
                            func_name, visibility, context, language
                        )
                    )
                    
                    state.def_index[qualified_name] = function_info
```

### 6. Language-Specific Public API Rules

#### Python
- **Public Functions**: Don't start with `_`
- **Public Classes**: Don't start with `_`  
- **Public Methods**: Don't start with `_`, not `__init__` or `__special__`
- **Context**: Module name from file path, class name from parent class_definition

#### Java  
- **Public Methods**: Have `public` modifier
- **Public Classes**: Have `public` modifier
- **Context**: Package from package_declaration, class from class_declaration

#### Go
- **Public Functions**: Start with uppercase letter
- **Public Types**: Start with uppercase letter
- **Public Methods**: Start with uppercase letter
- **Context**: Package from package clause, receiver type for methods

#### JavaScript/TypeScript
- **Public Functions**: Exported functions
- **Public Classes**: Exported classes  
- **Public Methods**: Methods on exported classes
- **Context**: Module from file, class from class_declaration

### 7. New MCP Integration

Add public API filtering to the MCP search tool:

```python
@mcp_server.tool()
async def search(
    # ... existing parameters ...
    public_apis_only: Annotated[
        bool, 
        Field(description="Only return public API methods")
    ] = False,
) -> str:
    """Search for code examples, optionally filtered to public APIs only."""
```

### 8. Implementation Steps

1. **Step 1**: Extend `FunctionInfo` and add new dataclasses
2. **Step 2**: Add public API detection rules to `LanguageConfig` 
3. **Step 3**: Implement context extraction methods
4. **Step 4**: Implement visibility determination logic
5. **Step 5**: Update main extraction logic with public API filtering
6. **Step 6**: Add tests with existing test data
7. **Step 7**: Update MCP tool to support public API filtering

### 9. Backward Compatibility

- Keep `public_apis_only=False` as default to maintain existing behavior
- All new fields in `FunctionInfo` have defaults to avoid breaking changes
- Existing snippet generation continues to work unchanged

### 10. Testing Strategy

Use existing test data in `tests/kodit/infrastructure/slicing/data/`:
- **Python**: Test underscore prefix detection
- **Java**: Test public modifier detection  
- **Go**: Test capitalization rules
- **JavaScript**: Test export detection

Expected public APIs from test data:
```python
# Python (main.py)
- create_sample_products()  # Public function
- demonstrate_cart()       # Public function  
- demonstrate_points()     # Public function
- main()                   # Public function

# Java (Main.java) 
- public static void main()  # Public method
- public methods in classes  # Public methods

# Go (main.go)
- createSampleProducts()     # Public (capitalized)
- demonstrateCart()         # Public (capitalized)  
- main()                    # Public (capitalized)
```

This approach extends the existing slicer architecture incrementally while adding the public API extraction capability you need for the MCP tool.