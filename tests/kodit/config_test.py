"""Tests for configuration classes."""

import os

from kodit.config import AppContext, AutoIndexingConfig, AutoIndexingSource


class TestAutoIndexingSource:
    """Test the AutoIndexingSource configuration class."""

    def test_auto_indexing_source_creation(self) -> None:
        """Test creating an AutoIndexingSource."""
        source = AutoIndexingSource(uri="https://github.com/test/repo")
        assert source.uri == "https://github.com/test/repo"


class TestAutoIndexingConfig:
    """Test the AutoIndexingConfig configuration class."""

    def test_auto_indexing_config_empty(self) -> None:
        """Test empty auto-indexing configuration."""
        config = AutoIndexingConfig()
        assert config.sources == []

    def test_auto_indexing_config_with_sources(self) -> None:
        """Test auto-indexing configuration with sources."""
        sources = [
            AutoIndexingSource(uri="https://github.com/test/repo1"),
            AutoIndexingSource(uri="https://github.com/test/repo2"),
        ]
        config = AutoIndexingConfig(sources=sources)
        assert len(config.sources) == 2
        assert config.sources[0].uri == "https://github.com/test/repo1"
        assert config.sources[1].uri == "https://github.com/test/repo2"


class TestAppContextAutoIndexing:
    """Test auto-indexing functionality in AppContext."""

    def test_get_auto_index_sources_empty(self) -> None:
        """Test getting auto-index sources when none are configured."""
        # Ensure no AUTO_INDEXING env vars are set
        env_vars_to_clean = [k for k in os.environ if k.startswith("AUTO_INDEXING")]
        saved_env_vars = {k: os.environ[k] for k in env_vars_to_clean}

        try:
            for k in env_vars_to_clean:
                del os.environ[k]

            app_context = AppContext()
            sources = (
                app_context.auto_indexing.sources if app_context.auto_indexing else []
            )
            assert sources == []
        finally:
            # Restore original env vars
            for k, v in saved_env_vars.items():
                os.environ[k] = v

    def test_get_auto_index_sources_with_config(self) -> None:
        """Test getting auto-index sources when configured."""
        auto_sources = [
            AutoIndexingSource(uri="https://github.com/test/repo1"),
            AutoIndexingSource(uri="/local/path/to/repo"),
        ]
        app_context = AppContext(auto_indexing=AutoIndexingConfig(sources=auto_sources))
        sources = app_context.auto_indexing.sources if app_context.auto_indexing else []
        assert sources == auto_sources

    def test_auto_indexing_from_environment_variables(self) -> None:
        """Test auto-indexing configuration from environment variables."""
        # Set environment variables for auto-indexing
        os.environ["AUTO_INDEXING_SOURCES_0_URI"] = "https://github.com/test/repo1"
        os.environ["AUTO_INDEXING_SOURCES_1_URI"] = "https://github.com/test/repo2"

        try:
            app_context = AppContext()
            sources = (
                app_context.auto_indexing.sources if app_context.auto_indexing else []
            )
            uris = [source.uri for source in sources]
            assert uris == [
                "https://github.com/test/repo1",
                "https://github.com/test/repo2",
            ]
        finally:
            # Clean up environment variables
            del os.environ["AUTO_INDEXING_SOURCES_0_URI"]
            del os.environ["AUTO_INDEXING_SOURCES_1_URI"]

    def test_endpoint_timeout_configuration(self) -> None:
        """Test endpoint timeout configuration from env vars."""
        # Set environment variables for different endpoint timeouts
        os.environ["EMBEDDING_ENDPOINT_TIMEOUT"] = "60.0"
        os.environ["ENRICHMENT_ENDPOINT_TIMEOUT"] = "90.0"

        try:
            # Create new context with env vars
            app_context = AppContext()

            # Verify timeout configurations
            assert app_context.embedding_endpoint is not None
            assert app_context.embedding_endpoint.timeout == 60.0

            assert app_context.enrichment_endpoint is not None
            assert app_context.enrichment_endpoint.timeout == 90.0

        finally:
            # Clean up environment variables
            if "DEFAULT_ENDPOINT_TIMEOUT" in os.environ:
                del os.environ["DEFAULT_ENDPOINT_TIMEOUT"]
            if "EMBEDDING_ENDPOINT_TIMEOUT" in os.environ:
                del os.environ["EMBEDDING_ENDPOINT_TIMEOUT"]
            if "ENRICHMENT_ENDPOINT_TIMEOUT" in os.environ:
                del os.environ["ENRICHMENT_ENDPOINT_TIMEOUT"]
