# Indexing Queue Refactor Plan (Updated)

## Current State After Recent Refactoring

### Recent Improvements

The codebase has undergone significant refactoring that simplifies the queue implementation:

1. **Domain Services Extracted**: Business logic is now properly separated into domain services:
   - `IndexDomainService` - handles working copy operations and snippet extraction
   - `BM25DomainService` - handles keyword indexing
   - `EmbeddingDomainService` - handles code/text embeddings
   - `EnrichmentDomainService` - handles snippet enrichment

2. **Clean Application Service**: `CodeIndexingApplicationService` now focuses on orchestration

3. **Repository Separation**: Snippet operations extracted into `SnippetRepository`

4. **Incremental Indexing**: Already implemented - only processes changed files

### Current Problem

The `CodeIndexingApplicationService.run_index()` method still runs all 9 indexing phases sequentially:

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

### 1. Group Related Operations into 5 Logical Tasks

#### Task 1: SYNC

- Refresh working copy
- Check if there are changes to process
- Record the status of files

#### Task 2: EXTRACT

- Delete old snippets for changed files
- Extract new snippets from changed files
- Store new snippets in database
- Mark files as processed

#### Task 3: BM25_INDEX

- Create BM25 keyword index for new snippets

#### Task 4: CODE_EMBEDDINGS

- Create code embeddings for new snippets
- Store code embeddings in database

#### Task 5: ENRICH

- Enrich snippets with metadata
- Create text embeddings from enriched content
- Update index timestamp
- Clear file processing statuses

### 2. Task Ordering Using Priority

Since the queue orders by `priority DESC, created_at ASC`, use QueuePriority enum:

```python
# QueuePriority base values:
# USER_INITIATED = 50
# BACKGROUND = 10

# For user-initiated tasks (API/CLI):
SYNC:            USER_INITIATED + 40  # 90
EXTRACT:         USER_INITIATED + 30  # 80
BM25_INDEX:      USER_INITIATED + 20  # 70
CODE_EMBEDDINGS: USER_INITIATED + 10  # 60
ENRICH:          USER_INITIATED       # 50

# For background sync tasks:
SYNC:            BACKGROUND + 40      # 50
EXTRACT:         BACKGROUND + 30      # 40
BM25_INDEX:      BACKGROUND + 20      # 30
CODE_EMBEDDINGS: BACKGROUND + 10      # 20
ENRICH:          BACKGROUND           # 10
```

This ensures user-initiated tasks always take priority over background syncs.

### 3. Implementation Changes

#### Step 1: Extend TaskType Enum

```python
# In src/kodit/domain/value_objects.py
class TaskType(Enum):
    """Task type."""
    SYNC = 1
    EXTRACT = 2
    BM25_INDEX = 3
    CODE_EMBEDDINGS = 4
    ENRICH = 5
```

#### Step 2: Update SQLAlchemy TaskType Enum

```python
# In src/kodit/infrastructure/sqlalchemy/entities.py
class TaskType(Enum):
    """Task type."""
    SYNC = 1
    EXTRACT = 2
    BM25_INDEX = 3
    CODE_EMBEDDINGS = 4
    ENRICH = 5
```

#### Step 3: Replace run_index with queue_index_tasks

```python
# In src/kodit/application/services/code_indexing_application_service.py

async def queue_index_tasks(
    self, 
    index_id: int,
    is_user_initiated: bool = True
) -> None:
    """Queue the 5 indexing tasks with priority ordering.
    
    This replaces the old run_index() method entirely.
    
    Args:
        index_id: The ID of the index to process
        is_user_initiated: True for API/CLI calls, False for background syncs
    """
    from kodit.application.services.queue_service import QueueService
    from kodit.domain.entities import Task
    from kodit.domain.value_objects import QueuePriority
    
    queue = QueueService(self.session_factory)
    
    # Use different base priority for user vs background tasks
    base = QueuePriority.USER_INITIATED if is_user_initiated else QueuePriority.BACKGROUND
    
    # Queue tasks with descending priority to ensure execution order
    await queue.enqueue_task(
        Task.create(TaskType.SYNC, base + 40, {"index_id": index_id})
    )
    await queue.enqueue_task(
        Task.create(TaskType.EXTRACT, base + 30, {"index_id": index_id})
    )
    await queue.enqueue_task(
        Task.create(TaskType.BM25_INDEX, base + 20, {"index_id": index_id})
    )
    await queue.enqueue_task(
        Task.create(TaskType.CODE_EMBEDDINGS, base + 10, {"index_id": index_id})
    )
    await queue.enqueue_task(
        Task.create(TaskType.ENRICH, base, {"index_id": index_id})
    )

# DELETE the old run_index() method entirely
```

