"""Test the reporting."""

import pytest

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.value_objects import ReportingState, TaskOperation, TrackableType


@pytest.mark.asyncio
async def test_nested_progress_trackers_share_parent_info() -> None:
    """Test the reporting."""
    progress_tracker = ProgressTracker.create(TaskOperation.RUN_INDEX)
    async with progress_tracker.create_child(
        TaskOperation.RUN_INDEX, trackable_id=1, trackable_type=TrackableType.INDEX
    ) as child_progress_tracker:
        assert progress_tracker.task_status.trackable_id == 1
        assert progress_tracker.task_status.trackable_type == TrackableType.INDEX
        assert child_progress_tracker.task_status.parent == progress_tracker.task_status
        assert child_progress_tracker.task_status.trackable_id == 1
        assert child_progress_tracker.task_status.trackable_type == TrackableType.INDEX


@pytest.mark.asyncio
async def test_progress_tracker_state_flow() -> None:
    """Test the progress tracker state flow."""
    progress_tracker = ProgressTracker.create(TaskOperation.RUN_INDEX)
    assert progress_tracker.task_status.state == ReportingState.STARTED
    await progress_tracker.set_total(100)
    await progress_tracker.set_current(50)
    assert progress_tracker.task_status.state == ReportingState.IN_PROGRESS
    async with progress_tracker.create_child(
        TaskOperation.RUN_INDEX, trackable_id=1, trackable_type=TrackableType.INDEX
    ) as child_progress_tracker:
        assert child_progress_tracker.task_status.state == ReportingState.STARTED
        await child_progress_tracker.set_total(100)
        await child_progress_tracker.set_current(50)
        assert child_progress_tracker.task_status.state == ReportingState.IN_PROGRESS
    assert child_progress_tracker.task_status.state == ReportingState.COMPLETED
    assert child_progress_tracker.task_status.total == 100
