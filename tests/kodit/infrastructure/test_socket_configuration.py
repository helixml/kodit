"""Test Unix socket configuration with the config system."""

import tempfile
from pathlib import Path

import pytest

from kodit.config import AppContext, Endpoint


@pytest.mark.asyncio
async def test_socket_path_configuration_via_endpoint() -> None:
    """Test that socket path configuration works through the Endpoint model."""
    with tempfile.TemporaryDirectory() as temp_dir:
        socket_path = str(Path(temp_dir) / "openai.sock")

        # Create endpoint configuration with socket path
        endpoint = Endpoint(
            type="openai",
            api_key="test-key",
            socket_path=socket_path,
            model="text-embedding-3-small",
            num_parallel_tasks=5,
        )

        # Verify endpoint configuration
        assert endpoint.socket_path == socket_path
        assert endpoint.api_key == "test-key"
        assert endpoint.model == "text-embedding-3-small"
        assert endpoint.num_parallel_tasks == 5


@pytest.mark.asyncio
async def test_socket_path_in_app_context() -> None:
    """Test socket path configuration in AppContext."""
    with tempfile.TemporaryDirectory() as temp_dir:
        embedding_socket = str(Path(temp_dir) / "embedding.sock")
        enrichment_socket = str(Path(temp_dir) / "enrichment.sock")

        # Create app context with socket paths
        app_context = AppContext(
            embedding_endpoint=Endpoint(
                type="openai",
                api_key="embed-key",
                socket_path=embedding_socket,
                model="text-embedding-3-small",
            ),
            enrichment_endpoint=Endpoint(
                type="openai",
                api_key="enrich-key",
                socket_path=enrichment_socket,
                model="gpt-4o-mini",
            ),
        )

        # Verify configurations
        assert app_context.embedding_endpoint
        assert app_context.embedding_endpoint.socket_path == embedding_socket
        assert app_context.enrichment_endpoint
        assert app_context.enrichment_endpoint.socket_path == enrichment_socket


@pytest.mark.asyncio
async def test_default_endpoint_with_socket() -> None:
    """Test using default endpoint with socket path."""
    with tempfile.TemporaryDirectory() as temp_dir:
        socket_path = str(Path(temp_dir) / "default.sock")

        # Create app context with default endpoint
        app_context = AppContext(
            default_endpoint=Endpoint(
                type="openai",
                api_key="default-key",
                socket_path=socket_path,
                model="text-embedding-3-small",
            )
        )

        # When no specific endpoints are set, default should be used
        assert app_context.default_endpoint
        assert app_context.default_endpoint.socket_path == socket_path
