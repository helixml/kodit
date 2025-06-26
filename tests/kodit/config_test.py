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
        app_context = AppContext()
        sources = app_context.auto_indexing.sources
        assert sources == []

    def test_get_auto_index_sources_with_config(self) -> None:
        """Test getting auto-index sources when configured."""
        auto_sources = [
            AutoIndexingSource(uri="https://github.com/test/repo1"),
            AutoIndexingSource(uri="/local/path/to/repo"),
        ]
        app_context = AppContext(auto_indexing=AutoIndexingConfig(sources=auto_sources))
        sources = app_context.auto_indexing.sources
        assert sources == auto_sources

    def test_auto_indexing_from_environment_variables(self) -> None:
        """Test auto-indexing configuration from environment variables."""
        # Set environment variables for auto-indexing
        os.environ["AUTO_INDEXING_SOURCES_0_URI"] = "https://github.com/test/repo1"
        os.environ["AUTO_INDEXING_SOURCES_1_URI"] = "https://github.com/test/repo2"

        try:
            app_context = AppContext()
            sources = app_context.auto_indexing.sources
            uris = [source.uri for source in sources]
            assert uris == [
                "https://github.com/test/repo1",
                "https://github.com/test/repo2",
            ]
        finally:
            # Clean up environment variables
            del os.environ["AUTO_INDEXING_SOURCES_0_URI"]
            del os.environ["AUTO_INDEXING_SOURCES_1_URI"]
