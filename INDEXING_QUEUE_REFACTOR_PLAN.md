# Simplified Indexing Queue Refactor Plan

## Current State

The `CodeIndexingApplicationService.run_index()` method runs all 9 indexing phases sequentially:

1. Refresh Working Copy
2. Delete Old Snippets  
3. Extract Snippets
4. Create BM25 Index
5. Create Code Embeddings
6. Enrich Snippets
7. Create Text Embeddings
8. Update Index Timestamp
9. Clear File Processing Statuses

**Problem**: If any phase fails, the entire process restarts from the beginning.

## Simplified Solution

### Group Related Operations into 3 Logical Tasks

Instead of 9 separate tasks, group operations that naturally belong together:

#### Task 1: SYNC

- Refresh working copy
- Check if there are changes to process (skip if no changes)
- Record the status of files.

#### Task 2: EXTRACT

- Delete old snippets for changed filesk
- Extract new snippets from changed files
- Store new snippets in database
- Mark files as processed

#### Task 3: BM25_INDEX

- Delete references and keywords related to deleted snippets
- Create BM25 keyword index for new snippets

#### Task 4: CODE_EMBEDDINGS

- Create code embeddings for new snippets
- Store code embeddings in database

#### Task 5: ENRICH

- Enrich snippets with metadata for new snippets
- Enrich snippets with summaries for new snippets
- Create text embeddings from enriched content for new snippets

### Why This Grouping Makes Sense

1. **SYNC**: Determines what files have changed and need processing
2. **EXTRACT**: Core snippet extraction and storage
3. **BM25_INDEX**: Keyword search index creation (fast, can be retried)
4. **CODE_EMBEDDINGS**: Vector embeddings for code search (slow, can be retried)
5. **ENRICH**: Enhancement with metadata and text embeddings (slowest, can be retried)

### Task Ordering

**Problem**: Tasks must run in the correct order (SYNC → EXTRACT → BM25_INDEX → CODE_EMBEDDINGS → ENRICH).

**Solution**: Use priority levels since the queue already orders by `priority DESC, created_at ASC`:

- SYNC: base_priority + 50 (runs first)
- EXTRACT: base_priority + 40
- BM25_INDEX: base_priority + 30  
- CODE_EMBEDDINGS: base_priority + 30
- ENRICH: base_priority + 10 (runs last)

**Safety**: Each task checks if prerequisites are available before proceeding (e.g.
checking whether snippets exist, or new files are available).

### Incremental Reindexing

**Problem**: It is vital that you only reindex snippets that have changed.

**Solution**: UNKNOWN

**Safety**: UNKNOWN

### Implementation Changes

#### 1. Extend TaskType Enum

```python
class TaskType(Enum):
    SYNC = 1
    EXTRACT = 2
    BM25_INDEX = 3
    CODE_EMBEDDINGS = 4
    ENRICH = 5
```

#### 2. Update CodeIndexingApplicationService

```python
class CodeIndexingApplicationService:
    async def queue_index_tasks(self, index_id: int, base_priority: int = 50) -> None:
        """Queue the 5 indexing tasks with priority ordering"""
        queue = QueueService(self.session_factory)
        
        # Queue tasks with descending priority to ensure execution order
        # Higher priority = runs first
        await queue.enqueue_task(
            Task.create(TaskType.SYNC, base_priority + 40, {"index_id": index_id})
        )
        await queue.enqueue_task(
            Task.create(TaskType.EXTRACT, base_priority + 30, {"index_id": index_id})
        )
        await queue.enqueue_task(
            Task.create(TaskType.BM25_INDEX, base_priority + 20, {"index_id": index_id})
        )
        await queue.enqueue_task(
            Task.create(TaskType.CODE_EMBEDDINGS, base_priority + 10, {"index_id": index_id})
        )
        await queue.enqueue_task(
            Task.create(TaskType.ENRICH, base_priority, {"index_id": index_id})
        )
    
    async def process_sync(self, index_id: int) -> None:
        """Handle SYNC task"""
        # Refresh working copy
        # Check for changes
        # Record file statuses
    
    async def process_extract(self, index_id: int) -> None:
        """Handle EXTRACT task"""
        # Safety: Check that SYNC has completed (working copy is fresh)
        index = await self.index_repository.get(index_id)
        if not index or not index.source.working_copy.has_changed_files():
            raise ValueError("...")
            
        # Delete old snippets for changed files
        # Extract new snippets from changed files
        # Store new snippets in database
        # Mark files as processed
    
    async def process_bm25_index(self, index_id: int) -> None:
        """Handle BM25_INDEX task"""
        # Safety: Check that EXTRACT has completed (new snippets exist)
        index = await self.index_repository.get(index_id)
        if not index or not index.has_unindexed_snippets():
            raise ValueError("...")
            
        # Delete references and keywords for deleted snippets
        # Create BM25 keyword index for new snippets
    
    async def process_code_embeddings(self, index_id: int) -> None:
        """Handle CODE_EMBEDDINGS task"""
        # Safety: Check that EXTRACT has completed
        index = await self.index_repository.get(index_id)
        if not index or not index.has_unindexed_snippets():
            raise ValueError("...")
            
        # Create code embeddings for new snippets
        # Store embeddings in database
    
    async def process_enrich(self, index_id: int) -> None:
        """Handle ENRICH task"""
        # Safety: Check that CODE_EMBEDDINGS has completed
        index = await self.index_repository.get(index_id)
        if not index or not index.has_unenriched_snippets():
            raise ValueError("...")
            
        # Enrich snippets with metadata for new snippets
        # Enrich snippets with summaries for new snippets  
        # Create text embeddings from enriched content
```

#### 3. Update IndexingWorkerService

```python
async def _process_task(self, task: Task) -> None:
    """Process a task based on its type"""
    index_id = task.payload.get("index_id")
    service = create_code_indexing_application_service(...)
    
    if task.type == TaskType.INDEX_UPDATE:
        # Legacy: run entire index
        await service.run_index(index)
    elif task.type == TaskType.SYNC:
        await service.process_sync(index_id)
    elif task.type == TaskType.EXTRACT:
        await service.process_extract(index_id)
    elif task.type == TaskType.BM25_INDEX:
        await service.process_bm25_index(index_id)
    elif task.type == TaskType.CODE_EMBEDDINGS:
        await service.process_code_embeddings(index_id)
    elif task.type == TaskType.ENRICH:
        await service.process_enrich(index_id)
```

#### 4. Replace all code in run_index with new queue methods

Replace the run_index method with the new queue methods, so that every caller now uses
the queue.

### Benefits

1. **Granular**: 5 focused tasks that can be retried individually
2. **No Data Passing**: Each task reads what it needs from the database
3. **Resilient**: Can retry expensive operations (embeddings, enrichment) without re-extracting
4. **Compatible**: Keep INDEX_UPDATE for backwards compatibility
5. **Clear Boundaries**: Each task has a single responsibility

### Next Steps

1. Add the 5 new TaskType enum values
2. Split `run_index()` into 5 methods, then remove the old code
3. Update worker to handle new task types
