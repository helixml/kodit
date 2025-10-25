"""Tests for SqlAlchemyTaskStatusRepository."""

from collections.abc import Callable

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import TaskStatus
from kodit.domain.value_objects import ReportingState, TaskOperation, TrackableType
from kodit.infrastructure.sqlalchemy.task_status_repository import (
    create_task_status_repository,
)


@pytest.fixture
def root_task_status() -> TaskStatus:
    """Create a root task status for testing."""
    return TaskStatus.create(
        operation=TaskOperation.ROOT,
        trackable_type=TrackableType.INDEX,
        trackable_id=1,
    )


@pytest.fixture
def child_task_status(root_task_status: TaskStatus) -> TaskStatus:
    """Create a child task status for testing."""
    return TaskStatus.create(
        operation=TaskOperation.CREATE_INDEX,
        parent=root_task_status,
        trackable_type=TrackableType.INDEX,
        trackable_id=1,
    )


@pytest.fixture
def grandchild_task_status(child_task_status: TaskStatus) -> TaskStatus:
    """Create a grandchild task status for testing."""
    return TaskStatus.create(
        operation=TaskOperation.EXTRACT_SNIPPETS,
        parent=child_task_status,
        trackable_type=TrackableType.INDEX,
        trackable_id=1,
    )


async def test_save_new_task_status(
    session_factory: Callable[[], AsyncSession],
    root_task_status: TaskStatus,
) -> None:
    """Test saving a new task status."""
    repository = create_task_status_repository(session_factory)

    # Save the task status
    await repository.save(root_task_status)

    # Load and verify
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 1
    assert loaded[0].id == root_task_status.id
    assert loaded[0].operation == TaskOperation.ROOT
    assert loaded[0].state == ReportingState.STARTED
    assert loaded[0].parent is None


async def test_save_task_status_hierarchy(
    session_factory: Callable[[], AsyncSession],
    root_task_status: TaskStatus,
    child_task_status: TaskStatus,
    grandchild_task_status: TaskStatus,
) -> None:
    """Test saving a task status hierarchy with multiple levels."""
    repository = create_task_status_repository(session_factory)

    # Save the hierarchy
    await repository.save(root_task_status)
    await repository.save(child_task_status)
    await repository.save(grandchild_task_status)

    # Load and verify
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 3

    # Find each task in the loaded list
    loaded_by_id = {task.id: task for task in loaded}

    # Verify root
    assert root_task_status.id in loaded_by_id
    root = loaded_by_id[root_task_status.id]
    assert root.parent is None
    assert root.operation == TaskOperation.ROOT

    # Verify child
    assert child_task_status.id in loaded_by_id
    child = loaded_by_id[child_task_status.id]
    assert child.parent is not None
    assert child.parent.id == root_task_status.id
    assert child.operation == TaskOperation.CREATE_INDEX

    # Verify grandchild
    assert grandchild_task_status.id in loaded_by_id
    grandchild = loaded_by_id[grandchild_task_status.id]
    assert grandchild.parent is not None
    assert grandchild.parent.id == child_task_status.id
    assert grandchild.operation == TaskOperation.EXTRACT_SNIPPETS


async def test_update_task_status(
    session_factory: Callable[[], AsyncSession],
    root_task_status: TaskStatus,
) -> None:
    """Test updating an existing task status."""
    repository = create_task_status_repository(session_factory)

    # Save initial task status
    await repository.save(root_task_status)

    # Update the task status
    root_task_status.set_total(100)
    root_task_status.set_current(50, "Processing...")
    root_task_status.state = ReportingState.IN_PROGRESS

    # Save the updated status
    await repository.save(root_task_status)

    # Load and verify updates
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 1
    assert loaded[0].state == ReportingState.IN_PROGRESS
    assert loaded[0].total == 100
    assert loaded[0].current == 50


async def test_update_task_status_to_completed(
    session_factory: Callable[[], AsyncSession],
    root_task_status: TaskStatus,
) -> None:
    """Test updating a task status to completed state."""
    repository = create_task_status_repository(session_factory)

    # Save initial task status
    await repository.save(root_task_status)

    # Update to in progress
    root_task_status.set_total(100)
    root_task_status.set_current(50)
    await repository.save(root_task_status)

    # Complete the task
    root_task_status.complete()
    await repository.save(root_task_status)

    # Load and verify
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 1
    assert loaded[0].state == ReportingState.COMPLETED
    assert loaded[0].current == 100
    assert loaded[0].total == 100


async def test_update_task_status_to_failed(
    session_factory: Callable[[], AsyncSession],
    root_task_status: TaskStatus,
) -> None:
    """Test updating a task status to failed state."""
    repository = create_task_status_repository(session_factory)

    # Save initial task status
    await repository.save(root_task_status)

    # Fail the task
    error_message = "Something went wrong"
    root_task_status.fail(error_message)
    await repository.save(root_task_status)

    # Load and verify
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 1
    assert loaded[0].state == ReportingState.FAILED
    assert loaded[0].error == error_message


