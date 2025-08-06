# Queue-Based Indexing Refactoring Plan

## Executive Summary

This plan outlines the refactoring of the Kodit indexing system to add queue-based processing for the `serve` command. When running as a server, all indexing operations (from API, auto-indexing, and sync scheduler) will go through a deduplicating priority queue processed by background workers. CLI commands remain unchanged and continue to execute directly.

## Current Architecture Analysis

### Current Implementation
- **CLI Commands**: Direct synchronous execution of indexing operations
- **FastAPI Server**: Currently serves MCP endpoints and API routes
- **Application Service**: `CodeIndexingApplicationService` handles all indexing operations directly
- **Background Services**: `AutoIndexingService` and `SyncSchedulerService` run in the background during `serve`

### Key Components
1. **CodeIndexingApplicationService**: Central service for indexing operations
2. **Domain Services**: BM25, embedding, enrichment services
3. **Infrastructure**: Repositories, language detection, slicing
4. **FastAPI App**: Hosts the API and manages background services

## Proposed Architecture

### Design Principles

1. **Server-Only**: Queue is always active in `serve` mode; CLI commands remain unchanged
2. **Single Process**: Queue and worker run in the same FastAPI/uvicorn process
3. **Mandatory in Server**: No optional checks - services always use queue in server mode
4. **Priority & Deduplication**: All tasks are prioritized and deduplicated automatically

### Core Components

#### 1. Queue Configuration (`src/kodit/config.py`)

```python
class IndexingQueueConfig(BaseModel):
    """Configuration for the indexing queue (always enabled in serve mode)."""
    
    max_size: int = Field(
        default=1000,
        description="Maximum number of tasks in the queue"
    )
    num_workers: int = Field(
        default=1,
        description="Number of concurrent workers processing the queue"
    )
    task_timeout: int = Field(
        default=3600,
        description="Maximum time (seconds) for a single indexing task"
    )
    retry_attempts: int = Field(
        default=3,
        description="Number of retry attempts for failed tasks"
    )
    retry_delay: float = Field(
        default=5.0,
        description="Delay (seconds) between retry attempts"
    )
    default_priority: int = Field(
        default=0,
        description="Default priority for tasks when not specified (higher = more urgent)"
    )

class AppContext(BaseSettings):
    # ... existing fields ...
    
    indexing_queue: IndexingQueueConfig = Field(
        default=IndexingQueueConfig(),
        description="Configuration for the indexing queue"
    )
```

#### 2. Queue System (`src/kodit/infrastructure/queue/`)

##### Base Queue Interface
```python
# src/kodit/infrastructure/queue/base.py
from abc import ABC, abstractmethod
from typing import TypeVar, Generic, AsyncIterator

T = TypeVar('T')

class QueueInterface(ABC, Generic[T]):
    @abstractmethod
    async def publish(self, item: T) -> None:
        """Publish an item to the queue."""
        
    @abstractmethod
    async def consume(self) -> AsyncIterator[T]:
        """Consume items from the queue."""
        
    @abstractmethod
    async def size(self) -> int:
        """Get the current queue size."""
```

