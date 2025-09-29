# API Documentation Extraction Design

## Overview

This design document outlines a simplified approach to extract public API information from codebases using Tree-sitter queries. The goal is to generate API documentation snippets that describe the public methods, classes, functions, and parameters available in each file.

## Scope

This design focuses on two specific steps:

1. **Update the current slicer** to extract public API information using Tree-sitter queries
2. **Create a new infrastructure class** that uses the slicer to generate API snippets per file

Each snippet will be a string describing all public APIs in a single file.

## Current Slicer Analysis

The current slicer (`src/kodit/infrastructure/slicing/slicer.py`):
- Uses Tree-sitter parser to build ASTs
- Manually walks nodes based on configured node types in `LanguageConfig`
- Extracts function definitions and builds call graphs
- Generates snippets focused on individual functions with dependencies

## Proposed Changes

### Step 1: Update Slicer for Public API Extraction

#### Add Tree-sitter Query Support

Replace the current manual node walking with Tree-sitter queries for extracting public APIs:

```python
# New queries for extracting public APIs
PUBLIC_API_QUERIES = {
    "python": """
    ; Public functions (not starting with _)
    (function_definition
      name: (identifier) @function.name
      parameters: (parameters) @function.params
      (#not-match? @function.name "^_.*")) @function.def

    ; Public classes
    (class_definition
      name: (identifier) @class.name
      superclasses: (argument_list)? @class.inheritance
      body: (block) @class.body) @class.def

    ; Public methods in classes (not starting with _)
    (class_definition
      body: (block
        (function_definition
          name: (identifier) @method.name
          parameters: (parameters) @method.params
          (#not-match? @method.name "^_.*")) @method.def)) @class.with.methods

    ; Docstrings
    (expression_statement
      (string) @docstring)
    """,
    
    "java": """
    ; Public methods
    (method_declaration
      (modifiers 
        (modifier "public"))
      name: (identifier) @method.name
      parameters: (formal_parameters) @method.params) @method.def

    ; Public classes
    (class_declaration
      (modifiers 
        (modifier "public"))
      name: (identifier) @class.name) @class.def

    ; Interfaces
    (interface_declaration
      (modifiers 
        (modifier "public"))
      name: (identifier) @interface.name) @interface.def
    """,
    
    "javascript": """
    ; Exported functions
    (export_statement
      (function_declaration
        name: (identifier) @function.name
        parameters: (formal_parameters) @function.params)) @function.export

    ; Exported classes
    (export_statement
      (class_declaration
        name: (identifier) @class.name)) @class.export

    ; Function expressions assigned to exports
    (assignment_expression
      left: (member_expression
        object: (identifier "module")
        property: (property_identifier "exports"))
      right: (function_expression)) @function.module.export
    """
}
```

#### Enhanced Data Structures

```python
@dataclass
class PublicAPIInfo:
    """Information about a public API element."""
    name: str
    type: str  # 'function', 'class', 'method', 'interface'
    signature: str
    docstring: str | None
    file_path: Path
    line_number: int
    parent_class: str | None = None  # For methods
    parameters: list[str] = field(default_factory=list)
    return_type: str | None = None

@dataclass
class FileAPIInfo:
    """All public API information for a single file."""
    file_path: Path
    language: str
    classes: list[PublicAPIInfo] = field(default_factory=list)
    functions: list[PublicAPIInfo] = field(default_factory=list)
    methods: list[PublicAPIInfo] = field(default_factory=list)  # Grouped by class
    exports: list[PublicAPIInfo] = field(default_factory=list)  # For JS/TS
```

#### Modified Slicer Method

