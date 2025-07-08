# Database-Backed Caching for SimpleAnalyzer

## Overview
Cache the slicer's analysis results in the database to avoid re-parsing unchanged files.

## 1. Create FileAnalysis Entity

### Domain Entity
```python
# In domain/entities.py
@dataclass
class FunctionDefinition(BaseModel):
    """Cached function definition."""
    name: str
    qualified_name: str  
    start_byte: int
    end_byte: int
    
@dataclass  
class FileAnalysis(BaseModel):
    """Cached analysis results for a file."""
    id: int | None = None
    file_id: int
    file_sha256: str
    language: str
    
    # Extracted metadata (serializable)
    function_definitions: list[FunctionDefinition]
    function_calls: dict[str, list[str]]  # {caller: [callees]}
    imports: dict[str, str]  # {name: qualified_path}
    
    created_at: datetime | None = None
    updated_at: datetime | None = None
```

### Database Table
```sql
CREATE TABLE file_analyses (
    id SERIAL PRIMARY KEY,
    file_id INTEGER REFERENCES files(id),
    file_sha256 VARCHAR(64) NOT NULL,
    language VARCHAR(32) NOT NULL,
    function_definitions JSONB,
    function_calls JSONB,
    imports JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    
    INDEX idx_file_sha256 (file_sha256)
);
```

## 2. Create CachedSimpleAnalyzer Wrapper

```python
class CachedSimpleAnalyzer:
    """Wrapper that adds database caching to SimpleAnalyzer."""
    
    def __init__(self, root_path: str, language: str, 
                 file_analysis_repo: FileAnalysisRepository):
        self.analyzer = SimpleAnalyzer(root_path, language)
        self.repo = file_analysis_repo
        
    async def analyze_with_cache(self) -> dict[str, FileAnalysis]:
        """Analyze files, using cache where possible."""
        analyses = {}
        
        for file_path in self.analyzer.state.files:
            # Get file hash
            file_hash = self._compute_sha256(file_path)
            
            # Check cache
            cached = await self.repo.get_by_hash(file_hash, self.analyzer.language)
            if cached:
                analyses[file_path] = cached
            else:
                # Analyze and cache
                analysis = self._analyze_single_file(file_path)
                saved = await self.repo.save(analysis)
                analyses[file_path] = saved
                
        return analyses
```

## 3. Integrate with Existing Snippet Extraction

### Modify CrossFileSnippetExtractor
```python
class CrossFileSnippetExtractor(SnippetExtractor):
    """Extractor using cross-file analysis with caching."""
    
    def __init__(self, file_analysis_repo: FileAnalysisRepository):
        self.repo = file_analysis_repo
        self._analyzer_cache = {}
        
    async def extract(self, file_path: Path, language: str) -> list[str]:
        # Get or create cached analyzer for directory
        dir_path = str(file_path.parent)
        
        if (dir_path, language) not in self._analyzer_cache:
            # Create analyzer with caching
            analyzer = CachedSimpleAnalyzer(dir_path, language, self.repo)
            await analyzer.analyze_with_cache()
            self._analyzer_cache[(dir_path, language)] = analyzer
            
        analyzer = self._analyzer_cache[(dir_path, language)]
        
        # Extract snippets for this specific file
        return self._extract_file_snippets(analyzer, file_path)
```

## 4. Repository Implementation

```python
class FileAnalysisRepository:
    """Repository for file analysis caching."""
    
    async def get_by_hash(self, file_hash: str, language: str) -> FileAnalysis | None:
        """Get cached analysis by file hash and language."""
        
    async def get_for_index(self, index_id: int) -> list[FileAnalysis]:
        """Get all analyses for files in an index."""
        
    async def save(self, analysis: FileAnalysis) -> FileAnalysis:
        """Save or update analysis."""
        
    async def delete_for_file(self, file_id: int) -> None:
        """Delete analysis when file changes."""
```

## 5. Migration Strategy

### Phase 1: Add Caching
1. Create FileAnalysis entity and table
2. Implement repository
3. Add CachedSimpleAnalyzer wrapper
4. Register new extractor for DEPENDENCY_AWARE strategy

### Phase 2: Optimization
1. Add batch operations for efficiency
2. Implement cache warming for entire projects
3. Add metrics for cache hit rates

## Benefits

1. **Minimal Changes**: Works alongside existing system
2. **Performance**: Only re-analyze changed files
3. **Scalability**: Database handles persistence
4. **Simple**: No complex memory management
5. **Debuggable**: Can query cached data directly

## Example Usage

```python
# When processing files
for file in changed_files:
    # Existing analysis is cached by sha256
    # Only new/modified files are re-analyzed
    snippets = await extractor.extract(file.path, language)
```

This approach leverages your existing database infrastructure and File entity sha256 tracking for efficient caching of analysis results.