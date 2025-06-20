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