```python
def extract_public_apis_from_git_files(
    self, files: list[GitFile], language: str = "python"
) -> list[FileAPIInfo]:
    """Extract public API information from files using Tree-sitter queries."""
    
    if not files:
        raise ValueError("No files provided")
    
    language = language.lower()
    if language not in PUBLIC_API_QUERIES:
        self.log.debug("No public API queries for language", language=language)
        return []
    
    # Initialize Tree-sitter parser
    ts_language = get_language(self._get_tree_sitter_language_name(language))
    parser = Parser(ts_language)
    query = ts_language.query(PUBLIC_API_QUERIES[language])
    
    file_apis = []
    
    for file in files:
        file_path = Path(file.path)
        if not file_path.exists():
            continue
            
        # Parse file
        with file_path.open("rb") as f:
            source_code = f.read()
        tree = parser.parse(source_code)
        
        # Execute query
        captures = query.captures(tree.root_node)
        
        # Extract API information
        api_info = self._extract_api_info_from_captures(
            captures, source_code, file_path, language
        )
        file_apis.append(api_info)
    
    return file_apis

def _extract_api_info_from_captures(
    self, 
    captures: list, 
    source_code: bytes, 
    file_path: Path, 
    language: str
) -> FileAPIInfo:
    """Extract structured API info from Tree-sitter query captures."""
    
    api_info = FileAPIInfo(file_path=file_path, language=language)
    source_lines = source_code.decode('utf-8').split('\n')
    
    for node, capture_name in captures:
        line_number = node.start_point[0] + 1
        
        if capture_name.endswith('.name'):
            name = node.text.decode('utf-8')
            
        elif capture_name.endswith('.def'):
            # Extract full signature and other details
            signature = self._extract_signature(node, source_code)
            docstring = self._find_docstring(node, captures)
            
            api_element = PublicAPIInfo(
                name=name,
                type=self._determine_api_type(capture_name),
                signature=signature,
                docstring=docstring,
                file_path=file_path,
                line_number=line_number
            )
            
            # Add to appropriate list
            if api_element.type == 'class':
                api_info.classes.append(api_element)
            elif api_element.type == 'function':
                api_info.functions.append(api_element)
            # etc.
    
    return api_info
```

### Step 2: API Snippet Generator Infrastructure Class

Create a new infrastructure class that uses the enhanced slicer to generate API documentation snippets:

```python
# src/kodit/infrastructure/api_documentation/api_snippet_generator.py

class APISnippetGenerator:
    """Generates API documentation snippets from public API information."""
    
    def __init__(self, slicer: Slicer):
        self.slicer = slicer
        self.log = structlog.get_logger(__name__)
    
    def generate_api_snippets_from_git_files(
        self, files: list[GitFile], language: str = "python"
    ) -> list[SnippetV2]:
        """Generate API documentation snippets for each file."""
        
        # Extract public API info using enhanced slicer
        file_apis = self.slicer.extract_public_apis_from_git_files(files, language)
        
        snippets = []
        for file_api in file_apis:
            # Generate a single snippet per file containing all public APIs
            api_documentation = self._format_file_api_documentation(file_api)
            
            if api_documentation.strip():  # Only create snippet if there are public APIs
                snippet = SnippetV2(
                    derives_from=[self._create_git_file_from_path(file_api.file_path)],
                    content=api_documentation,
                    extension=f"{language}_api",  # Mark as API documentation
                    sha=SnippetV2.compute_sha(api_documentation),
                )
                snippets.append(snippet)
        
        return snippets
    
    def _format_file_api_documentation(self, file_api: FileAPIInfo) -> str:
        """Format public API information into a documentation string."""
        
        lines = []
        lines.append(f"# Public API Documentation: {file_api.file_path.name}")
        lines.append(f"# Language: {file_api.language}")
        lines.append("")
        
        # Document public classes
        if file_api.classes:
            lines.append("## Public Classes")
            for cls in file_api.classes:
                lines.append(f"### {cls.name}")
                if cls.docstring:
                    lines.append(f"    {cls.docstring}")
                lines.append(f"    Signature: {cls.signature}")
                lines.append("")
        
        # Document public functions
        if file_api.functions:
            lines.append("## Public Functions")
            for func in file_api.functions:
                lines.append(f"### {func.name}")
                if func.docstring:
                    lines.append(f"    {func.docstring}")
                lines.append(f"    Signature: {func.signature}")
                if func.parameters:
                    lines.append(f"    Parameters: {', '.join(func.parameters)}")
                if func.return_type:
                    lines.append(f"    Returns: {func.return_type}")
                lines.append("")
        
        # Document methods grouped by class
        if file_api.methods:
            lines.append("## Public Methods by Class")
            current_class = None
            for method in sorted(file_api.methods, key=lambda m: m.parent_class or ""):
                if method.parent_class != current_class:
                    current_class = method.parent_class
                    lines.append(f"### Class: {current_class}")
                
                lines.append(f"#### {method.name}")
                if method.docstring:
                    lines.append(f"    {method.docstring}")
                lines.append(f"    Signature: {method.signature}")
                if method.parameters:
                    lines.append(f"    Parameters: {', '.join(method.parameters)}")
                lines.append("")
        
        return "\n".join(lines)
```

