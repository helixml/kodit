# Progress Reporting System Design

## Overview

This document outlines a unified, modular progress reporting mechanism for the Kodit codebase. The design aims to replace the current fragmented approach with a consistent, extensible system that supports multiple output targets (CLI/tqdm, logging, database callbacks) while providing simple current/total progress updates.

## Current State Analysis

The codebase currently has several progress implementations:

### Existing Components

1. **`ProgressEvent`** (`domain/value_objects.py:393`) - Core data structure with `operation`, `current`, `total`, and `message`
2. **`ProgressCallback`** (`domain/interfaces.py:8`) - Abstract interface with `on_progress()` and `on_complete()` methods
3. **`Reporter`** (`reporting.py`) - Unified logging/progress helper with lifecycle methods (`start`, `step`, `advance`, `done`)
4. **Progress UI Implementations** (`infrastructure/ui/progress.py`):
   - `TQDMProgressCallback` - TQDM integration
   - `LogProgressCallback` - Milestone-based logging
   - `LazyProgressCallback` - Only shows when work exists
   - `MultiStageProgressCallback` - Multiple operations

### Current Issues

- Fragmented across multiple files and concepts
- Complex lifecycle management (start/step/advance/done)
- Limited modularity for different output targets
- No centralized progress state management
- Database callback support not built-in

## Proposed Design

### Core Architecture

```
ProgressReporter (Main Interface)
├── ProgressState (Internal State)
├── ProgressModule (Abstract Base)
│   ├── TQDMModule
│   ├── LogModule
│   └── DatabaseModule
└── ProgressConfig
```

### Key Components

#### 1. ProgressReporter (Main Interface)

```python
class ProgressReporter:
    def __init__(self, operation: str, modules: list[ProgressModule], config: ProgressConfig = None)
    def set_total(self, total: int) -> None
    def set_current(self, current: int, message: str = None) -> None
    def increment(self, amount: int = 1, message: str = None) -> None
    def complete(self, message: str = None) -> None
```

#### 2. ProgressState (Internal State Management)

```python
@dataclass
class ProgressState:
    operation: str
    current: int = 0
    total: int = 0
    message: str | None = None
    start_time: datetime = field(default_factory=datetime.now)
    completed: bool = False
    
    @property
    def percentage(self) -> float
    @property
    def elapsed_time(self) -> timedelta
```

#### 3. ProgressModule (Modular Output)

```python
class ProgressModule(ABC):
    @abstractmethod
    def on_init(self, state: ProgressState) -> None
    @abstractmethod
    def on_update(self, state: ProgressState) -> None
    @abstractmethod
    def on_complete(self, state: ProgressState) -> None
```

#### 4. Concrete Modules

- **TQDMModule** - CLI progress bars
- **LogModule** - Structured logging with configurable intervals
- **DatabaseModule** - Generic callback system for database updates

#### 5. ProgressConfig

```python
@dataclass
class ProgressConfig:
    log_interval: int = 10  # Log every N%
    min_update_interval: timedelta = timedelta(milliseconds=100)
    auto_complete: bool = True
```

## Implementation Plan

### Phase 1: Core Infrastructure

1. **Create new progress module system**
   - Implement `ProgressReporter`, `ProgressState`, `ProgressModule`
   - Create `ProgressConfig` with sensible defaults

2. **Implement concrete modules**
   - `TQDMModule` - Migrate existing TQDM logic
   - `LogModule` - Migrate existing log callback logic
   - `DatabaseModule` - Generic callback wrapper

### Phase 2: Integration

3. **Create factory functions**
   - `create_cli_reporter()` - TQDM + Log modules
   - `create_server_reporter()` - Log + Database modules
   - `create_test_reporter()` - Null or memory-based modules

4. **Update existing Reporter class**
   - Maintain backward compatibility
   - Delegate to new ProgressReporter internally

### Phase 3: Migration

5. **Replace usage across codebase**
   - Update services to use new `set_current(current, total)` pattern
   - Remove old start/step/advance/done lifecycle complexity
   - Migrate existing progress callbacks

