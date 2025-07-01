# Migration to Index Aggregate Architecture

This document outlines the strategy for migrating from the current multiple-service architecture to the new Index aggregate root design.

## Current Architecture Issues

### Code Analysis of `CodeIndexingApplicationService`

The current application service has several problems:

1. **Service Proliferation**: 7+ domain services injected
2. **Manual Orchestration**: Application layer contains complex business logic
3. **Leaky Abstractions**: SQLAlchemy session management at application level
4. **Scattered State**: Snippet state tracked across multiple services

### Current Workflow Complexity

```python
# Current approach - complex orchestration
async def run_index(self, index_id: int) -> None:
    # 1. Get index from indexing service
    index = await self.indexing_domain_service.get_index(index_id)
    
    # 2. Delete old snippets via snippet service
    await self.snippet_domain_service.delete_snippets_for_index(index.id)
    
    # 3. Extract snippets via snippet service
    snippets = await self.snippet_domain_service.extract_and_create_snippets(...)
    
    # 4. Manual transaction management
    await self.session.commit()
    
    # 5. Create BM25 index via separate service
    await self._create_bm25_index(snippets, progress_callback)
    
    # 6. Create embeddings via separate service
    await self._create_code_embeddings(snippets, progress_callback)
    
    # 7. Enrich snippets via separate service
    await self._enrich_snippets(snippets, progress_callback)
    
    # 8. More embeddings via separate service
    await self._create_text_embeddings(snippets, progress_callback)
    
    # 9. Update timestamp via indexing service
    await self.indexing_domain_service.update_index_timestamp(index.id)
    
    # 10. Final commit
    await self.session.commit()
```

## New Architecture Benefits

### Simplified Application Service

```python
# New approach - aggregate root handles complexity
async def run_complete_indexing_workflow(
    self, uri: AnyUrl, local_path: Path
) -> domain_entities.Index:
    # 1. Create index (aggregate root)
    index = await self._index_domain_service.create_index(uri)
    
    # 2. Populate working copy (aggregate method)
    index = await self._index_domain_service.clone_and_populate_working_copy(
        index, local_path, SourceType.GIT
    )
    
    # 3. Extract snippets (aggregate method)
    index = await self._index_domain_service.extract_snippets(index)
    
    # 4. Simple transaction management
    await self._session.commit()
    
    return index
```

## Migration Strategy

### Phase 1: Parallel Implementation ✅

- [x] Create new domain entities (`domain/models/entities.py`)
- [x] Create repository protocol (`domain/models/protocols.py`)
- [x] Create mapping layer (`infrastructure/mappers/index_mapper.py`)
- [x] Create repository implementation (`infrastructure/sqlalchemy/index_repository.py`)
- [x] Create domain service (`domain/services/index_service.py`)
- [x] Create simplified application service (`application/services/simplified_indexing_service.py`)

### Phase 2: Feature Parity (Next Steps)

#### 2.1 Complete Index Domain Service
- [ ] Implement actual cloning logic in `clone_and_populate_working_copy`
- [ ] Complete snippet enrichment in `enrich_snippets_with_summaries`
- [ ] Add snippet search capabilities to Index aggregate
- [ ] Add BM25/embedding integration

#### 2.2 Application Service Integration
- [ ] Update application factories to create new services
- [ ] Add legacy compatibility methods
- [ ] Implement search functionality migration

#### 2.3 CLI Integration
- [ ] Update CLI commands to use new application service
- [ ] Maintain backward compatibility for existing commands

### Phase 3: Gradual Migration

#### 3.1 New Endpoints First
- [ ] Create new CLI commands using Index aggregate
- [ ] Add new MCP tools using simplified service
- [ ] Implement new features with aggregate root

#### 3.2 Legacy Adaptation
- [ ] Wrap old API calls to use new domain service
- [ ] Provide compatibility layer for existing integrations
- [ ] Migrate tests gradually

#### 3.3 Search Migration
- [ ] Move search logic into Index aggregate
- [ ] Create search value objects in domain
- [ ] Simplify search application service

### Phase 4: Complete Migration

#### 4.1 Remove Old Services
- [ ] Remove `IndexingDomainService` 
- [ ] Remove `SnippetDomainService`
- [ ] Remove `SourceService`
- [ ] Clean up old value objects

#### 4.2 Final Cleanup
- [ ] Remove legacy compatibility methods
- [ ] Update all tests to use new architecture
- [ ] Remove old application service

## Code Examples

### Before: Current Complexity

```python
class CodeIndexingApplicationService:
    def __init__(self, 
        indexing_domain_service: IndexingDomainService,
        snippet_domain_service: SnippetDomainService,
        source_service: SourceService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        enrichment_service: EnrichmentDomainService,
        session: AsyncSession,  # Leaky abstraction!
    ):
        # 7+ services to coordinate
```

### After: Aggregate Root Simplicity

```python
class SimplifiedIndexingApplicationService:
    def __init__(self,
        index_domain_service: IndexDomainService,
        session: AsyncSession,
    ):
        # Single domain service + session
        # All business logic in domain
```

## Benefits of Migration

### 1. **Reduced Complexity**
- Single domain service instead of 7+
- Business logic moves to domain layer
- Application layer focuses on coordination

### 2. **Better Domain Modeling**
- Index as true aggregate root
- Rich domain objects with behavior
- Proper encapsulation of business rules

### 3. **Improved Testability**
- Domain service can be tested in isolation
- No SQLAlchemy dependencies in domain tests
- Cleaner mocking for application tests

### 4. **Enhanced Maintainability**
- Clear boundaries between layers
- Easier to add new features
- Reduced coupling between services

### 5. **Better Performance**
- Fewer repository round trips
- Optimized aggregate loading
- Reduced object mapping overhead

## Risks and Mitigation

### Risk: Breaking Changes
**Mitigation**: Implement compatibility layer during transition

### Risk: Feature Regression
**Mitigation**: Comprehensive test coverage for both old and new

### Risk: Performance Impact
**Mitigation**: Benchmark and optimize aggregate loading

### Risk: Complex Migration
**Mitigation**: Gradual, phase-by-phase approach

## Success Metrics

- [ ] Reduced lines of code in application service (target: 50% reduction)
- [ ] Improved test coverage for domain logic
- [ ] Faster indexing workflow execution
- [ ] Fewer bugs related to state management
- [ ] Easier onboarding for new developers

## Next Immediate Steps

1. **Complete Domain Service**: Finish implementing cloning and enrichment
2. **Factory Integration**: Update application factories
3. **Simple CLI Command**: Create one new command using aggregate
4. **Performance Test**: Benchmark against current implementation
5. **Migration Plan**: Detail specific steps for first legacy endpoint