"""Tests for the git source factory module."""

import tempfile
from pathlib import Path
from unittest.mock import AsyncMock, patch

import git
import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Source
from kodit.domain.repositories import SourceRepository
from kodit.infrastructure.cloning.git.factory import GitSourceFactory
from kodit.infrastructure.cloning.git.working_copy import GitWorkingCopyProvider


@pytest.fixture
def mock_repository() -> AsyncMock:
    """Create a mock repository."""
    return AsyncMock(spec=SourceRepository)


@pytest.fixture
def mock_working_copy() -> AsyncMock:
    """Create a mock working copy provider."""
    return AsyncMock(spec=GitWorkingCopyProvider)


@pytest.fixture
def mock_session() -> AsyncMock:
    """Create a mock database session."""
    return AsyncMock(spec=AsyncSession)


@pytest.fixture
def factory(
    mock_repository: AsyncMock,
    mock_working_copy: AsyncMock,
    mock_session: AsyncMock,
) -> GitSourceFactory:
    """Create a GitSourceFactory instance with mocked dependencies."""
    return GitSourceFactory(
        repository=mock_repository,
        working_copy=mock_working_copy,
        session=mock_session,
    )


@pytest.mark.asyncio
async def test_url_normalization_with_pat_should_sanitize_credentials(
    factory: GitSourceFactory, tmp_path: Path
) -> None:
    """Test that URLs with personal access tokens are properly sanitized."""
    # URLs with PATs that should be sanitized
    pat_urls = [
        "https://phil:7lKCobJPAY1ekOS5kxxxxxxxx@dev.azure.com/winderai/private-test/_git/private-test",
        "https://winderai@dev.azure.com/winderai/private-test/_git/private-test",
        "https://username:token123@github.com/username/repo.git",
        "https://user:pass@gitlab.com/user/repo.git",
    ]

    expected_sanitized = [
        "https://dev.azure.com/winderai/private-test/_git/private-test",
        "https://dev.azure.com/winderai/private-test/_git/private-test",
        "https://github.com/username/repo.git",
        "https://gitlab.com/user/repo.git",
    ]

    for i, pat_url in enumerate(pat_urls):
        # Mock the repository to return None (source doesn't exist)
        factory.repository.get_by_uri = AsyncMock(return_value=None)

        # Mock the working copy to return a temporary path
        temp_clone_path = tmp_path / f"clone_{i}"
        temp_clone_path.mkdir()
        factory.working_copy.prepare = AsyncMock(return_value=temp_clone_path)

        # Mock the source creation
        def mock_save(source_arg: Source) -> Source:
            # Return the actual Source object that was passed in
            return source_arg

        factory.repository.save = AsyncMock(side_effect=mock_save)

        # Create a temporary git repo to simulate the clone
        with tempfile.TemporaryDirectory() as temp_dir:
            repo = git.Repo.init(temp_dir)

            # Mock the remote URL to return the PAT URL (this simulates the current bug)
            with patch.object(repo, "remote") as mock_remote:
                mock_remote.return_value.url = pat_url

                with patch("git.Repo.clone_from"), patch("git.Repo") as mock_git_repo:
                    mock_git_repo.return_value = repo

                    # Call the create method
                    result = await factory.create(pat_url)

                    # Verify that the URL was sanitized in repository.save
                    assert result.uri == expected_sanitized[i], (
                        f"URL should be sanitized: {pat_url} -> {expected_sanitized[i]}"
                    )

                    # Also verify that repository.save was called with the correct URI
                    save_call_args = factory.repository.save.call_args
                    assert save_call_args is not None, (
                        "repository.save should have been called"
                    )
                    saved_source = save_call_args[0][0]  # First positional argument
                    assert saved_source.uri == expected_sanitized[i], (
                        f"repository.save was called with wrong URI: {saved_source.uri}"
                    )


