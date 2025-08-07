"""Test the API of the app with a real database."""

import shutil
from collections.abc import AsyncIterator, Callable
from contextlib import AbstractAsyncContextManager, asynccontextmanager
from datetime import UTC, datetime
from unittest.mock import patch

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from kodit.app import app
from kodit.config import AppContext
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.value_objects import FileProcessingStatus, SearchResult
from kodit.infrastructure.api.v1.schemas.context import AppLifespanState
from kodit.infrastructure.sqlalchemy.entities import (
    Author,
    AuthorFileMapping,
    File,
    Index,
    Snippet,
    Source,
    SourceType,
)

EXAMPLE_REPO_URI = "https://gist.github.com/philwinder/db2e17413332844fa4b14971ae5adb34"


@pytest.fixture
def test_lifespan(
    app_context: AppContext,
) -> Callable[[FastAPI], AbstractAsyncContextManager[AppLifespanState]]:
    """Create a test lifespan function."""

    @asynccontextmanager
    async def lifespan(_: FastAPI) -> AsyncIterator[AppLifespanState]:
        """Create a test lifespan function."""
        yield AppLifespanState(app_context=app_context)

    return lifespan


@pytest.fixture
def test_lifespan_with_api_keys(
    app_context: AppContext,
) -> Callable[[FastAPI], AbstractAsyncContextManager[AppLifespanState]]:
    """Create a test lifespan function."""

    @asynccontextmanager
    async def lifespan(_: FastAPI) -> AsyncIterator[AppLifespanState]:
        """Create a test lifespan function."""
        app_context.api_keys = ["test"]
        yield AppLifespanState(app_context=app_context)

    return lifespan


@pytest.fixture
def test_app() -> Callable[
    [Callable[[FastAPI], AbstractAsyncContextManager[AppLifespanState]]], FastAPI
]:
    """Create a test app."""

    def _test_app(
        lifespan: Callable[[FastAPI], AbstractAsyncContextManager[AppLifespanState]],
    ) -> FastAPI:
        app.router.lifespan_context = lifespan
        return app

    return _test_app


async def create_test_data(app_context: AppContext) -> dict:
    """Create test data in the database."""
    db = await app_context.get_db()
    async with db.session_factory() as session:
        # Create source
        source = Source(
            uri="https://github.com/test/repo",
            cloned_path="/tmp/test/repo",  # noqa: S108
            source_type=SourceType.GIT,
        )
        session.add(source)
        await session.flush()

        # Create index
        index = Index(source_id=source.id)
        session.add(index)
        await session.flush()

        # Create authors
        author1 = Author(name="Test Author", email="test@example.com")
        author2 = Author(name="Another Author", email="another@example.com")
        session.add_all([author1, author2])
        await session.flush()

        # Create files
        file1 = File(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source_id=source.id,
            mime_type="text/x-python",
            uri="file:///tmp/test/repo/app.js",
            cloned_path="/tmp/test/repo/main.py",  # noqa: S108
            sha256="abc123",
            size_bytes=1000,
            extension="py",
            file_processing_status=FileProcessingStatus.CLEAN,
        )

        file2 = File(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            source_id=source.id,
            mime_type="text/javascript",
            uri="file:///tmp/test/repo/app.js",
            cloned_path="/tmp/test/repo/app.js",  # noqa: S108
            sha256="def456",
            size_bytes=2000,
            extension="js",
            file_processing_status=FileProcessingStatus.CLEAN,
        )

        session.add_all([file1, file2])
        await session.flush()

        # Create author-file mappings
        mapping1 = AuthorFileMapping(author_id=author1.id, file_id=file1.id)
        mapping2 = AuthorFileMapping(author_id=author2.id, file_id=file2.id)
        session.add_all([mapping1, mapping2])
        await session.flush()

        # Create snippets
        snippet1 = Snippet(
            file_id=file1.id,
            index_id=index.id,
            content="def hello_world():\n    print('Hello, World!')",
            summary="A simple hello world function",
        )

        snippet2 = Snippet(
            file_id=file2.id,
            index_id=index.id,
            content="function greet(name) {\n    console.log(`Hello, ${name}!`);\n}",
            summary="A greeting function in JavaScript",
        )

        session.add_all([snippet1, snippet2])
        await session.commit()

        return {
            "source": source,
            "index": index,
            "files": [file1, file2],
            "authors": [author1, author2],
            "snippets": [snippet1, snippet2],
        }