6. **Clean up deprecated code**
   - Remove old progress implementations after migration
   - Update tests and documentation

## Module Specifications

### TQDMModule

- Uses existing TQDM integration patterns
- Handles dynamic total updates
- Message truncation/padding for consistent display
- Configurable position and styling

### LogModule

- Milestone-based logging (configurable percentage intervals)
- Rate limiting to prevent log spam
- Structured logging with operation context
- Support for completion messages

### DatabaseModule

- Generic callback mechanism: `Callable[[ProgressState], None]`
- Async callback support
- Error handling and retry logic
- Configurable update frequency

## Usage Examples

### Simple CLI Progress

```python
reporter = create_cli_reporter("Processing files")
reporter.set_total(1000)

for i, file in enumerate(files):
    process_file(file)
    reporter.set_current(i + 1, f"Processing {file.name}")

reporter.complete("All files processed")
```

### Server with Database Updates

```python
def update_job_progress(state: ProgressState):
    db.update_job(job_id, current=state.current, total=state.total)

reporter = ProgressReporter(
    "Indexing repository",
    modules=[
        LogModule(interval=5),  # Log every 5%
        DatabaseModule(callback=update_job_progress)
    ]
)

reporter.set_total(total_files)
for i, file in enumerate(files):
    index_file(file)
    reporter.set_current(i + 1)
```

### Multi-Module Configuration

```python
reporter = ProgressReporter(
    "Complex operation",
    modules=[
        TQDMModule(),
        LogModule(interval=10),
        DatabaseModule(callback=db_callback, update_interval=timedelta(seconds=1))
    ],
    config=ProgressConfig(
        min_update_interval=timedelta(milliseconds=50),
        auto_complete=True
    )
)
```

## Migration Strategy

### Backward Compatibility

- Keep existing `Reporter` class as facade over new system
- Maintain existing `ProgressCallback` interface
- Provide migration utilities and documentation

### Deprecation Timeline

1. **Phase 1**: Introduce new system alongside existing
2. **Phase 2**: Update internal usage to new system
3. **Phase 3**: Deprecate old interfaces with warnings
4. **Phase 4**: Remove deprecated code after 2-3 releases

## Benefits

### For Developers

- **Simpler API**: Just `set_current(current, total)` vs complex lifecycle
- **Modular**: Mix and match output targets as needed
- **Consistent**: Same interface across CLI, server, and test environments
- **Extensible**: Easy to add new output modules (webhooks, metrics, etc.)

### For Operations

- **Better Observability**: Consistent progress reporting in logs
- **Database Integration**: Built-in support for persisting progress
- **Performance**: Rate limiting and batching to prevent overhead

### For Testing

- **Mockable**: Easy to test progress logic without actual I/O
- **Verifiable**: Capture and assert on progress updates
- **Isolated**: No side effects in unit tests

## Technical Considerations

### Performance

- Rate limiting to prevent excessive updates
- Lazy initialization of expensive resources (TQDM bars)
- Efficient state diffing to minimize redundant work

### Error Handling

- Module failures don't crash progress reporting
- Graceful degradation when modules unavailable
- Comprehensive logging of module errors

### Concurrency

- Thread-safe state updates
- Async module support for database callbacks
- No blocking operations in progress reporting

## Files to Modify

### New Files

- `src/kodit/progress/reporter.py` - Main ProgressReporter implementation
- `src/kodit/progress/modules/` - Directory for module implementations
  - `base.py` - ProgressModule abstract base
  - `tqdm.py` - TQDMModule implementation
  - `log.py` - LogModule implementation  
  - `database.py` - DatabaseModule implementation
- `src/kodit/progress/config.py` - ProgressConfig and factory functions

### Modified Files

- `src/kodit/reporting.py` - Update to use new system internally
- `src/kodit/infrastructure/ui/progress.py` - Migrate to new modules
- Services using progress reporting - Update to simpler API
- Tests - Update to use new interfaces

### Deprecated Files

- Eventually remove complex progress callback implementations
- Maintain minimal compatibility shims as needed
