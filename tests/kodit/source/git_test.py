from pathlib import Path
import pytest

from kodit.source.git import is_valid_clone_target


current_dir = Path(__file__).parent


@pytest.mark.parametrize(
    "uri, expected",
    [
        ("https://github.com/helixml/kodit.git", True),
        ("https://github.com/helixml/kodit", True),
        (f"file://{current_dir}/../../..", True),
        (f"{current_dir}/../../..", True),
        (f"file://{current_dir}", False),
    ],
)
def test_is_valid_clone_target(uri: str, expected: bool) -> None:
    assert is_valid_clone_target(uri) == expected
