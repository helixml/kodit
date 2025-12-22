"""Tests for Task domain entity."""

from datetime import UTC, datetime, timedelta

from kodit.domain.entities import Task
from kodit.domain.value_objects import TaskOperation


def test_calculate_backoff_delay_initial() -> None:
    """Test backoff delay for first retry (retry_count=0)."""
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )
    # At retry_count=0: 5 * 2^0 = 5 seconds
    assert task.calculate_backoff_delay() == 5


def test_calculate_backoff_delay_progressive() -> None:
    """Test backoff delay increases exponentially."""
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )

    # Test progressive backoff: 5, 10, 20, 40, 80, 160, 300 (capped)
    expected_delays = [5, 10, 20, 40, 80, 160, 300, 300]
    for i, expected in enumerate(expected_delays):
        task.retry_count = i
        assert task.calculate_backoff_delay() == expected, f"retry_count={i}"


def test_calculate_backoff_delay_capped_at_5_minutes() -> None:
    """Test backoff delay is capped at 300 seconds (5 minutes)."""
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )
    task.retry_count = 100  # Very high retry count
    assert task.calculate_backoff_delay() == 300


def test_mark_for_retry_increments_count() -> None:
    """Test mark_for_retry increments retry_count."""
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )
    assert task.retry_count == 0

    task.mark_for_retry()
    assert task.retry_count == 1

    task.mark_for_retry()
    assert task.retry_count == 2


def test_mark_for_retry_sets_next_retry_at() -> None:
    """Test mark_for_retry sets next_retry_at in the future."""
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )
    assert task.next_retry_at is None

    before = datetime.now(UTC)
    task.mark_for_retry()
    after = datetime.now(UTC)

    assert task.next_retry_at is not None
    # First retry: 5 seconds delay
    expected_min = before + timedelta(seconds=5)
    expected_max = after + timedelta(seconds=5)
    assert expected_min <= task.next_retry_at <= expected_max


def test_mark_for_retry_uses_correct_backoff() -> None:
    """Test mark_for_retry uses exponential backoff based on retry count."""
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )

    # First retry: 5 seconds (uses retry_count after increment)
    task.mark_for_retry()
    first_retry = task.next_retry_at
    assert first_retry is not None

    # Second retry: 10 seconds
    task.mark_for_retry()
    second_retry = task.next_retry_at
    assert second_retry is not None
    # Second retry should be further in the future (10s vs 5s from new base)
    assert second_retry > first_retry


def test_new_task_has_zero_retry_count() -> None:
    """Test newly created tasks have retry_count=0 and no next_retry_at."""
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )
    assert task.retry_count == 0
    assert task.next_retry_at is None
