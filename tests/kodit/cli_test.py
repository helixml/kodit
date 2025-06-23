"""Test the CLI."""

import asyncio
from datetime import datetime, UTC
from pathlib import Path
import tempfile
from typing import Generator
import pytest
from click.testing import CliRunner
from sqlalchemy.ext.asyncio import AsyncSession, create_async_engine, async_sessionmaker
from unittest.mock import patch, AsyncMock, MagicMock

from kodit.cli import cli
from kodit.domain.entities import File, Index, Snippet, Source, SourceType
from kodit.domain.value_objects import SnippetSearchFilters, MultiSearchRequest


@pytest.fixture
def tmp_data_dir() -> Generator[Path, None, None]:
    """Create a temporary data directory."""
    with tempfile.TemporaryDirectory() as tmp_dir:
        yield Path(tmp_dir)


@pytest.fixture
def runner(tmp_data_dir: Path) -> Generator[CliRunner, None, None]:
    """Create a CliRunner instance."""
    runner = CliRunner()
    runner.env = {
        "DISABLE_TELEMETRY": "true",
        "DATA_DIR": str(tmp_data_dir),
        "DB_URL": f"sqlite+aiosqlite:///{tmp_data_dir}/test.db",
    }
    yield runner


def test_version_command(runner: CliRunner) -> None:
    """Test that the version command runs successfully."""
    result = runner.invoke(cli, ["version"])
    # The command should exit with success
    assert result.exit_code == 0


def test_telemetry_disabled_in_these_tests(runner: CliRunner) -> None:
    """Test that telemetry is disabled in these tests."""
    result = runner.invoke(cli, ["version"])
    assert result.exit_code == 0
    assert "Telemetry has been disabled" in result.output


def test_env_vars_work(runner: CliRunner) -> None:
    """Test that env vars work."""
    runner.env = {**runner.env, "LOG_LEVEL": "DEBUG"}
    result = runner.invoke(cli, ["index"])
    assert result.exit_code == 0
    assert result.output.count("debug") > 10  # The db spits out lots of debug messages


def test_dotenv_file_works(runner: CliRunner) -> None:
    """Test that the .env file works."""
    with tempfile.NamedTemporaryFile(delete=False) as f:
        f.write(b"LOG_LEVEL=DEBUG")
        f.flush()
        result = runner.invoke(cli, ["--env-file", f.name, "index"])
        assert result.exit_code == 0
        assert (
            result.output.count("debug") > 10
        )  # The db spits out lots of debug messages


def test_dotenv_file_not_found(runner: CliRunner) -> None:
    """Test that the .env file not found error is raised."""
    result = runner.invoke(cli, ["--env-file", "nonexistent.env", "index"])
    assert result.exit_code == 2
    assert "does not exist" in result.output


def test_search_language_filtering_help(runner: CliRunner) -> None:
    """Test that language filtering options are available in search commands."""

    # Test that language filter option is available in code search
    result = runner.invoke(cli, ["search", "code", "--help"])
    assert result.exit_code == 0
    assert "--language TEXT" in result.output
    assert "Filter by programming language" in result.output

    # Test that language filter option is available in keyword search
    result = runner.invoke(cli, ["search", "keyword", "--help"])
    assert result.exit_code == 0
    assert "--language TEXT" in result.output
    assert "Filter by programming language" in result.output

    # Test that language filter option is available in text search
    result = runner.invoke(cli, ["search", "text", "--help"])
    assert result.exit_code == 0
    assert "--language TEXT" in result.output
    assert "Filter by programming language" in result.output

    # Test that language filter option is available in hybrid search
    result = runner.invoke(cli, ["search", "hybrid", "--help"])
    assert result.exit_code == 0
    assert "--language TEXT" in result.output
    assert "Filter by programming language" in result.output


def test_search_language_filtering_with_mocks(runner: CliRunner) -> None:
    """Test that language filtering works in search commands using mocks."""

    # Mock the search functionality
    mock_snippets = [
        MagicMock(
            id=1,
            content="def hello_world():\n    print('Hello from Python!')",
            file=MagicMock(extension="py"),
        ),
        MagicMock(
            id=2,
            content="function helloWorld() {\n    console.log('Hello from JavaScript!');\n}",
            file=MagicMock(extension="js"),
        ),
        MagicMock(
            id=3,
            content='func helloWorld() {\n    fmt.Println("Hello from Go!")\n}',
            file=MagicMock(extension="go"),
        ),
    ]

    # Mock the indexing application service
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    # Mock the snippet application service
    mock_snippet_service = MagicMock()
    mock_snippet_service.search = AsyncMock(return_value=mock_snippets)

    with (
        patch(
            "kodit.cli.create_indexing_application_service", return_value=mock_service
        ),
        patch(
            "kodit.cli.create_snippet_application_service",
            return_value=mock_snippet_service,
        ),
    ):
        # Test code search with Python language filter
        result = runner.invoke(cli, ["search", "code", "hello", "--language", "python"])
        assert result.exit_code == 0

        # Verify that the search was called with the correct filters
        mock_service.search.assert_called_once()
        call_args = mock_service.search.call_args[0][0]
        assert isinstance(call_args, MultiSearchRequest)
        assert call_args.code_query == "hello"
        assert call_args.filters is not None
        assert call_args.filters.language == "python"


def test_search_filters_parsing(runner: CliRunner) -> None:
    """Test that search filters are properly parsed from CLI arguments."""

    # Mock the search functionality
    mock_snippets = [MagicMock(id=1, content="test snippet")]
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    with patch(
        "kodit.cli.create_indexing_application_service", return_value=mock_service
    ):
        # Test with all filter options
        result = runner.invoke(
            cli,
            [
                "search",
                "code",
                "test query",
                "--language",
                "python",
                "--author",
                "alice",
                "--created-after",
                "2023-01-01",
                "--created-before",
                "2023-12-31",
                "--source-repo",
                "github.com/example/repo",
            ],
        )

        assert result.exit_code == 0

        # Verify that the search was called with the correct filters
        mock_service.search.assert_called_once()
        call_args = mock_service.search.call_args[0][0]
        assert isinstance(call_args, MultiSearchRequest)
        assert call_args.code_query == "test query"
        assert call_args.filters is not None
        assert call_args.filters.language == "python"
        assert call_args.filters.author == "alice"
        assert call_args.filters.created_after is not None
        assert call_args.filters.created_before is not None
        assert call_args.filters.source_repo == "github.com/example/repo"


def test_search_without_filters(runner: CliRunner) -> None:
    """Test that search works without filters."""

    # Mock the search functionality
    mock_snippets = [MagicMock(id=1, content="test snippet")]
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    with patch(
        "kodit.cli.create_indexing_application_service", return_value=mock_service
    ):
        # Test without any filters
        result = runner.invoke(cli, ["search", "code", "test query"])

        assert result.exit_code == 0

        # Verify that the search was called without filters
        mock_service.search.assert_called_once()
        call_args = mock_service.search.call_args[0][0]
        assert isinstance(call_args, MultiSearchRequest)
        assert call_args.code_query == "test query"
        assert call_args.filters is None
