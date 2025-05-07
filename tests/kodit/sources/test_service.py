"""Tests for the source service module."""

from datetime import datetime, timedelta
from pathlib import Path

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.sources.repository import SourceRepository
from kodit.sources.service import SourceService


@pytest.fixture
def repository(session: AsyncSession) -> SourceRepository:
    """Create a repository instance with a real database session."""
    return SourceRepository(session)


@pytest.fixture
def service(repository: SourceRepository) -> SourceService:
    """Create a service instance with a real repository."""
    return SourceService(repository)


async def test_create_folder_source(
    service: SourceService, repository: SourceRepository, tmp_path: Path
):
    """Test creating a folder source through the service."""
    # Create a temporary directory for testing
    test_dir = tmp_path / "test_folder"
    test_dir.mkdir()

    await service.create(str(test_dir))

    # Verify the source was created
    sources = await service.list_sources()
    assert len(sources) == 1
    assert sources[0].uri == str(test_dir)


@pytest.mark.asyncio
async def test_create_git_source(service: SourceService, repository: SourceRepository):
    """Test creating a git source through the service."""
    await service.create("https://github.com/user/repo.git")

    # Verify the source was created
    sources = await service.list_sources()
    assert len(sources) == 1
    assert sources[0].uri == "https://github.com/user/repo.git"


@pytest.mark.asyncio
async def test_list_sources(service: SourceService, tmp_path: Path) -> None:
    """Test listing all sources through the service."""
    # Create a temporary directory for testing
    test_dir = tmp_path / "test_folder"
    test_dir.mkdir()

    # Create a folder source
    await service.create(str(test_dir))

    # List sources
    sources = await service.list_sources()

    assert len(sources) == 1
    assert sources[0].id == 1
    assert sources[0].created_at - datetime.now() < timedelta(seconds=1)
    assert sources[0].uri.endswith("test_folder")
