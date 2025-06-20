"""Tests for the git working copy provider module."""

from pathlib import Path
from unittest.mock import patch

import git
import pytest

from kodit.infrastructure.cloning.git.working_copy import GitWorkingCopyProvider


@pytest.fixture
def working_copy(tmp_path: Path) -> GitWorkingCopyProvider:
    """Create a GitWorkingCopyProvider instance."""
    return GitWorkingCopyProvider(tmp_path)


@pytest.mark.asyncio
async def test_prepare_should_not_leak_credentials_in_directory_name(
    working_copy: GitWorkingCopyProvider, tmp_path: Path
) -> None:
    """Test that directory names don't contain sensitive credentials."""
    # URLs with PATs that should not appear in directory names
    pat_urls = [
        "https://phil:7lKCobJPAY1ekOS5kxxxxxxxx@dev.azure.com/winderai/private-test/_git/private-test",
        "https://winderai@dev.azure.com/winderai/private-test/_git/private-test",
        "https://username:token123@github.com/username/repo.git",
        "https://user:pass@gitlab.com/user/repo.git",
    ]

    expected_safe_directories = [
        "https___dev.azure.com_winderai_private-test__git_private-test",
        "https___dev.azure.com_winderai_private-test__git_private-test",
        "https___github.com_username_repo.git",
        "https___gitlab.com_user_repo.git",
    ]

    for i, pat_url in enumerate(pat_urls):
        # Mock git.Repo.clone_from to avoid actual cloning
        with patch("git.Repo.clone_from") as mock_clone:
            # Call the prepare method
            result_path = await working_copy.prepare(pat_url)

            # Verify that the directory name doesn't contain credentials
            directory_name = result_path.name
            assert directory_name == expected_safe_directories[i], (
                f"Directory name should not contain credentials: {directory_name}"
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
async def test_prepare_should_fail_when_directory_name_exceeds_windows_path_limit(
    working_copy: GitWorkingCopyProvider, tmp_path: Path
) -> None:
    """Test that prepare fails when the resulting directory name exceeds Windows 256 character path limit."""
    # Create a URL that, when sanitized and converted to directory name, will exceed 256 characters
    # This URL is designed to be extremely long to trigger the Windows path limit issue
    long_url = (
        "https://extremely-long-domain-name-that-will-definitely-exceed-windows-path-limits-and-cause-issues.com/"
        "very-long-organization-name-with-many-words-and-descriptive-text/"
        "very-long-project-name-with-additional-descriptive-text/"
        "_git/"
        "extremely-long-repository-name-with-many-subdirectories-and-deeply-nested-paths-that-cause-issues-on-windows-systems-and-this-is-just-the-beginning-of-the-very-long-name-that-continues-for-many-more-characters-to-ensure-we-hit-the-limit"
    )

    # Mock git.Repo.clone_from to avoid actual cloning
    with patch("git.Repo.clone_from") as mock_clone:
        # Call the prepare method
        result_path = await working_copy.prepare(long_url)

        # Get the directory name that would be created
        directory_name = result_path.name

        # Print the actual directory name and its length for debugging
        print(f"Directory name: {directory_name}")
        print(f"Directory name length: {len(directory_name)}")

        # This test should FAIL because the directory name exceeds 256 characters
        # The directory name is created by replacing "/" and ":" with "_" in the sanitized URL
        # Windows has a 256 character path limit, so this should cause issues
        assert len(directory_name) <= 256, (
            f"Directory name exceeds Windows 256 character path limit: "
            f"{len(directory_name)} characters: {directory_name}"
        )


@pytest.mark.asyncio
async def test_prepare_clean_urls_should_work_normally(
    working_copy: GitWorkingCopyProvider, tmp_path: Path
) -> None:
    """Test that clean URLs work normally without any issues."""
    clean_urls = [
        "https://github.com/username/repo.git",
        "https://dev.azure.com/winderai/public-test/_git/public-test",
        "git@github.com:username/repo.git",
    ]

    expected_directories = [
        "https___github.com_username_repo.git",
        "https___dev.azure.com_winderai_public-test__git_public-test",
        "git@github.com_username_repo.git",
    ]

    for i, clean_url in enumerate(clean_urls):
        # Mock git.Repo.clone_from to avoid actual cloning
        with patch("git.Repo.clone_from") as mock_clone:
            # Call the prepare method
            result_path = await working_copy.prepare(clean_url)

            # Verify that the directory name is as expected
            directory_name = result_path.name
            assert directory_name == expected_directories[i], (
                f"Directory name should match expected: {directory_name}"
            )

            # Verify that the directory was created
            assert result_path.exists()
            assert result_path.is_dir()


@pytest.mark.asyncio
async def test_prepare_ssh_urls_should_work_normally(
    working_copy: GitWorkingCopyProvider, tmp_path: Path
) -> None:
    """Test that SSH URLs work normally."""
    ssh_urls = [
        "git@github.com:username/repo.git",
        "ssh://git@github.com:2222/username/repo.git",
    ]

    expected_directories = [
        "git@github.com_username_repo.git",
        "ssh___git@github.com_2222_username_repo.git",
    ]

    for i, ssh_url in enumerate(ssh_urls):
        # Mock git.Repo.clone_from to avoid actual cloning
        with patch("git.Repo.clone_from") as mock_clone:
            # Call the prepare method
            result_path = await working_copy.prepare(ssh_url)

            # Verify that the directory name is as expected
            directory_name = result_path.name
            assert directory_name == expected_directories[i], (
                f"Directory name should match expected: {directory_name}"
            )

            # Verify that the directory was created
            assert result_path.exists()
            assert result_path.is_dir()


@pytest.mark.asyncio
async def test_prepare_handles_clone_errors_gracefully(
    working_copy: GitWorkingCopyProvider, tmp_path: Path
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
    working_copy: GitWorkingCopyProvider, tmp_path: Path
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
