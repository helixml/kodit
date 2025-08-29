"""Test the Reporter."""

from unittest.mock import MagicMock

from kodit.domain.value_objects import ProgressState
from kodit.infrastructure.reporting.progress import Progress
from kodit.infrastructure.reporting.reporter import Reporter


def test_reporter_basic_workflow() -> None:
    """Test the basic workflow of the Reporter."""
    mock_progress = MagicMock(spec=Progress)
    reporter = Reporter(modules=[mock_progress])

    # Test update
    reporter.update(ProgressState(current=5, total=10, message="Halfway"))
    assert mock_progress.on_update.call_count == 1
    assert mock_progress.on_update.call_args[0][0].current == 5
    assert mock_progress.on_update.call_args[0][0].total == 10
    assert mock_progress.on_update.call_args[0][0].message == "Halfway"

    # Test complete
    reporter.complete()
    assert mock_progress.on_complete.call_count == 1
