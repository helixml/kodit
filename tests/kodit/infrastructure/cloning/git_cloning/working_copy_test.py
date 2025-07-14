"""Tests for the git working copy provider module."""

import hashlib
from pathlib import Path
from unittest.mock import MagicMock, patch

import git
import pytest

from kodit.domain.entities import WorkingCopy
from kodit.infrastructure.cloning.git.working_copy import GitWorkingCopyProvider


@pytest.fixture
def working_copy(tmp_path: Path) -> GitWorkingCopyProvider:
    """Create a GitWorkingCopyProvider instance."""
    return GitWorkingCopyProvider(tmp_path)


def get_expected_directory_name(uri: str) -> str:
    """Get the expected directory name for a given URI."""
    sanitized_uri = WorkingCopy.sanitize_git_url(uri)
    dir_hash = hashlib.sha256(str(sanitized_uri).encode("utf-8")).hexdigest()[:16]
    return f"repo-{dir_hash}"


@pytest.mark.asyncio
async def test_prepare_should_not_leak_credentials_in_directory_name(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that directory names don't contain sensitive credentials."""
    # URLs with PATs that should not appear in directory names
    pat_urls = [
        "https://phil:7lKCobJPAY1ekOS5kxxxxxxxx@dev.azure.com/winderai/private-test/_git/private-test",
        "https://winderai@dev.azure.com/winderai/private-test/_git/private-test",
        "https://username:token123@github.com/username/repo.git",
        "https://user:pass@gitlab.com/user/repo.git",
    ]

    for pat_url in pat_urls:
        # Mock git.Repo.clone_from to avoid actual cloning
        with patch("git.Repo.clone_from"):
            # Call the prepare method
            result_path = await working_copy.prepare(pat_url)

            # Verify that the directory name doesn't contain credentials
            directory_name = result_path.name
            expected_name = get_expected_directory_name(pat_url)
            assert directory_name == expected_name, (
                f"Directory name should match expected hash: {directory_name}"
            )

            # Verify that the directory name doesn't contain the PAT/token
            assert "7lKCobJPAY1ekOS5kxxxxxxxx" not in directory_name, (
                f"Directory name contains PAT: {directory_name}"
            )
            assert "token123" not in directory_name, (
                f"Directory name contains token: {directory_name}"
            )
            assert "pass" not in directory_name, (
                f"Directory name contains password: {directory_name}"
            )

            # Verify that the directory was created
            assert result_path.exists()
            assert result_path.is_dir()


@pytest.mark.asyncio
async def test_prepare_should_not_exceed_windows_path_limit(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that directory names never exceed Windows 256 character path limit."""
    # Create a URL that, when sanitized, exceeds 256 characters
    # This URL is designed to be extremely long to trigger the Windows path limit issue
    long_url = (
        "https://extremely-long-domain-name-that-will-definitely-exceed-windows-path-limits-and-cause-issues.com/"
        "very-long-organization-name-with-many-words-and-descriptive-text/"
        "very-long-project-name-with-additional-descriptive-text/"
        "_git/"
        "extremely-long-repository-name-with-many-subdirectories-and-deeply-nested-paths-that-cause-issues-on-windows-systems-and-this-is-just-the-beginning-of-the-very-long-name-that-continues-for-many-more-characters-to-ensure-we-hit-the-limit"
    )

    # Mock git.Repo.clone_from to avoid actual cloning
    with patch("git.Repo.clone_from"):
        # Call the prepare method
        result_path = await working_copy.prepare(long_url)

        # Get the directory name that would be created
        directory_name = result_path.name

        # Print the actual directory name and its length for debugging

        # This test should PASS because the directory name is now a short hash
        # The directory should be in format "repo-<16-char-hash>" (21 characters total)
        assert len(directory_name) <= 256, (
            f"Directory name exceeds Windows 256 character path limit: "
            f"{len(directory_name)} characters: {directory_name}"
        )
        assert directory_name.startswith("repo-"), (
            f"Directory name should start with 'repo-': {directory_name}"
        )
        assert len(directory_name) == 21, (
            f"Directory name should be exactly 21 characters: {directory_name}"
        )


@pytest.mark.asyncio
async def test_prepare_clean_urls_should_work_normally(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that clean URLs work normally without any issues."""
    clean_urls = [
        "https://github.com/username/repo.git",
        "https://dev.azure.com/winderai/public-test/_git/public-test",
        "git@github.com:username/repo.git",
    ]

    for clean_url in clean_urls:
        # Mock git.Repo.clone_from to avoid actual cloning
        with patch("git.Repo.clone_from"):
            # Call the prepare method
            result_path = await working_copy.prepare(clean_url)

            # Verify that the directory name is as expected
            directory_name = result_path.name
            expected_name = get_expected_directory_name(clean_url)
            assert directory_name == expected_name, (
                f"Directory name should match expected hash: {directory_name}"
            )

            # Verify that the directory was created
            assert result_path.exists()
            assert result_path.is_dir()


@pytest.mark.asyncio
async def test_prepare_ssh_urls_should_work_normally(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that SSH URLs work normally."""
    ssh_urls = [
        "git@github.com:username/repo.git",
        "ssh://git@github.com:2222/username/repo.git",
    ]

    for ssh_url in ssh_urls:
        # Mock git.Repo.clone_from to avoid actual cloning
        with patch("git.Repo.clone_from"):
            # Call the prepare method
            result_path = await working_copy.prepare(ssh_url)

            # Verify that the directory name is as expected
            directory_name = result_path.name
            expected_name = get_expected_directory_name(ssh_url)
            assert directory_name == expected_name, (
                f"Directory name should match expected hash: {directory_name}"
            )

            # Verify that the directory was created
            assert result_path.exists()
            assert result_path.is_dir()


@pytest.mark.asyncio
async def test_prepare_handles_clone_errors_gracefully(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that clone errors are handled gracefully."""
    url = "https://github.com/username/repo.git"

    # Mock git.Repo.clone_from to raise an error
    with patch("git.Repo.clone_from") as mock_clone:
        mock_clone.side_effect = git.GitCommandError(
            "git", "clone", "Repository not found"
        )

        # Should raise ValueError for clone errors
        with pytest.raises(ValueError, match="Failed to clone repository"):
            await working_copy.prepare(url)


@pytest.mark.asyncio
async def test_prepare_handles_already_exists_error(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that 'already exists' errors are handled gracefully."""
    url = "https://github.com/username/repo.git"

    # Mock git.Repo.clone_from to raise an "already exists" error
    with patch("git.Repo.clone_from") as mock_clone:
        mock_clone.side_effect = git.GitCommandError(
            "git", "clone", "already exists and is not an empty directory"
        )

        # Should not raise an error for "already exists"
        result_path = await working_copy.prepare(url)

        # Verify that the directory was created
        assert result_path.exists()
        assert result_path.is_dir()


@pytest.mark.asyncio
async def test_sync_directory_does_not_exist_should_call_prepare(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that sync() calls prepare() when directory doesn't exist."""
    url = "https://github.com/username/repo.git"

    # Mock prepare method to avoid actual cloning
    with patch.object(working_copy, "prepare") as mock_prepare:
        mock_prepare.return_value = Path("/fake/path")

        # Call sync method
        result_path = await working_copy.sync(url)

        # Verify prepare was called
        mock_prepare.assert_called_once_with(url)
        assert result_path == Path("/fake/path")


@pytest.mark.asyncio
async def test_sync_directory_exists_but_no_git_should_call_prepare(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that sync() calls prepare() when directory exists but is not a Git repo."""
    url = "https://github.com/username/repo.git"

    # Create a directory that exists but is not a git repo
    clone_path = working_copy.get_clone_path(url)
    clone_path.mkdir(parents=True, exist_ok=True)

    # Mock prepare method to avoid actual cloning
    with patch.object(working_copy, "prepare") as mock_prepare:
        mock_prepare.return_value = clone_path

        # Call sync method
        result_path = await working_copy.sync(url)

        # Verify prepare was called
        mock_prepare.assert_called_once_with(url)
        assert result_path == clone_path


@pytest.mark.asyncio
async def test_sync_valid_git_repository_should_pull(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that sync() pulls from origin when repository is valid."""
    url = "https://github.com/username/repo.git"

    # Create a directory that exists with .git subdirectory
    clone_path = working_copy.get_clone_path(url)
    clone_path.mkdir(parents=True, exist_ok=True)
    (clone_path / ".git").mkdir(parents=True, exist_ok=True)

    # Mock git.Repo and origin.pull
    mock_repo = MagicMock()
    mock_origin = MagicMock()
    mock_repo.remotes.origin = mock_origin

    with patch("git.Repo") as mock_git_repo:
        mock_git_repo.return_value = mock_repo

        # Call sync method
        result_path = await working_copy.sync(url)

        # Verify git.Repo was called with the correct path
        mock_git_repo.assert_called_once_with(clone_path)

        # Verify origin.pull was called
        mock_origin.pull.assert_called_once()

        # Verify the correct path was returned
        assert result_path == clone_path


@pytest.mark.asyncio
async def test_sync_invalid_git_repository_should_reclone(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that sync() re-clones when repository is invalid."""
    url = "https://github.com/username/repo.git"

    # Create a directory that exists with .git subdirectory
    clone_path = working_copy.get_clone_path(url)
    clone_path.mkdir(parents=True, exist_ok=True)
    (clone_path / ".git").mkdir(parents=True, exist_ok=True)

    # Mock git.Repo to raise InvalidGitRepositoryError
    with patch("git.Repo") as mock_git_repo:
        mock_git_repo.side_effect = git.InvalidGitRepositoryError("Invalid repo")

        # Mock shutil.rmtree to track directory removal
        with (
            patch("shutil.rmtree") as mock_rmtree,
            patch.object(working_copy, "prepare") as mock_prepare,
        ):
            mock_prepare.return_value = clone_path

            # Call sync method
            result_path = await working_copy.sync(url)

            # Verify git.Repo was called with the correct path
            mock_git_repo.assert_called_once_with(clone_path)

            # Verify the invalid directory was removed
            mock_rmtree.assert_called_once_with(clone_path)

            # Verify prepare was called to re-clone
            mock_prepare.assert_called_once_with(url)

            # Verify the correct path was returned
            assert result_path == clone_path


@pytest.mark.asyncio
async def test_sync_get_clone_path_should_match_prepare(
    working_copy: GitWorkingCopyProvider,
) -> None:
    """Test that sync() and prepare() use the same clone path."""
    url = "https://github.com/username/repo.git"

    # Get clone paths from both methods
    sync_path = working_copy.get_clone_path(url)

    # Mock prepare to return the expected path
    with patch.object(working_copy, "prepare") as mock_prepare:
        mock_prepare.return_value = sync_path

        # Call sync method (will call prepare since directory doesn't exist)
        result_path = await working_copy.sync(url)

        # Verify paths match
        assert result_path == sync_path
        mock_prepare.assert_called_once_with(url)
