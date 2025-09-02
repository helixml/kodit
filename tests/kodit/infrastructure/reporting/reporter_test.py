"""Test the Reporter."""

from unittest.mock import MagicMock

from kodit.domain.value_objects import StepState
from kodit.infrastructure.reporting.progress import Progress
from kodit.infrastructure.reporting.reporter import (
    Reporter,
    complete_step,
    create_index_operation,
    create_step,
)


def test_reporter_operation_workflow() -> None:
    """Test the operation workflow of the Reporter."""
    mock_progress = MagicMock(spec=Progress)
    reporter = Reporter(modules=[mock_progress])

    # Test operation start
    operation = create_index_operation(1, "test_operation")
    reporter.start_operation(operation)
    assert mock_progress.on_operation_start.call_count == 1
    assert mock_progress.on_operation_start.call_args[0][0].index_id == 1
    assert mock_progress.on_operation_start.call_args[0][0].type == "test_operation"

    # Test step update
    step = create_step("test_step")
    reporter.update_step(operation, step)
    assert mock_progress.on_step_update.call_count == 1
    assert mock_progress.on_step_update.call_args[0][0].index_id == 1
    assert mock_progress.on_step_update.call_args[0][1].name == "test_step"
    assert mock_progress.on_step_update.call_args[0][1].state == StepState.RUNNING

    # Test step completion
    completed_step = complete_step(step)
    reporter.update_step(operation, completed_step)
    assert mock_progress.on_step_update.call_count == 2
    assert mock_progress.on_step_update.call_args[0][1].state == StepState.COMPLETED
    assert mock_progress.on_step_update.call_args[0][1].progress_percentage == 100.0

    # Test operation complete
    reporter.complete_operation(operation)
    assert mock_progress.on_operation_complete.call_count == 1