##### Deduplicating Priority Queue Implementation
```python
# src/kodit/infrastructure/queue/dedup_priority_queue.py
import asyncio
import heapq
from dataclasses import dataclass, field
from typing import Any, TypeVar, Generic, AsyncIterator, Protocol

T = TypeVar('T')

class Identifiable(Protocol):
    """Protocol for items that can provide a unique identifier."""
    def get_unique_id(self) -> str:
        """Return unique identifier for deduplication."""
        ...

@dataclass(order=True)
class PrioritizedItem:
    priority: int
    sequence: int = field(compare=True)  # For FIFO within same priority
    item: Any = field(compare=False)
    unique_id: str = field(compare=False)

class DedupPriorityQueue(QueueInterface[T]):
    """Priority queue with deduplication - prevents duplicate items."""
    
    def __init__(self, maxsize: int = 0):
        self.maxsize = maxsize
        self._heap: list[PrioritizedItem] = []
        self._item_map: dict[str, PrioritizedItem] = {}  # Track items by unique ID
        self._sequence = 0
        self._lock = asyncio.Lock()
        self._not_empty = asyncio.Condition()
        self._not_full = asyncio.Condition()
    
    async def publish(self, item: T, priority: int = 0) -> bool:
        """Add item to queue with priority. Returns True if added, False if already exists.
        
        If item already exists with lower priority, updates to higher priority.
        """
        # Get unique ID from item
        if hasattr(item, 'get_unique_id'):
            unique_id = item.get_unique_id()
        elif hasattr(item, 'source_uri'):
            unique_id = item.source_uri  # For IndexingTask
        else:
            unique_id = str(item)
        
        async with self._lock:
            # Check if item already exists
            if unique_id in self._item_map:
                existing = self._item_map[unique_id]
                # Only update if new priority is higher (more negative in heap)
                if -priority < existing.priority:
                    # Update priority by removing and re-adding
                    self._heap.remove(existing)
                    heapq.heapify(self._heap)
                    
                    new_item = PrioritizedItem(
                        priority=-priority,
                        sequence=self._sequence,
                        item=item,
                        unique_id=unique_id
                    )
                    heapq.heappush(self._heap, new_item)
                    self._item_map[unique_id] = new_item
                    self._sequence += 1
                    return True
                return False  # Item exists with same or higher priority
            
            # Check queue size
            if self.maxsize and len(self._heap) >= self.maxsize:
                async with self._not_full:
                    await self._not_full.wait()
            
            # Add new item
            prioritized_item = PrioritizedItem(
                priority=-priority,  # Negative for min-heap (higher priority first)
                sequence=self._sequence,
                item=item,
                unique_id=unique_id
            )
            heapq.heappush(self._heap, prioritized_item)
            self._item_map[unique_id] = prioritized_item
            self._sequence += 1
        
        async with self._not_empty:
            self._not_empty.notify()
        
        return True
    
    async def consume(self) -> AsyncIterator[T]:
        """Consume items from queue in priority order."""
        while True:
            async with self._not_empty:
                while not self._heap:
                    await self._not_empty.wait()
                
                async with self._lock:
                    prioritized_item = heapq.heappop(self._heap)
                    # Remove from tracking map
                    del self._item_map[prioritized_item.unique_id]
                    item = prioritized_item.item
                
                async with self._not_full:
                    self._not_full.notify()
                
                yield item
    
    async def size(self) -> int:
        """Get current queue size."""
        async with self._lock:
            return len(self._heap)
    
    async def contains(self, unique_id: str) -> bool:
        """Check if an item with the given ID is in the queue."""
        async with self._lock:
            return unique_id in self._item_map
    
    async def peek_all(self) -> list[T]:
        """Get all items in priority order without removing them."""
        async with self._lock:
            # Sort heap items by priority and sequence
            sorted_items = sorted(self._heap)
            return [item.item for item in sorted_items]
```

#### 2. Indexing Queue Messages (`src/kodit/domain/value_objects.py`)

```python
from enum import Enum
from dataclasses import dataclass
from typing import Optional

class IndexingTaskType(Enum):
    CREATE_INDEX = "create_index"
    UPDATE_INDEX = "update_index"
    DELETE_INDEX = "delete_index"
    REINDEX = "reindex"

@dataclass
class IndexingTask:
    """Represents an indexing task to be processed."""
    task_id: str
    task_type: IndexingTaskType
    source_uri: str
    index_id: Optional[int] = None
    priority: int = 0
    retry_count: int = 0
    max_retries: int = 3
    metadata: dict = field(default_factory=dict)
    
    def get_unique_id(self) -> str:
        """Return unique identifier for deduplication.
        
        For CREATE_INDEX and REINDEX: source_uri
        For UPDATE_INDEX and DELETE_INDEX: index_id
        """
        if self.task_type in (IndexingTaskType.CREATE_INDEX, IndexingTaskType.REINDEX):
            return f"{self.task_type.value}:{self.source_uri}"
        elif self.index_id is not None:
            return f"{self.task_type.value}:{self.index_id}"
        else:
            return f"{self.task_type.value}:{self.source_uri}"
```

#### 3. Queue-Based Application Service