#### Step 4: Implement the 5 Task Handlers

```python
async def process_sync(self, index_id: int) -> None:
    """Handle SYNC task - refresh working copy."""
    index = await self.index_repository.get(index_id)
    if not index:
        raise ValueError(f"Index not found: {index_id}")
    
    async with self.operation.create_child(
        TaskOperation.REFRESH_WORKING_COPY,
        trackable_type=TrackableType.INDEX,
        trackable_id=index_id
    ) as step:
        index.source.working_copy = await self.index_domain_service.refresh_working_copy(
            index.source.working_copy, step
        )
        await self.index_repository.update(index)
        
        if len(index.source.working_copy.changed_files()) == 0:
            self.log.info("No new changes to index", index_id=index_id)
            await step.skip("No new changes to index")
            # Don't queue further tasks if no changes
            return

async def process_extract(self, index_id: int) -> None:
    """Handle EXTRACT task - extract snippets from changed files."""
    index = await self.index_repository.get(index_id)
    if not index:
        raise ValueError(f"Index not found: {index_id}")
    
    # Safety check: ensure we have changed files to process
    if len(index.source.working_copy.changed_files()) == 0:
        self.log.info("No files to extract", index_id=index_id)
        return
    
    async with self.operation.create_child(
        TaskOperation.EXTRACT_SNIPPETS,
        trackable_type=TrackableType.INDEX,
        trackable_id=index_id
    ) as operation:
        # Delete old snippets
        async with operation.create_child(TaskOperation.DELETE_OLD_SNIPPETS) as step:
            await self.snippet_repository.delete_by_file_ids(
                [f.id for f in index.source.working_copy.changed_files() if f.id]
            )
        
        # Extract new snippets
        extracted_snippets = await self.index_domain_service.extract_snippets_from_index(
            index=index, step=operation
        )
        
        # Persist files and snippets
        await self.index_repository.update(index)
        if extracted_snippets and index.id:
            await self.snippet_repository.add(extracted_snippets, index.id)

async def process_bm25_index(self, index_id: int) -> None:
    """Handle BM25_INDEX task - create keyword index."""
    async with self.operation.create_child(
        TaskOperation.CREATE_BM25_INDEX,
        trackable_type=TrackableType.INDEX,
        trackable_id=index_id
    ) as step:
        snippets = await self.snippet_repository.get_by_index_id(index_id)
        snippet_list = [sc.snippet for sc in snippets]
        
        if not snippet_list:
            self.log.info("No snippets to index", index_id=index_id)
            return
        
        await self._create_bm25_index(snippet_list)

async def process_code_embeddings(self, index_id: int) -> None:
    """Handle CODE_EMBEDDINGS task - create code embeddings."""
    async with self.operation.create_child(
        TaskOperation.CREATE_CODE_EMBEDDINGS,
        trackable_type=TrackableType.INDEX,
        trackable_id=index_id
    ) as step:
        snippets = await self.snippet_repository.get_by_index_id(index_id)
        snippet_list = [sc.snippet for sc in snippets]
        
        if not snippet_list:
            self.log.info("No snippets for embeddings", index_id=index_id)
            return
        
        await self._create_code_embeddings(snippet_list, step)

async def process_enrich(self, index_id: int) -> None:
    """Handle ENRICH task - enrich snippets and create text embeddings."""
    index = await self.index_repository.get(index_id)
    if not index:
        raise ValueError(f"Index not found: {index_id}")
    
    async with self.operation.create_child(
        TaskOperation.ENRICH_SNIPPETS,
        trackable_type=TrackableType.INDEX,
        trackable_id=index_id
    ) as operation:
        snippets = await self.snippet_repository.get_by_index_id(index_id)
        snippet_list = [sc.snippet for sc in snippets]
        
        if not snippet_list:
            self.log.info("No snippets to enrich", index_id=index_id)
            return
        
        # Enrich snippets
        enriched_snippets = await self.index_domain_service.enrich_snippets_in_index(
            snippets=snippet_list,
            reporting_step=operation,
        )
        await self.snippet_repository.update(enriched_snippets)
        
        # Create text embeddings
        async with operation.create_child(TaskOperation.CREATE_TEXT_EMBEDDINGS) as step:
            await self._create_text_embeddings(enriched_snippets, step)
        
        # Update timestamp
        async with operation.create_child(TaskOperation.UPDATE_INDEX_TIMESTAMP) as step:
            await self.index_repository.update_index_timestamp(index_id)
        
        # Clear file processing statuses
        async with operation.create_child(TaskOperation.CLEAR_FILE_PROCESSING_STATUSES) as step:
            index.source.working_copy.clear_file_processing_statuses()
            await self.index_repository.update(index)
```

