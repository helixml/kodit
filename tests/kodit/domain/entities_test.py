"""Tests for the WorkingCopy class."""

import pytest
from pydantic import AnyUrl

from kodit.domain.entities import WorkingCopy


def test_sanitize_git_url_with_username_password() -> None:
    """Test sanitizing URLs with username and password."""
    url = "https://phil:7lKCobJPAY1ekOS5kxxxxxxxx@dev.azure.com/winderai/private-test/_git/private-test"
    expected = "https://dev.azure.com/winderai/private-test/_git/private-test"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(expected)


def test_sanitize_git_url_with_username_only() -> None:
    """Test sanitizing URLs with username only."""
    url = "https://winderai@dev.azure.com/winderai/private-test/_git/private-test"
    expected = "https://dev.azure.com/winderai/private-test/_git/private-test"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(expected)


def test_sanitize_git_url_with_github_pat() -> None:
    """Test sanitizing GitHub URLs with personal access tokens."""
    url = "https://username:token123@github.com/username/repo.git"
    expected = "https://github.com/username/repo.git"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(expected)


def test_sanitize_git_url_with_gitlab_credentials() -> None:
    """Test sanitizing GitLab URLs with credentials."""
    url = "https://user:pass@gitlab.com/user/repo.git"
    expected = "https://gitlab.com/user/repo.git"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(expected)


def test_sanitize_git_url_with_port() -> None:
    """Test sanitizing URLs with port numbers."""
    url = "https://user:pass@github.com:443/username/repo.git"
    expected = "https://github.com:443/username/repo.git"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(expected)


def test_sanitize_git_url_with_query_params() -> None:
    """Test sanitizing URLs with query parameters."""
    url = "https://user:pass@github.com/username/repo.git?ref=main"
    expected = "https://github.com/username/repo.git?ref=main"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(expected)


def test_sanitize_git_url_with_fragment() -> None:
    """Test sanitizing URLs with fragments."""
    url = "https://user:pass@github.com/username/repo.git#main"
    expected = "https://github.com/username/repo.git#main"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(expected)


def test_sanitize_git_url_with_path_params() -> None:
    """Test sanitizing URLs with path parameters."""
    url = "https://user:pass@github.com/username/repo.git;param=value"
    expected = "https://github.com/username/repo.git;param=value"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(expected)


def test_sanitize_git_url_clean_urls_unchanged() -> None:
    """Test that clean URLs without credentials remain unchanged."""
    clean_urls = [
        "https://github.com/username/repo.git",
        "https://dev.azure.com/winderai/public-test/_git/public-test",
        "https://gitlab.com/user/repo.git",
    ]

    for url in clean_urls:
        assert WorkingCopy.sanitize_git_url(url) == AnyUrl(url)


def test_sanitize_git_url_ssh_urls_unchanged() -> None:
    """Test that SSH URLs remain unchanged."""
    ssh_urls = [
        "ssh://git@github.com:2222/username/repo.git",
        "ssh://user@host.com:22/path/to/repo.git",
    ]

    for url in ssh_urls:
        assert WorkingCopy.sanitize_git_url(url) == AnyUrl(url)


def test_sanitize_raw_ssh_url() -> None:
    """Test sanitizing URLs with path parameters."""
    url = "git@github.com:username/repo.git"
    expected = "ssh://git@github.com/username/repo.git"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(expected)


def test_sanitize_git_url_file_urls_unchanged() -> None:
    """Test that file URLs remain unchanged."""
    file_urls = [
        "file:///path/to/repo.git",
        "file://localhost/path/to/repo.git",
    ]

    for url in file_urls:
        assert WorkingCopy.sanitize_git_url(url) == AnyUrl(url)


def test_sanitize_git_url_invalid_urls() -> None:
    """Test that invalid URLs are handled gracefully."""
    invalid_urls = [
        "not-a-url",
        "",
    ]

    for url in invalid_urls:
        with pytest.raises(Exception):  # noqa: B017,PT011
            WorkingCopy.sanitize_git_url(url)


def test_sanitize_git_url_edge_cases() -> None:
    """Test edge cases for URL sanitization."""
    # URL with @ in the path (should not be confused with credentials)
    url = "https://github.com/username/repo@main.git"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(url)

    # URL with multiple @ symbols
    url = "https://user:pass@host.com/path@with@ats"
    expected = "https://host.com/path@with@ats"
    assert WorkingCopy.sanitize_git_url(url) == AnyUrl(expected)
