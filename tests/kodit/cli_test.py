"""Test the CLI."""

import tempfile
from collections.abc import Generator
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from click.testing import CliRunner

from kodit.cli import cli
from kodit.domain.value_objects import MultiSearchRequest


@pytest.fixture
def tmp_data_dir() -> Generator[Path, None, None]:
    """Create a temporary data directory."""
    with tempfile.TemporaryDirectory() as tmp_dir:
        yield Path(tmp_dir)


@pytest.fixture
def runner(tmp_data_dir: Path) -> CliRunner:
    """Create a CliRunner instance."""
    runner = CliRunner()
    runner.env = {
        "DISABLE_TELEMETRY": "true",
        "DATA_DIR": str(tmp_data_dir),
        "DB_URL": f"sqlite+aiosqlite:///{tmp_data_dir}/test.db",
    }
    return runner


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
            content=(
                "function helloWorld() {\n    console.log('Hello from JavaScript!');\n}"
            ),
            file=MagicMock(extension="js"),
        ),
        MagicMock(
            id=3,
            content='func helloWorld() {\n    fmt.Println("Hello from Go!")\n}',
            file=MagicMock(extension="go"),
        ),
    ]

    # Mock the unified application service
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    with patch(
        "kodit.cli.create_code_indexing_application_service", return_value=mock_service
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
        "kodit.cli.create_code_indexing_application_service", return_value=mock_service
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
        "kodit.cli.create_code_indexing_application_service", return_value=mock_service
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


def test_search_language_filter_all_commands(runner: CliRunner) -> None:
    """Test language filtering across all search command types."""
    # Mock the search functionality
    mock_snippets = [MagicMock(id=1, content="test snippet")]
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    with patch(
        "kodit.cli.create_code_indexing_application_service", return_value=mock_service
    ):
        # Test code search with language filter
        result = runner.invoke(
            cli, ["search", "code", "test", "--language", "javascript"]
        )
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.language == "javascript"

        # Reset mock
        mock_service.search.reset_mock()

        # Test keyword search with language filter
        result = runner.invoke(
            cli, ["search", "keyword", "test", "--language", "python"]
        )
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.language == "python"

        # Reset mock
        mock_service.search.reset_mock()

        # Test text search with language filter
        result = runner.invoke(cli, ["search", "text", "test", "--language", "go"])
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.language == "go"

        # Reset mock
        mock_service.search.reset_mock()

        # Test hybrid search with language filter
        result = runner.invoke(
            cli,
            [
                "search",
                "hybrid",
                "--keywords",
                "test",
                "--code",
                "test",
                "--text",
                "test",
                "--language",
                "rust",
            ],
        )
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.language == "rust"


def test_search_author_filter(runner: CliRunner) -> None:
    """Test author filtering functionality."""
    # Mock the search functionality
    mock_snippets = [MagicMock(id=1, content="test snippet")]
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    with patch(
        "kodit.cli.create_code_indexing_application_service", return_value=mock_service
    ):
        # Test with author filter
        result = runner.invoke(cli, ["search", "code", "test", "--author", "john.doe"])
        assert result.exit_code == 0

        # Verify that the search was called with the correct author filter
        mock_service.search.assert_called_once()
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.author == "john.doe"

        # Test with author filter containing spaces
        mock_service.search.reset_mock()
        result = runner.invoke(cli, ["search", "code", "test", "--author", "John Doe"])
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.author == "John Doe"


def test_search_created_after_filter(runner: CliRunner) -> None:
    """Test created-after date filtering functionality."""
    # Mock the search functionality
    mock_snippets = [MagicMock(id=1, content="test snippet")]
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    with patch(
        "kodit.cli.create_code_indexing_application_service", return_value=mock_service
    ):
        # Test with created-after filter
        result = runner.invoke(
            cli, ["search", "code", "test", "--created-after", "2023-06-15"]
        )
        assert result.exit_code == 0

        # Verify that the search was called with the correct date filter
        mock_service.search.assert_called_once()
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.created_after is not None
        assert call_args.filters.created_after.strftime("%Y-%m-%d") == "2023-06-15"


def test_search_created_before_filter(runner: CliRunner) -> None:
    """Test created-before date filtering functionality."""
    # Mock the search functionality
    mock_snippets = [MagicMock(id=1, content="test snippet")]
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    with patch(
        "kodit.cli.create_code_indexing_application_service", return_value=mock_service
    ):
        # Test with created-before filter
        result = runner.invoke(
            cli, ["search", "code", "test", "--created-before", "2024-01-31"]
        )
        assert result.exit_code == 0

        # Verify that the search was called with the correct date filter
        mock_service.search.assert_called_once()
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.created_before is not None
        assert call_args.filters.created_before.strftime("%Y-%m-%d") == "2024-01-31"


def test_search_source_repo_filter(runner: CliRunner) -> None:
    """Test source repository filtering functionality."""
    # Mock the search functionality
    mock_snippets = [MagicMock(id=1, content="test snippet")]
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    with patch(
        "kodit.cli.create_code_indexing_application_service", return_value=mock_service
    ):
        # Test with source-repo filter
        result = runner.invoke(
            cli,
            ["search", "code", "test", "--source-repo", "github.com/example/project"],
        )
        assert result.exit_code == 0

        # Verify that the search was called with the correct source repo filter
        mock_service.search.assert_called_once()
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.source_repo == "github.com/example/project"


def test_search_multiple_filters_combination(runner: CliRunner) -> None:
    """Test combinations of multiple filters."""
    # Mock the search functionality
    mock_snippets = [MagicMock(id=1, content="test snippet")]
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    with patch(
        "kodit.cli.create_code_indexing_application_service", return_value=mock_service
    ):
        # Test language + author combination
        result = runner.invoke(
            cli, ["search", "code", "test", "--language", "python", "--author", "alice"]
        )
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.language == "python"
        assert call_args.filters.author == "alice"

        # Reset mock
        mock_service.search.reset_mock()

        # Test language + date combination
        result = runner.invoke(
            cli,
            [
                "search",
                "code",
                "test",
                "--language",
                "javascript",
                "--created-after",
                "2023-01-01",
            ],
        )
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.language == "javascript"
        assert call_args.filters.created_after is not None
        assert call_args.filters.created_after.strftime("%Y-%m-%d") == "2023-01-01"

        # Reset mock
        mock_service.search.reset_mock()

        # Test author + source-repo combination
        result = runner.invoke(
            cli,
            [
                "search",
                "code",
                "test",
                "--author",
                "bob",
                "--source-repo",
                "github.com/example/repo",
            ],
        )
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.author == "bob"
        assert call_args.filters.source_repo == "github.com/example/repo"

        # Reset mock
        mock_service.search.reset_mock()

        # Test all filters together
        result = runner.invoke(
            cli,
            [
                "search",
                "code",
                "test",
                "--language",
                "go",
                "--author",
                "charlie",
                "--created-after",
                "2023-06-01",
                "--created-before",
                "2023-12-31",
                "--source-repo",
                "github.com/example/project",
            ],
        )
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert call_args.filters.language == "go"
        assert call_args.filters.author == "charlie"
        assert call_args.filters.created_after is not None
        assert call_args.filters.created_after.strftime("%Y-%m-%d") == "2023-06-01"
        assert call_args.filters.created_before is not None
        assert call_args.filters.created_before.strftime("%Y-%m-%d") == "2023-12-31"
        assert call_args.filters.source_repo == "github.com/example/project"


def test_search_invalid_date_format(runner: CliRunner) -> None:
    """Test that invalid date formats raise an error."""
    # Test with invalid date format
    result = runner.invoke(
        cli, ["search", "code", "test", "--created-after", "invalid-date"]
    )
    assert result.exit_code != 0
    assert result.exception is not None
    assert "Invalid date format for --created-after" in str(result.exception)
    assert "Expected ISO 8601 format (YYYY-MM-DD)" in str(result.exception)

    # Test with invalid created-before date format
    result = runner.invoke(
        cli, ["search", "code", "test", "--created-before", "not-a-date"]
    )
    assert result.exit_code != 0
    assert result.exception is not None
    assert "Invalid date format for --created-before" in str(result.exception)
    assert "Expected ISO 8601 format (YYYY-MM-DD)" in str(result.exception)


def test_search_filter_case_insensitivity(runner: CliRunner) -> None:
    """Test that language filters are case insensitive."""
    # Mock the search functionality
    mock_snippets = [MagicMock(id=1, content="test snippet")]
    mock_service = MagicMock()
    mock_service.search = AsyncMock(return_value=mock_snippets)

    with patch(
        "kodit.cli.create_code_indexing_application_service", return_value=mock_service
    ):
        # Test with uppercase language
        result = runner.invoke(cli, ["search", "code", "test", "--language", "PYTHON"])
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert (
            call_args.filters.language == "python"
        )  # Should be normalized to lowercase

        # Reset mock
        mock_service.search.reset_mock()

        # Test with mixed case language
        result = runner.invoke(
            cli, ["search", "code", "test", "--language", "JavaScript"]
        )
        assert result.exit_code == 0
        call_args = mock_service.search.call_args[0][0]
        assert (
            call_args.filters.language == "javascript"
        )  # Should be normalized to lowercase


def test_search_filter_help_text(runner: CliRunner) -> None:
    """Test that all filter options show up in help text."""
    # Test code search help
    result = runner.invoke(cli, ["search", "code", "--help"])
    assert result.exit_code == 0
    assert "--language TEXT" in result.output
    assert "--author TEXT" in result.output
    assert "--created-after TEXT" in result.output
    assert "--created-before TEXT" in result.output
    assert "--source-repo TEXT" in result.output

    # Test keyword search help
    result = runner.invoke(cli, ["search", "keyword", "--help"])
    assert result.exit_code == 0
    assert "--language TEXT" in result.output
    assert "--author TEXT" in result.output
    assert "--created-after TEXT" in result.output
    assert "--created-before TEXT" in result.output
    assert "--source-repo TEXT" in result.output

    # Test text search help
    result = runner.invoke(cli, ["search", "text", "--help"])
    assert result.exit_code == 0
    assert "--language TEXT" in result.output
    assert "--author TEXT" in result.output
    assert "--created-after TEXT" in result.output
    assert "--created-before TEXT" in result.output
    assert "--source-repo TEXT" in result.output

    # Test hybrid search help
    result = runner.invoke(cli, ["search", "hybrid", "--help"])
    assert result.exit_code == 0
    assert "--language TEXT" in result.output
    assert "--author TEXT" in result.output
    assert "--created-after TEXT" in result.output
    assert "--created-before TEXT" in result.output
    assert "--source-repo TEXT" in result.output