#### Step 5: Update IndexingWorkerService

```python
# In src/kodit/application/services/indexing_worker_service.py

async def _process_task(self, task: Task, operation: ProgressTracker) -> None:
    """Process a task based on its type."""
    index_id = task.payload.get("index_id")
    service = create_code_indexing_application_service(
        app_context=self.app_context,
        session_factory=self.session_factory,
        operation=operation,
    )
    
    if task.type == TaskType.SYNC:
        await service.process_sync(index_id)
    elif task.type == TaskType.EXTRACT:
        await service.process_extract(index_id)
    elif task.type == TaskType.BM25_INDEX:
        await service.process_bm25_index(index_id)
    elif task.type == TaskType.CODE_EMBEDDINGS:
        await service.process_code_embeddings(index_id)
    elif task.type == TaskType.ENRICH:
        await service.process_enrich(index_id)
    else:
        self.log.warning(
            "Unknown task type",
            task_id=task.id,
            task_type=task.type,
        )
```

#### Step 6: Update Task Mapper

```python
# In src/kodit/infrastructure/mappers/task_mapper.py

TASK_TYPE_MAPPING: ClassVar[dict[db_entities.TaskType, TaskType]] = {
    db_entities.TaskType.SYNC: TaskType.SYNC,
    db_entities.TaskType.EXTRACT: TaskType.EXTRACT,
    db_entities.TaskType.BM25_INDEX: TaskType.BM25_INDEX,
    db_entities.TaskType.CODE_EMBEDDINGS: TaskType.CODE_EMBEDDINGS,
    db_entities.TaskType.ENRICH: TaskType.ENRICH,
}
```

#### Step 7: Update All Callers

Replace all calls to `run_index()` with `queue_index_tasks()`:

```python
# For user-initiated indexing (CLI/API):
# Before (remove this):
await service.run_index(index)

# After (use this):
await service.queue_index_tasks(index.id, is_user_initiated=True)

# For background sync tasks:
# Before (remove this):
await service.run_index(index)

# After (use this):
await service.queue_index_tasks(index.id, is_user_initiated=False)
```

Files that need updating:
- `src/kodit/cli.py` - Update the index command (use `is_user_initiated=True`)
- `src/kodit/application/services/sync_scheduler.py` - Update scheduled syncs (use `is_user_initiated=False`)
- Any API endpoints that trigger indexing (use `is_user_initiated=True`)

### 4. Migration Strategy

1. **Remove INDEX_UPDATE**: Delete the old INDEX_UPDATE task type completely
2. **Update all callers**: Replace all `run_index()` calls with `queue_index_tasks()`
3. **Remove run_index()**: Delete the run_index method from CodeIndexingApplicationService
4. **Database migration**: Create Alembic migration to update task type enum in database

## Benefits

1. **Granular Retry**: Individual phases can be retried without restarting
2. **Better Resource Management**: Heavy operations (embeddings) can be rate-limited
3. **Incremental Progress**: Users see progress through individual task completion
4. **Parallel Processing**: BM25_INDEX and CODE_EMBEDDINGS could potentially run in parallel
5. **Clean Separation**: Each task has a single responsibility

## Testing Strategy

1. **Unit Tests**: Test each process_* method independently
2. **Integration Test**: Queue all 5 tasks and verify execution order
3. **Failure Recovery Test**: Simulate failures at each stage and verify retry behavior
4. **Performance Test**: Ensure no regression in indexing speed

## Next Steps

1. ✅ Review current codebase structure (DONE)
2. Update TaskType enums in both domain and SQLAlchemy layers
3. Implement the 5 process_* methods in CodeIndexingApplicationService
4. Add queue_index_tasks method to replace run_index
5. Update IndexingWorkerService to handle new task types
6. Update all callers (CLI, sync scheduler, APIs)
7. Delete run_index() method completely
8. Create Alembic migration for database enum change
9. Write comprehensive tests for new task handlers
10. Update documentation
