"""Tests for RepositoryStatusSummary domain entity."""

from datetime import UTC, datetime

from kodit.domain.entities import RepositoryStatusSummary, TaskStatus
from kodit.domain.value_objects import (
    IndexStatus,
    ReportingState,
    TaskOperation,
    TrackableType,
)


def _create_task(
    operation: TaskOperation = TaskOperation.ROOT,
    state: ReportingState = ReportingState.STARTED,
    error: str | None = None,
    updated_at: datetime | None = None,
) -> TaskStatus:
    """Create a task status for testing."""
    task = TaskStatus.create(
        operation=operation,
        trackable_type=TrackableType.KODIT_REPOSITORY,
        trackable_id=1,
    )
    task.state = state
    task.error = error
    if updated_at:
        task.updated_at = updated_at
    return task


class TestFromTasks:
    """Tests for RepositoryStatusSummary.from_tasks.

    Priority: failed > in_progress > completed > pending.
    Timestamp reflects the most recent task with the reported status.
    """

    def test_empty_tasks_returns_pending(self) -> None:
        """Empty task list should return pending status with current time."""
        summary = RepositoryStatusSummary.from_tasks([])

        assert summary.status == IndexStatus.PENDING
        assert summary.message == ""
        assert summary.updated_at is not None

    # --- Failed status tests ---

    def test_any_failed_returns_failed(self) -> None:
        """Any FAILED task should return failed, even with completed tasks."""
        older_time = datetime(2024, 1, 1, tzinfo=UTC)
        newer_time = datetime(2024, 1, 2, tzinfo=UTC)

        tasks = [
            _create_task(
                state=ReportingState.COMPLETED,
                updated_at=newer_time,  # More recent but completed
            ),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.FAILED,
                error="Clone failed",
                updated_at=older_time,  # Older but failed
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.FAILED
        assert summary.message == "Clone failed"

    def test_failed_uses_failed_task_timestamp(self) -> None:
        """Failed status should use the failed task's timestamp, not overall."""
        completed_time = datetime(2024, 1, 2, tzinfo=UTC)
        failed_time = datetime(2024, 1, 1, tzinfo=UTC)

        tasks = [
            _create_task(
                state=ReportingState.COMPLETED,
                updated_at=completed_time,
            ),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.FAILED,
                error="Error",
                updated_at=failed_time,
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.FAILED
        assert summary.updated_at == failed_time  # Uses failed task's time

    def test_multiple_failures_uses_most_recent_failed(self) -> None:
        """With multiple failures, use the most recent failed task."""
        older_failure = datetime(2024, 1, 1, tzinfo=UTC)
        newer_failure = datetime(2024, 1, 2, tzinfo=UTC)

        tasks = [
            _create_task(
                state=ReportingState.FAILED,
                error="Old error",
                updated_at=older_failure,
            ),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.FAILED,
                error="New error",
                updated_at=newer_failure,
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.FAILED
        assert summary.message == "New error"
        assert summary.updated_at == newer_failure

    def test_failed_with_none_error_uses_empty_message(self) -> None:
        """Failed task with no error message should use empty string."""
        tasks = [
            _create_task(
                state=ReportingState.FAILED,
                error=None,
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.FAILED
        assert summary.message == ""

    # --- In progress status tests ---

    def test_any_in_progress_returns_in_progress(self) -> None:
        """Any IN_PROGRESS task should return in_progress (if no failures)."""
        older_time = datetime(2024, 1, 1, tzinfo=UTC)
        newer_time = datetime(2024, 1, 2, tzinfo=UTC)

        tasks = [
            _create_task(
                state=ReportingState.COMPLETED,
                updated_at=newer_time,  # More recent but completed
            ),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.IN_PROGRESS,
                updated_at=older_time,  # Older but in progress
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.IN_PROGRESS

    def test_any_started_returns_in_progress(self) -> None:
        """Any STARTED task should return in_progress (if no failures)."""
        tasks = [
            _create_task(state=ReportingState.COMPLETED),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.STARTED,
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.IN_PROGRESS

    def test_in_progress_uses_in_progress_task_timestamp(self) -> None:
        """In progress status should use the in_progress task's timestamp."""
        completed_time = datetime(2024, 1, 2, tzinfo=UTC)
        in_progress_time = datetime(2024, 1, 1, tzinfo=UTC)

        tasks = [
            _create_task(
                state=ReportingState.COMPLETED,
                updated_at=completed_time,
            ),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.IN_PROGRESS,
                updated_at=in_progress_time,
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.IN_PROGRESS
        assert summary.updated_at == in_progress_time

    def test_failed_takes_priority_over_in_progress(self) -> None:
        """Failed should take priority over in_progress."""
        tasks = [
            _create_task(state=ReportingState.IN_PROGRESS),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.FAILED,
                error="Error",
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.FAILED

    # --- Completed status tests ---

    def test_all_completed_returns_completed(self) -> None:
        """All COMPLETED tasks should return completed."""
        tasks = [
            _create_task(state=ReportingState.COMPLETED),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.COMPLETED,
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.COMPLETED

    def test_all_skipped_returns_completed(self) -> None:
        """All SKIPPED tasks should return completed."""
        tasks = [
            _create_task(state=ReportingState.SKIPPED),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.SKIPPED,
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.COMPLETED

    def test_mixed_terminal_returns_completed(self) -> None:
        """Mix of COMPLETED and SKIPPED should return completed."""
        tasks = [
            _create_task(state=ReportingState.COMPLETED),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.SKIPPED,
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.COMPLETED

    def test_completed_uses_most_recent_timestamp(self) -> None:
        """Completed status should use most recent task's timestamp."""
        older_time = datetime(2024, 1, 1, tzinfo=UTC)
        newer_time = datetime(2024, 1, 2, tzinfo=UTC)

        tasks = [
            _create_task(state=ReportingState.COMPLETED, updated_at=older_time),
            _create_task(
                operation=TaskOperation.CLONE_REPOSITORY,
                state=ReportingState.COMPLETED,
                updated_at=newer_time,
            ),
        ]

        summary = RepositoryStatusSummary.from_tasks(tasks)

        assert summary.status == IndexStatus.COMPLETED
        assert summary.updated_at == newer_time
