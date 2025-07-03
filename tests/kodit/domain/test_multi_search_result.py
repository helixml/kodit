"""Test the MultiSearchResult value object."""

import json
from datetime import UTC, datetime

import pytest

from kodit.domain.value_objects import MultiSearchResult


@pytest.fixture
def sample_search_result() -> MultiSearchResult:
    """Create a sample MultiSearchResult for testing."""
    return MultiSearchResult(
        id=1,
        content="def hello_world():\n    print('Hello, World!')",
        original_scores=[0.95, 0.78],
        source_uri="https://github.com/example/repo",
        relative_path="src/hello.py",
        language="python",
        authors=["alice", "bob"],
        created_at=datetime(2023, 6, 15, 10, 30, 45, tzinfo=UTC),
        summary="A simple hello world function",
    )


@pytest.fixture
def sample_search_results(
    sample_search_result: MultiSearchResult,
) -> list[MultiSearchResult]:
    """Create a list of sample MultiSearchResult objects."""
    result1 = sample_search_result
    result2 = MultiSearchResult(
        id=2,
        content="function greetUser(name) {\n    console.log(`Hello, ${name}!`);\n}",
        original_scores=[0.87, 0.92],
        source_uri="https://github.com/example/frontend",
        relative_path="src/greet.js",
        language="javascript",
        authors=["charlie"],
        created_at=datetime(2023, 7, 20, 14, 15, 30, tzinfo=UTC),
        summary="A function to greet users by name",
    )
    return [result1, result2]


def test_multi_search_result_str_format(
    sample_search_result: MultiSearchResult,
) -> None:
    """Test the __str__ method output format."""
    result_str = str(sample_search_result)

    # Check that all expected fields are present in the output
    assert "id: 1" in result_str
    assert "source: https://github.com/example/repo" in result_str
    assert "path: src/hello.py" in result_str
    assert "lang: python" in result_str
    assert "created: 2023-06-15T10:30:45" in result_str
    assert "authors: alice, bob" in result_str
    assert "scores: [0.95, 0.78]" in result_str
    assert "A simple hello world function" in result_str
    assert "def hello_world():" in result_str
    assert "```python" in result_str
    assert "```" in result_str
    assert "---" in result_str


def test_multi_search_result_to_json(sample_search_result: MultiSearchResult) -> None:
    """Test the to_json method output format."""
    json_str = sample_search_result.to_json()

    # Parse the JSON to verify it's valid
    json_obj = json.loads(json_str)

    # Verify all expected fields are present
    assert json_obj["id"] == 1
    assert json_obj["source"] == "https://github.com/example/repo"
    assert json_obj["path"] == "src/hello.py"
    assert json_obj["lang"] == "python"
    assert json_obj["created"] == "2023-06-15T10:30:45+00:00"
    assert json_obj["author"] == "alice, bob"
    assert json_obj["score"] == [0.95, 0.78]
    assert json_obj["code"] == "def hello_world():\n    print('Hello, World!')"
    assert json_obj["summary"] == "A simple hello world function"


def test_multi_search_result_to_jsonlines(
    sample_search_results: list[MultiSearchResult],
) -> None:
    """Test the to_jsonlines class method."""
    jsonlines_str = MultiSearchResult.to_jsonlines(sample_search_results)

    # Split by newlines to get individual JSON objects
    lines = jsonlines_str.split("\n")
    assert len(lines) == 2

    # Parse each line as JSON
    json1 = json.loads(lines[0])
    json2 = json.loads(lines[1])

    # Verify first result
    assert json1["id"] == 1
    assert json1["source"] == "https://github.com/example/repo"
    assert json1["path"] == "src/hello.py"
    assert json1["lang"] == "python"
    assert json1["author"] == "alice, bob"

    # Verify second result
    assert json2["id"] == 2
    assert json2["source"] == "https://github.com/example/frontend"
    assert json2["path"] == "src/greet.js"
    assert json2["lang"] == "javascript"
    assert json2["author"] == "charlie"


def test_multi_search_result_to_string(
    sample_search_results: list[MultiSearchResult],
) -> None:
    """Test the to_string class method."""
    string_output = MultiSearchResult.to_string(sample_search_results)

    # Should contain both results separated by double newlines
    assert "id: 1" in string_output
    assert "id: 2" in string_output
    assert "src/hello.py" in string_output
    assert "src/greet.js" in string_output
    assert "```python" in string_output
    assert "```javascript" in string_output

    # Check that results are separated by double newlines
    parts = string_output.split("\n\n")
    assert len(parts) >= 2  # Should have at least 2 major parts


def test_empty_search_results() -> None:
    """Test handling of empty search results."""
    # Test empty list
    jsonlines_str = MultiSearchResult.to_jsonlines([])
    assert jsonlines_str == ""

    string_output = MultiSearchResult.to_string([])
    assert string_output == ""


def test_search_result_with_empty_authors() -> None:
    """Test MultiSearchResult with empty authors list."""
    result = MultiSearchResult(
        id=3,
        content="print('test')",
        original_scores=[0.5],
        source_uri="https://github.com/test/repo",
        relative_path="test.py",
        language="python",
        authors=[],
        created_at=datetime(2023, 1, 1, tzinfo=UTC),
        summary="Test snippet",
    )

    # Test string format
    result_str = str(result)
    assert "authors: " in result_str

    # Test JSON format
    json_obj = json.loads(result.to_json())
    assert json_obj["author"] == ""


def test_search_result_with_special_characters() -> None:
    """Test MultiSearchResult with special characters in content."""
    result = MultiSearchResult(
        id=4,
        content='print("Hello, "World"!")\n# Special chars: äöü, 中文, emoji 🚀',
        original_scores=[0.8],
        source_uri="https://github.com/test/unicode",
        relative_path="unicode_test.py",
        language="python",
        authors=["unicode_user"],
        created_at=datetime(2023, 1, 1, tzinfo=UTC),
        summary="Test with special characters",
    )

    # Should not raise exceptions
    result_str = str(result)
    assert "Special chars" in result_str

    json_str = result.to_json()
    json_obj = json.loads(json_str)
    assert "Special chars" in json_obj["code"]


def test_search_result_case_sensitivity() -> None:
    """Test that language is properly handled in JSON output."""
    result = MultiSearchResult(
        id=5,
        content="console.log('test');",
        original_scores=[0.7],
        source_uri="https://github.com/test/js",
        relative_path="test.js",
        language="JavaScript",  # Mixed case
        authors=["js_dev"],
        created_at=datetime(2023, 1, 1, tzinfo=UTC),
        summary="JavaScript test",
    )

    json_obj = json.loads(result.to_json())
    assert json_obj["lang"] == "javascript"  # Should be lowercase