##### Internal Queue Service (Used by existing services)
```python
# src/kodit/application/services/indexing_queue_service.py
class IndexingQueueService:
    """Internal service for queue operations - used by other services."""
    
    def __init__(self, queue: DedupPriorityQueue[IndexingTask]):
        self.queue = queue
        self.log = structlog.get_logger(__name__)
        
    async def queue_index_task(
        self, source_uri: str, priority: int = 0
    ) -> tuple[str, bool]:
        """Queue an indexing task internally (e.g., from auto-indexing service).
        
        Returns: (task_id, was_added) tuple
        """
        task = IndexingTask(
            task_id=str(uuid.uuid4()),
            task_type=IndexingTaskType.CREATE_INDEX,
            source_uri=source_uri,
            priority=priority
        )
        was_added = await self.queue.publish(task, priority)
        
        if was_added:
            self.log.info(
                "Indexing task queued internally",
                task_id=task.task_id,
                source_uri=source_uri,
                priority=priority
            )
        else:
            self.log.info(
                "Indexing task already in queue",
                source_uri=source_uri,
                priority=priority
            )
        
        return task.task_id, was_added
```

##### Worker Service
```python
# src/kodit/application/services/indexing_worker_service.py
class IndexingWorkerService:
    """Service for processing indexing tasks from the queue."""
    
    def __init__(
        self,
        queue: DedupPriorityQueue[IndexingTask],
        app_context: AppContext,
        session_factory: Callable[[], AsyncSession]
    ):
        self.queue = queue
        self.app_context = app_context
        self.session_factory = session_factory
        self._workers: List[asyncio.Task] = []
        self.log = structlog.get_logger(__name__)
        
    async def start_workers(self, num_workers: int = 1) -> None:
        """Start worker tasks to process the queue."""
        for i in range(num_workers):
            worker = asyncio.create_task(self._worker_loop(i))
            self._workers.append(worker)
    
    async def _worker_loop(self, worker_id: int) -> None:
        """Main worker loop for processing tasks."""
        self.log.info(f"Worker {worker_id} started")
        async for task in self.queue.consume():
            await self._process_task(task, worker_id)
    
    async def _process_task(self, task: IndexingTask, worker_id: int) -> None:
        """Process a single indexing task."""
        self.log.info(
            "Processing task",
            worker_id=worker_id,
            task_id=task.task_id,
            task_type=task.task_type.value,
            source_uri=task.source_uri
        )
        
        async with self.session_factory() as session:
            service = create_code_indexing_application_service(
                app_context=self.app_context,
                session=session,
            )
            
            try:
                if task.task_type == IndexingTaskType.CREATE_INDEX:
                    # Create and run index
                    index = await service.create_index_from_uri(task.source_uri)
                    await service.run_index(index)
                    
                elif task.task_type == IndexingTaskType.UPDATE_INDEX:
                    # Update existing index
                    if task.index_id:
                        index = await service.index_repository.get(task.index_id)
                        if index:
                            await service.run_index(index)
                    
                self.log.info("Task completed successfully", task_id=task.task_id)
                
            except Exception as e:
                await self._handle_task_failure(task, e)
    
    async def _handle_task_failure(self, task: IndexingTask, error: Exception) -> None:
        """Handle failed task with retry logic."""
        self.log.exception(
            "Task failed",
            task_id=task.task_id,
            retry_count=task.retry_count,
            error=str(error)
        )
        
        if task.retry_count < task.max_retries:
            # Re-queue with incremented retry count
            task.retry_count += 1
            await asyncio.sleep(self.app_context.indexing_queue.retry_delay)
            await self.queue.publish(task, task.priority)
            self.log.info("Task re-queued for retry", task_id=task.task_id)
```

#### 4. Service Refactoring

##### AutoIndexingService Refactoring
```python
# src/kodit/application/services/auto_indexing_service.py
class AutoIndexingService:
    """Service for automatically indexing configured sources via queue."""
    
    def __init__(
        self,
        app_context: AppContext,
        session_factory: Callable[[], AsyncSession],
        queue_service: IndexingQueueService,  # Required in server mode
    ) -> None:
        self.app_context = app_context
        self.session_factory = session_factory
        self.queue_service = queue_service
        self.log = structlog.get_logger(__name__)
    
    async def _index_sources(self, sources: list[str]) -> None:
        """Queue all configured sources for indexing."""
        for source in sources:
            try:
                # Check if index exists
                async with self.session_factory() as session:
                    service = create_code_indexing_application_service(
                        app_context=self.app_context,
                        session=session,
                    )
                    if await service.does_index_exist(source):
                        self.log.info("Index already exists, skipping", source=source)
                        continue
                
                # Queue the indexing task
                task_id, was_added = await self.queue_service.queue_index_task(
                    source_uri=source,
                    priority=self.app_context.indexing_queue.default_priority
                )
                
                if was_added:
                    self.log.info("Queued auto-indexing task", source=source, task_id=task_id)
                else:
                    self.log.info("Auto-indexing task already queued", source=source)
                    
            except Exception as exc:
                self.log.exception("Failed to queue auto-index", source=source, error=str(exc))
```