async def test_update_child_task_in_hierarchy(
    session_factory: Callable[[], AsyncSession],
    root_task_status: TaskStatus,
    child_task_status: TaskStatus,
) -> None:
    """Test updating a child task status in a hierarchy."""
    repository = create_task_status_repository(session_factory)

    # Save the hierarchy
    await repository.save(root_task_status)
    await repository.save(child_task_status)

    # Update the child
    child_task_status.set_total(50)
    child_task_status.set_current(25)
    await repository.save(child_task_status)

    # Load and verify
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 2

    loaded_by_id = {task.id: task for task in loaded}
    child = loaded_by_id[child_task_status.id]
    assert child.total == 50
    assert child.current == 25
    assert child.parent is not None
    assert child.parent.id == root_task_status.id


async def test_delete_task_status(
    session_factory: Callable[[], AsyncSession],
    root_task_status: TaskStatus,
) -> None:
    """Test deleting a task status."""
    repository = create_task_status_repository(session_factory)

    # Save the task status
    await repository.save(root_task_status)

    # Verify it exists
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 1

    # Delete it
    await repository.delete(root_task_status)

    # Verify it's gone
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 0


async def test_load_empty_hierarchy(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test loading when no task statuses exist."""
    repository = create_task_status_repository(session_factory)

    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=999
    )
    assert len(loaded) == 0


async def test_multiple_trackables(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that task statuses are properly isolated by trackable_id."""
    repository = create_task_status_repository(session_factory)

    # Create task statuses for two different trackable_ids
    task1 = TaskStatus.create(
        operation=TaskOperation.ROOT,
        trackable_type=TrackableType.INDEX,
        trackable_id=1,
    )
    task2 = TaskStatus.create(
        operation=TaskOperation.ROOT,
        trackable_type=TrackableType.INDEX,
        trackable_id=2,
    )

    await repository.save(task1)
    await repository.save(task2)

    # Load for trackable_id 1
    loaded1 = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded1) == 1
    assert loaded1[0].trackable_id == 1

    # Load for trackable_id 2
    loaded2 = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=2
    )
    assert len(loaded2) == 1
    assert loaded2[0].trackable_id == 2


async def test_save_hierarchy_out_of_order(
    session_factory: Callable[[], AsyncSession],
    root_task_status: TaskStatus,
    child_task_status: TaskStatus,
) -> None:
    """Test saving child before parent (should save parent automatically)."""
    repository = create_task_status_repository(session_factory)

    # Save child first (should trigger parent save)
    await repository.save(child_task_status)

    # Load and verify both exist
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 2

    loaded_by_id = {task.id: task for task in loaded}
    assert root_task_status.id in loaded_by_id
    assert child_task_status.id in loaded_by_id


async def test_hierarchy_with_three_levels(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test a three-level hierarchy to ensure parent relationships work correctly."""
    repository = create_task_status_repository(session_factory)

    # Create a three-level hierarchy
    level1 = TaskStatus.create(
        operation=TaskOperation.ROOT,
        trackable_type=TrackableType.INDEX,
        trackable_id=1,
    )

    level2 = TaskStatus.create(
        operation=TaskOperation.CREATE_INDEX,
        parent=level1,
        trackable_type=TrackableType.INDEX,
        trackable_id=1,
    )

    level3 = TaskStatus.create(
        operation=TaskOperation.EXTRACT_SNIPPETS,
        parent=level2,
        trackable_type=TrackableType.INDEX,
        trackable_id=1,
    )

    # Save all three
    await repository.save(level1)
    await repository.save(level2)
    await repository.save(level3)

    # Load and verify hierarchy
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 3

    loaded_by_id = {task.id: task for task in loaded}

    # Verify level1 (root)
    assert level1.id in loaded_by_id
    loaded_level1 = loaded_by_id[level1.id]
    assert loaded_level1.parent is None

    # Verify level2 (middle)
    assert level2.id in loaded_by_id
    loaded_level2 = loaded_by_id[level2.id]
    assert loaded_level2.parent is not None
    assert loaded_level2.parent.id == level1.id

    # Verify level3 (leaf)
    assert level3.id in loaded_by_id
    loaded_level3 = loaded_by_id[level3.id]
    assert loaded_level3.parent is not None
    assert loaded_level3.parent.id == level2.id


async def test_update_preserves_hierarchy(
    session_factory: Callable[[], AsyncSession],
    root_task_status: TaskStatus,
    child_task_status: TaskStatus,
) -> None:
    """Test that updating a task status preserves its parent relationship."""
    repository = create_task_status_repository(session_factory)

    # Save the hierarchy
    await repository.save(root_task_status)
    await repository.save(child_task_status)

    # Update the child multiple times
    child_task_status.set_total(100)
    await repository.save(child_task_status)

    child_task_status.set_current(50)
    await repository.save(child_task_status)

    child_task_status.complete()
    await repository.save(child_task_status)

    # Load and verify hierarchy is preserved
    loaded = await repository.load_with_hierarchy(
        trackable_type=TrackableType.INDEX.value, trackable_id=1
    )
    assert len(loaded) == 2

    loaded_by_id = {task.id: task for task in loaded}
    child = loaded_by_id[child_task_status.id]
    assert child.parent is not None
    assert child.parent.id == root_task_status.id
    assert child.state == ReportingState.COMPLETED