### Integration Points

#### Modified Commit Indexing Service

Update the commit indexing service to also generate API snippets:

```python
# In CommitIndexingApplicationService.process_snippets_for_commit()

# After existing snippet extraction:
if self.generate_api_documentation:  # New config flag
    api_generator = APISnippetGenerator(slicer)
    api_snippets = api_generator.generate_api_snippets_from_git_files(
        lang_files, language=lang
    )
    all_snippets.extend(api_snippets)
```

#### New MCP Tool (Future)

The generated API snippets can be searched using the existing search infrastructure, or a new specialized tool can be added later:

```python
@mcp_server.tool()
async def search_api_documentation(
    ctx: Context,
    query: str,
    language: str | None = None
) -> str:
    """Search for API documentation in the indexed codebase."""
    
    # Use existing search but filter for API snippets
    search_request = MultiSearchRequest(
        keywords=[query],
        text_query=query,
        filters=SnippetSearchFilters(
            language=language,
            extension=f"{language}_api" if language else None
        )
    )
    
    # Use existing search service
    service = ctx.request_context.lifespan_context.server_factory.code_search_application_service()
    results = await service.search(search_request)
    
    return MultiSearchResult.to_jsonlines(results)
```

## Example Output

For a Python file containing:

```python
class UserManager:
    """Manages user authentication and profiles."""
    
    def authenticate(self, username: str, password: str) -> User | None:
        """Authenticate user with credentials."""
        # implementation
    
    def _hash_password(self, password: str) -> str:
        """Private method for password hashing."""
        # implementation

def create_user(name: str, email: str) -> User:
    """Create a new user account."""
    # implementation

def _internal_helper():
    """Private helper function."""
    # implementation
```

The generated API snippet would be:

```
# Public API Documentation: user_manager.py
# Language: python

## Public Classes
### UserManager
    Manages user authentication and profiles.
    Signature: class UserManager:

## Public Functions
### create_user
    Create a new user account.
    Signature: def create_user(name: str, email: str) -> User:
    Parameters: name, email
    Returns: User

## Public Methods by Class
### Class: UserManager
#### authenticate
    Authenticate user with credentials.
    Signature: def authenticate(self, username: str, password: str) -> User | None:
    Parameters: self, username, password
```

## Implementation Steps

1. **Add Tree-sitter query support** to the slicer with public API queries for Python, Java, JavaScript
2. **Create `APISnippetGenerator`** infrastructure class
3. **Update commit indexing** to optionally generate API snippets
4. **Test with sample files** to validate query accuracy
5. **Add configuration** to enable/disable API snippet generation
6. **Extend to more languages** as needed

This simplified approach focuses on the core goal: extracting public API information and generating descriptive snippets per file, without the complexity of dynamic querying or hierarchical context generation.