##### SyncSchedulerService Refactoring
```python
# src/kodit/application/services/sync_scheduler.py
class SyncSchedulerService:
    """Service for scheduling periodic sync operations via queue."""
    
    def __init__(
        self,
        app_context: AppContext,
        session_factory: Callable[[], AsyncSession],
        queue_service: IndexingQueueService,  # Required in server mode
    ) -> None:
        self.app_context = app_context
        self.session_factory = session_factory
        self.queue_service = queue_service
        self.log = structlog.get_logger(__name__)
    
    async def _perform_sync(self) -> None:
        """Queue sync operations for all indexes."""
        self.log.info("Starting sync operation")
        
        async with self.session_factory() as session:
            # Get all indexes that need syncing
            index_query_service = IndexQueryService(
                index_repository=SqlAlchemyIndexRepository(session=session),
                fusion_service=ReciprocalRankFusionService(),
            )
            indexes = await index_query_service.list_indexes()
            
            # Queue sync tasks
            for index in indexes:
                task = IndexingTask(
                    task_id=str(uuid.uuid4()),
                    task_type=IndexingTaskType.UPDATE_INDEX,
                    source_uri=str(index.source.working_copy.remote_uri),
                    index_id=index.id,
                    priority=0,  # Lower priority for sync
                )
                was_added = await self.queue_service.queue.publish(task, task.priority)
                
                if was_added:
                    self.log.info("Queued sync task", index_id=index.id)
```

##### API Handler Refactoring
```python
# src/kodit/infrastructure/api/v1/routers/indexes.py
@router.post("", status_code=202)
async def create_index(
    request: IndexCreateRequest,
    app_service: IndexingAppServiceDep,
    queue_service: IndexingQueueService = Depends(get_queue_service),
) -> IndexResponse:
    """Create a new index and queue indexing task."""
    # Create index structure
    index = await app_service.create_index_from_uri(request.data.attributes.uri)
    
    # Queue the indexing task with higher priority for API requests
    task_id, was_added = await queue_service.queue_index_task(
        source_uri=request.data.attributes.uri,
        priority=10  # Higher priority for user-initiated requests
    )
    
    if not was_added:
        self.log.info("Index task already queued", source_uri=request.data.attributes.uri)
    
    return IndexResponse(
        data=IndexData(
            type="index",
            id=str(index.id),
            attributes=IndexAttributes(
                created_at=index.created_at,
                updated_at=index.updated_at,
                uri=str(index.source.working_copy.remote_uri),
            ),
        )
    )
```

#### 5. Factory Updates

```python
# src/kodit/application/factories/queue_factory.py
def create_indexing_queue(app_context: AppContext) -> DedupPriorityQueue[IndexingTask]:
    """Factory for creating the deduplicating indexing queue."""
    return DedupPriorityQueue(maxsize=app_context.indexing_queue.max_size)

def create_indexing_queue_service(app_context: AppContext) -> IndexingQueueService:
    """Factory for creating the internal queue service."""
    queue = create_indexing_queue(app_context)
    return IndexingQueueService(queue)

def create_indexing_worker(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession]
) -> IndexingWorkerService:
    """Factory for creating the indexing worker."""
    queue = create_indexing_queue(app_context)
    return IndexingWorkerService(queue, app_context, session_factory)

def get_queue_service(
    app_context: AppContext = Depends(get_app_context)
) -> IndexingQueueService:
    """Get the queue service (always available in server mode)."""
    return create_indexing_queue_service(app_context)
```

#### 6. FastAPI Integration Updates