@pytest.mark.asyncio
async def test_index(
    app_context: AppContext,
    test_app: Callable[
        [Callable[[FastAPI], AbstractAsyncContextManager[AppLifespanState]]], FastAPI
    ],
    test_lifespan: Callable[[FastAPI], AbstractAsyncContextManager[AppLifespanState]],
) -> None:
    """Test the full lifecycle of the index endpoint."""
    # Create test data
    test_data = await create_test_data(app_context)

    with TestClient(test_app(test_lifespan)) as client:
        # Test list indexes
        response = client.get(
            "/api/v1/indexes",
        )
        assert response.status_code == 200
        data = response.json()
        assert "data" in data
        assert len(data["data"]) == 1
        assert data["data"][0]["type"] == "index"
        assert data["data"][0]["id"] == str(test_data["index"].id)
        assert data["data"][0]["attributes"]["uri"] == test_data["source"].uri

        response = client.post(
            "/api/v1/indexes",
            json={
                "data": {
                    "type": "index",
                    "attributes": {
                        "uri": EXAMPLE_REPO_URI,
                    },
                }
            },
        )
        assert response.status_code == 202
        data = response.json()
        assert "data" in data
        assert data["data"]["type"] == "index"
        assert data["data"]["attributes"]["uri"] == EXAMPLE_REPO_URI

        # Test get specific index
        response = client.get(
            f"/api/v1/indexes/{test_data['index'].id}",
        )
        assert response.status_code == 200
        data = response.json()
        assert "data" in data
        assert data["data"]["type"] == "index"
        assert data["data"]["id"] == str(test_data["index"].id)

        # Test get non-existent index
        response = client.get(
            "/api/v1/indexes/999999",
        )
        assert response.status_code == 404

        # Test invalid index ID
        response = client.get(
            "/api/v1/indexes/invalid",
        )
        assert response.status_code == 422

        # Test delete index
        with patch.object(shutil, "rmtree"):
            response = client.delete(
                f"/api/v1/indexes/{test_data['index'].id}",
            )
            assert response.status_code == 204

        # Test delete non-existent index
        response = client.delete(
            "/api/v1/indexes/999999",
        )
        assert response.status_code == 404

        # Test get index after deletion
        response = client.get(
            f"/api/v1/indexes/{test_data['index'].id}",
        )
        assert response.status_code == 404


@pytest.mark.asyncio
async def test_index_with_api_keys(
    test_app: Callable[
        [Callable[[FastAPI], AbstractAsyncContextManager[AppLifespanState]]], FastAPI
    ],
    test_lifespan_with_api_keys: Callable[
        [FastAPI], AbstractAsyncContextManager[AppLifespanState]
    ],
) -> None:
    """Test the index endpoint with invalid API keys."""
    with TestClient(test_app(test_lifespan_with_api_keys)) as client:
        response = client.get(
            "/api/v1/indexes",
        )
        assert response.status_code == 401

        response = client.get(
            "/api/v1/indexes",
            headers={"X-API-KEY": "test"},
        )
        assert response.status_code == 200


@pytest.mark.asyncio
async def test_search(
    app_context: AppContext,
    test_app: Callable[
        [Callable[[FastAPI], AbstractAsyncContextManager[AppLifespanState]]], FastAPI
    ],
    test_lifespan_with_api_keys: Callable[
        [FastAPI], AbstractAsyncContextManager[AppLifespanState]
    ],
) -> None:
    """Test the search endpoint."""
    test_data = await create_test_data(app_context)

    with patch.object(BM25DomainService, "search") as mock_search_snippets:
        mock_search_snippets.return_value = [
            SearchResult(
                snippet_id=test_data["snippets"][0].id,
                score=1.0,
            )
        ]
        with TestClient(test_app(test_lifespan_with_api_keys)) as client:
            response = client.post(
                "/api/v1/search",
                json={
                    "data": {
                        "type": "search",
                        "attributes": {
                            "keywords": ["hello"],
                        },
                    },
                },
            )
            assert response.status_code == 200
            data = response.json()
            assert "data" in data
            assert len(data["data"]) == 1
            assert data["data"][0]["type"] == "snippet"
            assert data["data"][0]["id"] == test_data["snippets"][0].id
            assert (
                data["data"][0]["attributes"]["source_uri"]
                == "https://github.com/test/repo"
            )
            assert data["data"][0]["attributes"]["relative_path"] == "app.js"