@pytest.mark.asyncio
async def test_url_normalization_without_credentials_should_remain_unchanged(
    factory: GitSourceFactory, tmp_path: Path
) -> None:
    """Test that URLs without credentials remain unchanged."""
    clean_urls = [
        "https://github.com/username/repo.git",
        "https://dev.azure.com/winderai/public-test/_git/public-test",
        "git@github.com:username/repo.git",
    ]

    for i, clean_url in enumerate(clean_urls):
        # Mock the repository to return None (source doesn't exist)
        factory.repository.get_by_uri = AsyncMock(return_value=None)

        # Mock the working copy to return a temporary path
        temp_clone_path = tmp_path / f"clone_{i}"
        temp_clone_path.mkdir()
        factory.working_copy.prepare = AsyncMock(return_value=temp_clone_path)

        # Mock the source creation
        def mock_save(source_arg: Source) -> Source:
            # Return the actual Source object that was passed in
            return source_arg

        factory.repository.save = AsyncMock(side_effect=mock_save)

        # Create a temporary git repo to simulate the clone
        with tempfile.TemporaryDirectory() as temp_dir:
            repo = git.Repo.init(temp_dir)

            # Mock the remote URL to return the clean URL
            with patch.object(repo, "remote") as mock_remote:
                mock_remote.return_value.url = clean_url

                with patch("git.Repo.clone_from"), patch("git.Repo") as mock_git_repo:
                    mock_git_repo.return_value = repo

                    # Call the create method
                    result = await factory.create(clean_url)

                    # Verify that the URL remains unchanged
                    assert result.uri == clean_url

                    # Also verify that repository.save was called with the correct URI
                    save_call_args = factory.repository.save.call_args
                    assert save_call_args is not None, (
                        "repository.save should have been called"
                    )
                    saved_source = save_call_args[0][0]  # First positional argument
                    assert saved_source.uri == clean_url, (
                        f"repository.save was called with wrong URI: {saved_source.uri}"
                    )


@pytest.mark.asyncio
async def test_url_normalization_ssh_urls_should_remain_unchanged(
    factory: GitSourceFactory, tmp_path: Path
) -> None:
    """Test that SSH URLs remain unchanged."""
    ssh_urls = [
        "git@github.com:username/repo.git",
        "ssh://git@github.com:2222/username/repo.git",
    ]

    for i, ssh_url in enumerate(ssh_urls):
        # Mock the repository to return None (source doesn't exist)
        factory.repository.get_by_uri = AsyncMock(return_value=None)

        # Mock the working copy to return a temporary path
        temp_clone_path = tmp_path / f"clone_{i}"
        temp_clone_path.mkdir()
        factory.working_copy.prepare = AsyncMock(return_value=temp_clone_path)

        # Mock the source creation
        def mock_save(source_arg: Source) -> Source:
            # Return the actual Source object that was passed in
            return source_arg

        factory.repository.save = AsyncMock(side_effect=mock_save)

        # Create a temporary git repo to simulate the clone
        with tempfile.TemporaryDirectory() as temp_dir:
            repo = git.Repo.init(temp_dir)

            # Mock the remote URL to return the SSH URL
            with patch.object(repo, "remote") as mock_remote:
                mock_remote.return_value.url = ssh_url

                with patch("git.Repo.clone_from"), patch("git.Repo") as mock_git_repo:
                    mock_git_repo.return_value = repo

                    # Call the create method
                    result = await factory.create(ssh_url)

                    # Verify that the SSH URL remains unchanged
                    assert result.uri == ssh_url

                    # Also verify that repository.save was called with the correct URI
                    save_call_args = factory.repository.save.call_args
                    assert save_call_args is not None, (
                        "repository.save should have been called"
                    )
                    saved_source = save_call_args[0][0]  # First positional argument
                    assert saved_source.uri == ssh_url, (
                        f"repository.save was called with wrong URI: {saved_source.uri}"
                    )