```python
# src/kodit/app.py
from kodit.application.services.indexing_worker_service import IndexingWorkerService
from kodit.application.factories.queue_factory import create_indexing_worker

# Global services
_indexing_worker_service: IndexingWorkerService | None = None

@asynccontextmanager
async def app_lifespan(_: FastAPI) -> AsyncIterator[AppLifespanState]:
    """Manage application lifespan for auto-indexing, sync, and queue worker."""
    global _indexing_worker_service
    
    app_context = AppContext()
    db = await app_context.get_db()
    
    # ... existing auto-indexing and sync scheduler setup ...
    
    # Create queue service (always enabled in server mode)
    queue_service = create_indexing_queue_service(app_context)
    
    # Start indexing queue worker
    _indexing_worker_service = create_indexing_worker(
        app_context=app_context,
        session_factory=db.session_factory
    )
    await _indexing_worker_service.start_workers(
        num_workers=app_context.indexing_queue.num_workers
    )
    
    # Initialize auto-indexing service with queue
    _auto_indexing_service = AutoIndexingService(
        app_context=app_context,
        session_factory=db.session_factory,
        queue_service=queue_service
    )
    await _auto_indexing_service.start_background_indexing()
    
    # Initialize sync scheduler service with queue
    if app_context.periodic_sync.enabled:
        _sync_scheduler_service = SyncSchedulerService(
            app_context=app_context,
            session_factory=db.session_factory,
            queue_service=queue_service
        )
        _sync_scheduler_service.start_periodic_sync(
            interval_seconds=app_context.periodic_sync.interval_seconds
        )
    
    yield AppLifespanState(app_context=app_context)
    
    # Stop services
    if _indexing_worker_service:
        await _indexing_worker_service.stop_workers()
    # ... existing cleanup ...
```

#### 7. Queue Status API Endpoint

```python
# src/kodit/infrastructure/api/v1/routers/queue.py
from kodit.infrastructure.queue.dedup_priority_queue import DedupPriorityQueue
from kodit.domain.value_objects import IndexingTask

@router.get("/queue")
async def get_queue_status(
    queue: DedupPriorityQueue[IndexingTask] = Depends(get_indexing_queue)
) -> QueueStatusResponse:
    """Get the current status of the indexing queue."""
    size = await queue.size()
    items = await queue.peek_all()  # Non-destructive view of queue items
    
    return QueueStatusResponse(
        queue_size=size,
        max_size=queue.maxsize,
        items=[
            QueueItemResponse(
                task_id=item.task_id,
                task_type=item.task_type.value,
                source_uri=item.source_uri,
                priority=item.priority,
                position=idx
            )
            for idx, item in enumerate(items)
        ]
    )

@router.get("/queue/{task_id}")
async def get_task_in_queue(
    task_id: str,
    queue: DedupPriorityQueue[IndexingTask] = Depends(get_indexing_queue)
) -> QueueItemResponse | None:
    """Check if a specific task is in the queue."""
    items = await queue.peek_all()
    for idx, item in enumerate(items):
        if item.task_id == task_id:
            return QueueItemResponse(
                task_id=item.task_id,
                task_type=item.task_type.value,
                source_uri=item.source_uri,
                priority=item.priority,
                position=idx
            )
    return None
```

## Implementation Phases

### Phase 1: Configuration & Queue Infrastructure (Days 1-2)
1. Add `IndexingQueueConfig` to `config.py`
2. Implement `QueueInterface` abstract base class
3. Create `AsyncIOQueue` with priority support
4. Add `IndexingTask` value object
5. Write unit tests for priority queue logic

### Phase 2: Queue & Worker Services (Days 3-4)
1. Implement `IndexingQueueService` for internal use
2. Implement `IndexingWorkerService` with retry logic
3. Add queue factory functions
4. Test worker lifecycle management

### Phase 3: Service Refactoring (Days 5-6)
1. Refactor `AutoIndexingService` to use queue
2. Refactor `SyncSchedulerService` to use queue
3. Update API handlers to use queue
4. Update `app.py` to wire everything together

### Phase 4: FastAPI Integration (Days 7-8)
1. Add queue monitoring API endpoints
2. Test integration with existing services
3. Verify queue deduplication works correctly
4. Performance testing with concurrent operations

### Phase 5: Testing & Documentation (Days 9-10)
1. End-to-end testing with queue in serve mode
2. Verify CLI commands remain unchanged
3. Test queue deduplication with duplicate requests
4. Performance testing with concurrent requests
5. Update API documentation

## Benefits

