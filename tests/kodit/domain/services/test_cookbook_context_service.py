"""Tests for CookbookContextService."""

import tempfile
from collections.abc import Generator
from pathlib import Path
from typing import Any
from unittest.mock import MagicMock

import pytest

from kodit.domain.services.cookbook_context_service import CookbookContextService


@pytest.fixture
def temp_repo() -> Generator[Path, None, None]:
    """Create a temporary repository directory with basic files."""
    with tempfile.TemporaryDirectory() as tmp:
        repo_path = Path(tmp)

        # Create README
        (repo_path / "README.md").write_text(
            "# Test Library\nA simple testing library for demos."
        )

        # Create package.json
        (repo_path / "package.json").write_text(
            '{"name": "test-lib", "version": "1.0.0"}'
        )

        yield repo_path


@pytest.fixture
def sample_api_modules() -> list[Any]:
    """Create sample API module definitions using mocks."""
    # Create mock function
    func = MagicMock()
    func.simple_name = "create_client"
    func.docstring = "Create a new client instance"

    # Create mock method
    method = MagicMock()
    method.simple_name = "connect"
    method.docstring = "Connect to server"

    # Create mock class
    cls = MagicMock()
    cls.simple_name = "Client"
    cls.docstring = "Main client class"
    cls.methods = [method]

    # Create mock module
    module = MagicMock()
    module.module_path = "test_lib.core"
    module.module_docstring = "Core functionality"
    module.functions = [func]
    module.classes = [cls]
    module.types = []

    return [module]


@pytest.mark.asyncio
async def test_gather_context_includes_all_sections(
    temp_repo: Path, sample_api_modules: list[Any]
) -> None:
    """Test that gather_context includes all major sections."""
    service = CookbookContextService()

    context = await service.gather_context(
        temp_repo, language="javascript", api_modules=sample_api_modules
    )

    # Verify all expected sections are present
    assert "## Primary Language" in context
    assert "javascript" in context
    assert "## README" in context
    assert "Test Library" in context
    assert "## API Structure" in context
    assert "test_lib.core" in context
    assert "## Package Information" in context
    assert "package.json" in context


@pytest.mark.asyncio
async def test_gather_context_without_api_modules(temp_repo: Path) -> None:
    """Test that gather_context works without API modules."""
    service = CookbookContextService()

    context = await service.gather_context(temp_repo, language="python")

    # Should still have basic sections
    assert "## Primary Language" in context
    assert "python" in context
    assert "## README" in context
    # Should not have API structure section
    assert "## API Structure" not in context


@pytest.mark.asyncio
async def test_format_api_structure_includes_classes_and_functions(
    sample_api_modules: list[Any],
) -> None:
    """Test that API structure formatting includes key elements."""
    service = CookbookContextService()

    formatted = service._format_api_structure(sample_api_modules)  # noqa: SLF001

    # Verify module is listed
    assert "test_lib.core" in formatted
    # Verify class is listed
    assert "Client" in formatted
    # Verify function is listed
    assert "create_client" in formatted
    # Verify method is listed
    assert "connect" in formatted


@pytest.mark.asyncio
async def test_format_api_structure_with_empty_modules() -> None:
    """Test that empty modules list returns empty string."""
    service = CookbookContextService()

    formatted = service._format_api_structure([])  # noqa: SLF001

    assert formatted == ""
