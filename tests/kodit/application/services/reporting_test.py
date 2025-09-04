"""Test the reporting."""

import pytest

from kodit.application.services.reporting import OperationType, ProgressTracker
from kodit.domain.value_objects import ReportingState, TrackableType


@pytest.mark.asyncio
async def test_nested_progress_trackers_share_parent_info() -> None:
    """Test the reporting."""
    progress_tracker = ProgressTracker(OperationType.ROOT.value)
    await progress_tracker.set_tracking_info(1, TrackableType.INDEX)
    child_progress_tracker = progress_tracker.create_child(
        OperationType.CREATE_INDEX.value
    )
    assert progress_tracker.trackable_id == 1
    assert progress_tracker.trackable_type == TrackableType.INDEX
    assert child_progress_tracker.parent == progress_tracker
    assert child_progress_tracker.trackable_id == 1
    assert child_progress_tracker.trackable_type == TrackableType.INDEX


@pytest.mark.asyncio
async def test_progress_tracker_state_flow() -> None:
    """Test the progress tracker state flow."""
    progress_tracker = ProgressTracker(OperationType.ROOT.value)
    async with progress_tracker:
        assert progress_tracker.snapshot.state == ReportingState.STARTED
        await progress_tracker.set_total(100)
        await progress_tracker.set_current(50)
        assert progress_tracker.snapshot.state == ReportingState.IN_PROGRESS
    assert progress_tracker.snapshot.state == ReportingState.COMPLETED
    assert progress_tracker.snapshot.total == 100