1. **Non-blocking API**: FastAPI endpoints return immediately after queuing tasks
2. **Resource Management**: Control concurrent indexing operations via worker count
3. **Priority Queue**: All tasks are prioritized, with configurable default priority
4. **Deduplication**: Prevents duplicate indexing of the same source
5. **Priority Updates**: Higher priority requests automatically update existing tasks
6. **Resilience**: Failed tasks are automatically retried
7. **Single Process**: No additional infrastructure required (runs in uvicorn)
8. **Simplified Code**: No conditional logic for queue vs direct execution

## Addressing Your Questions

### 1. Queue Only in Serve Command
✅ The queue is only activated when running `kodit serve`. All CLI commands (`index`, `search`, etc.) continue to work synchronously as before.

### 2. Configuration in config.py
✅ Added `IndexingQueueConfig` class with all queue settings, accessible via `app_context.indexing_queue`.

### 3. Deduplicating Priority Queue with heapq
The `DedupPriorityQueue` implementation:
- Uses `heapq` for efficient priority ordering
- Maintains a dictionary to track unique items and prevent duplicates
- Automatically updates priority if a higher priority request comes in for the same item
- Returns feedback on whether item was added or already existed
- Thread-safe operations using `asyncio.Lock` and `asyncio.Condition`
- Items must provide a unique identifier (via `get_unique_id()` method or `source_uri` attribute)

### 4. asyncio and FastAPI/Uvicorn Compatibility
The queue works seamlessly with FastAPI:
- All queue operations use `asyncio` primitives that run in the same event loop as FastAPI
- The queue operations are non-blocking (using `await`)
- FastAPI can handle requests while workers process tasks in the background
- All runs in a single process/event loop, so no IPC overhead

## Risks & Mitigations

### Risk 1: In-Memory Queue Data Loss
- **Mitigation**: Tasks are lost on server restart (acceptable for initial version)
- **Future**: Add optional Redis persistence

### Risk 2: Queue Overflow
- **Mitigation**: Configure `max_size` limit, return 503 when queue is full

### Risk 3: Long-Running Tasks
- **Mitigation**: Configure `task_timeout` to prevent stuck workers

## Example Usage

### Starting the Server with Queue
```bash
# Start server with default queue settings
kodit serve

# Start with custom queue configuration via environment variables
INDEXING_QUEUE_NUM_WORKERS=3 \
INDEXING_QUEUE_MAX_SIZE=5000 \
INDEXING_QUEUE_DEFAULT_PRIORITY=5 \
kodit serve
```

### Queue Monitoring API
```bash
# View entire queue status
GET /api/v1/queue

# Response:
{
    "queue_size": 5,
    "max_size": 1000,
    "items": [
        {
            "task_id": "abc-123",
            "task_type": "create_index",
            "source_uri": "https://github.com/example/repo",
            "priority": 10,
            "position": 0
        },
        ...
    ]
}

# Check if specific task is in queue
GET /api/v1/queue/{task_id}
```

## Monitoring & Observability

1. **Queue Metrics**:
   - Queue size
   - Task throughput
   - Task latency
   - Failed task count

2. **Worker Metrics**:
   - Active workers
   - Tasks processed per worker
   - Worker health status

3. **Logging**:
   - Task lifecycle events
   - Worker start/stop events
   - Error and retry events

## Migration Strategy

1. **No Breaking Changes**: Existing CLI commands remain unchanged
2. **Server Mode Only**: Queue is always active when running `kodit serve`
3. **Internal Only**: Queue is used internally by services, no public POST endpoints
4. **Monitoring**: New `/queue` endpoints for visibility into queue state
5. **Testing**: Test in development with `serve` before production deployment

## Alternative Approaches Considered

1. **Celery**: Rejected - requires separate worker process and message broker
2. **Background Tasks (FastAPI)**: Rejected - no queue management or retry logic
3. **Threading**: Rejected - GIL limitations, harder to manage
4. **Separate Worker Process**: Rejected - adds deployment complexity

## Success Criteria

1. FastAPI server remains responsive during indexing operations
2. Queue handles 100+ concurrent indexing requests
3. Deduplication prevents duplicate tasks in queue
4. Priority updates work correctly for existing tasks
4. Failed tasks are retried as configured
5. No impact on CLI command functionality

## Next Steps

1. Review and approve updated plan
2. Implement Phase 1 (Configuration & Queue)
3. Add unit tests for priority queue
4. Implement worker service
5. Integrate with FastAPI app