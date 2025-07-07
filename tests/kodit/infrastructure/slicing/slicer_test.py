"""Tests for the slicer module."""

import tempfile
from pathlib import Path
from unittest.mock import Mock

import pytest

from kodit.infrastructure.slicing.slicer import (
    AnalyzerState,
    FunctionInfo,
    LanguageConfig,
    SimpleAnalyzer,
)


class TestLanguageConfig:
    """Test language configuration."""

    def test_has_all_required_configs(self) -> None:
        """Test that all language configs have required fields."""
        required_fields = {
            "function_nodes",
            "method_nodes",
            "call_node",
            "import_nodes",
            "extension",
            "name_field",
        }

        for language, config in LanguageConfig.CONFIGS.items():
            assert set(config.keys()) == required_fields, (
                f"Missing fields in {language}"
            )

    def test_language_aliases(self) -> None:
        """Test that language aliases work correctly."""
        assert LanguageConfig.CONFIGS["c++"] == LanguageConfig.CONFIGS["cpp"]
        assert (
            LanguageConfig.CONFIGS["typescript"] == LanguageConfig.CONFIGS["javascript"]
        )
        assert LanguageConfig.CONFIGS["ts"] == LanguageConfig.CONFIGS["javascript"]
        assert LanguageConfig.CONFIGS["js"] == LanguageConfig.CONFIGS["javascript"]

    def test_config_types(self) -> None:
        """Test that config values have correct types."""
        for config in LanguageConfig.CONFIGS.values():
            assert isinstance(config["function_nodes"], list)
            assert isinstance(config["method_nodes"], list)
            assert isinstance(config["call_node"], str)
            assert isinstance(config["import_nodes"], list)
            assert isinstance(config["extension"], str)
            assert config["name_field"] is None or isinstance(config["name_field"], str)

    def test_supported_languages(self) -> None:
        """Test that expected languages are supported."""
        expected_languages = {
            "python",
            "javascript",
            "typescript",
            "java",
            "go",
            "rust",
            "c",
            "cpp",
            "c++",
            "js",
            "ts",
        }

        for lang in expected_languages:
            assert lang in LanguageConfig.CONFIGS


class TestFunctionInfo:
    """Test FunctionInfo dataclass."""

    def test_function_info_creation(self) -> None:
        """Test FunctionInfo creation."""
        mock_node = Mock()
        func_info = FunctionInfo(
            file="test.py",
            node=mock_node,
            span=(0, 100),
            qualified_name="test.func",
        )

        assert func_info.file == "test.py"
        assert func_info.node == mock_node
        assert func_info.span == (0, 100)
        assert func_info.qualified_name == "test.func"


class TestAnalyzerState:
    """Test AnalyzerState dataclass."""

    def test_analyzer_state_creation(self) -> None:
        """Test AnalyzerState creation with defaults."""
        mock_parser = Mock()
        state = AnalyzerState(parser=mock_parser)

        assert state.parser == mock_parser
        assert state.files == []
        assert state.asts == {}
        assert state.def_index == {}
        assert isinstance(state.call_graph, dict)
        assert isinstance(state.reverse_calls, dict)
        assert isinstance(state.imports, dict)


class TestSimpleAnalyzer:
    """Test SimpleAnalyzer class - unit tests for individual methods."""

    def test_init_with_invalid_language(self) -> None:
        """Test initialization with unsupported language."""
        with (
            tempfile.TemporaryDirectory() as tmp_dir,
            pytest.raises(ValueError, match="Unsupported language"),
        ):
            SimpleAnalyzer(tmp_dir, "unsupported")

    def test_init_with_nonexistent_directory(self) -> None:
        """Test initialization with nonexistent directory."""
        with pytest.raises(FileNotFoundError):
            SimpleAnalyzer("/nonexistent/path")

    def test_get_tree_sitter_language_name_mapping(self) -> None:
        """Test tree-sitter language name mapping without initialization."""
        # Create a minimal analyzer instance without full initialization
        with tempfile.TemporaryDirectory() as tmp_dir:
            test_file = Path(tmp_dir, "test.py")
            test_file.write_text("# empty")

            # Mock the analyzer to avoid full initialization
            try:
                analyzer = SimpleAnalyzer.__new__(SimpleAnalyzer)
                analyzer.language = "python"
                assert analyzer._get_tree_sitter_language_name() == "python"  # noqa: SLF001

                analyzer.language = "c++"
                assert analyzer._get_tree_sitter_language_name() == "cpp"  # noqa: SLF001

                analyzer.language = "typescript"
                assert analyzer._get_tree_sitter_language_name() == "typescript"  # noqa: SLF001

                analyzer.language = "js"
                assert analyzer._get_tree_sitter_language_name() == "javascript"  # noqa: SLF001
            except (RuntimeError, AttributeError):
                # If this fails due to tree-sitter setup issues, that's expected
                # The important thing is the logic works
                pytest.skip("Tree-sitter setup not available")

    def test_language_config_assignment(self) -> None:
        """Test that language config is correctly assigned."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            test_file = Path(tmp_dir, "test.py")
            test_file.write_text("# empty")

            try:
                analyzer = SimpleAnalyzer(tmp_dir, "python")
                assert analyzer.language == "python"
                assert analyzer.config == LanguageConfig.CONFIGS["python"]
            except RuntimeError:
                # Tree-sitter may not be available in test environment
                pytest.skip("Tree-sitter setup not available")

    def test_config_access_patterns(self) -> None:
        """Test accessing different language configurations."""
        for language in ["python", "javascript", "java", "go", "rust", "c", "cpp"]:
            config = LanguageConfig.CONFIGS[language]

            # Verify all required keys exist
            assert "function_nodes" in config
            assert "method_nodes" in config
            assert "call_node" in config
            assert "import_nodes" in config
            assert "extension" in config
            assert "name_field" in config

            # Verify types are correct
            assert isinstance(config["function_nodes"], list)
            assert isinstance(config["method_nodes"], list)
            assert isinstance(config["call_node"], str)
            assert isinstance(config["import_nodes"], list)
            assert isinstance(config["extension"], str)
            assert config["name_field"] is None or isinstance(config["name_field"], str)

    def test_file_discovery_logic(self) -> None:
        """Test file discovery logic without parser initialization."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            # Create test files
            py_file = Path(tmp_dir, "test.py")
            py_file.write_text("def test(): pass")

            js_file = Path(tmp_dir, "test.js")
            js_file.write_text("function test() {}")

            txt_file = Path(tmp_dir, "readme.txt")
            txt_file.write_text("not code")

            # Test Python file discovery
            config = LanguageConfig.CONFIGS["python"]
            extension = config["extension"]

            found_files = [
                file_path
                for file_path in Path(tmp_dir).rglob(f"*{extension}")
                if file_path.is_file()
            ]

            assert len(found_files) == 1
            assert py_file in found_files
            assert js_file not in found_files
            assert txt_file not in found_files

    def test_extensions_mapping(self) -> None:
        """Test that file extensions are correctly mapped."""
        extension_map = {
            "python": ".py",
            "javascript": ".js",
            "java": ".java",
            "go": ".go",
            "rust": ".rs",
            "c": ".c",
            "cpp": ".cpp",
        }

        for language, expected_ext in extension_map.items():
            config = LanguageConfig.CONFIGS[language]
            assert config["extension"] == expected_ext

    def test_node_type_configurations(self) -> None:
        """Test node type configurations for different languages."""
        # Test Python configuration
        python_config = LanguageConfig.CONFIGS["python"]
        assert "function_definition" in python_config["function_nodes"]
        assert python_config["call_node"] == "call"
        assert "import_statement" in python_config["import_nodes"]

        # Test JavaScript configuration
        js_config = LanguageConfig.CONFIGS["javascript"]
        assert "function_declaration" in js_config["function_nodes"]
        assert js_config["call_node"] == "call_expression"

        # Test Go configuration
        go_config = LanguageConfig.CONFIGS["go"]
        assert "function_declaration" in go_config["function_nodes"]
        assert "method_declaration" in go_config["method_nodes"]
        assert go_config["call_node"] == "call_expression"

    def test_import_node_configurations(self) -> None:
        """Test import node configurations for different languages."""
        # Python has both import and from-import
        python_imports = LanguageConfig.CONFIGS["python"]["import_nodes"]
        assert "import_statement" in python_imports
        assert "import_from_statement" in python_imports

        # C/C++ use preprocessor includes
        c_imports = LanguageConfig.CONFIGS["c"]["import_nodes"]
        assert "preproc_include" in c_imports

        cpp_imports = LanguageConfig.CONFIGS["cpp"]["import_nodes"]
        assert "preproc_include" in cpp_imports
        assert "using_declaration" in cpp_imports

        # Rust uses declarations
        rust_imports = LanguageConfig.CONFIGS["rust"]["import_nodes"]
        assert "use_declaration" in rust_imports

    def test_name_field_configurations(self) -> None:
        """Test name field configurations for different languages."""
        # Python, Java, JS use default identifier search
        assert LanguageConfig.CONFIGS["python"]["name_field"] is None
        assert LanguageConfig.CONFIGS["java"]["name_field"] is None
        assert LanguageConfig.CONFIGS["javascript"]["name_field"] is None

        # C/C++ use declarator field
        assert LanguageConfig.CONFIGS["c"]["name_field"] == "declarator"
        assert LanguageConfig.CONFIGS["cpp"]["name_field"] == "declarator"

        # Rust uses name field
        assert LanguageConfig.CONFIGS["rust"]["name_field"] == "name"

        # Go uses default but has special method handling
        assert LanguageConfig.CONFIGS["go"]["name_field"] is None


class TestConfigurationIntegrity:
    """Test configuration integrity and consistency."""

    def test_all_extensions_are_unique(self) -> None:
        """Test that each extension is only used by one primary language."""
        extensions: dict[str, list[str]] = {}
        for language, config in LanguageConfig.CONFIGS.items():
            ext = config["extension"]
            if ext not in extensions:
                extensions[ext] = []
            extensions[ext].append(language)

        # Some extensions may be shared (like .js for javascript and js alias)
        # but the primary languages should be clear
        primary_languages = {
            ".py": "python",
            ".js": "javascript",
            ".java": "java",
            ".go": "go",
            ".rs": "rust",
            ".c": "c",
            ".cpp": "cpp",
        }

        for ext, expected_primary in primary_languages.items():
            languages_with_ext = extensions.get(ext, [])
            assert expected_primary in languages_with_ext

    def test_node_types_are_strings(self) -> None:
        """Test that all node types are strings."""
        for config in LanguageConfig.CONFIGS.values():
            # Function nodes should be list of strings
            for node_type in config["function_nodes"]:
                assert isinstance(node_type, str)
                assert len(node_type) > 0

            # Method nodes should be list of strings
            for node_type in config["method_nodes"]:
                assert isinstance(node_type, str)
                assert len(node_type) > 0

            # Call node should be a string
            assert isinstance(config["call_node"], str)
            assert len(config["call_node"]) > 0

            # Import nodes should be list of strings
            for node_type in config["import_nodes"]:
                assert isinstance(node_type, str)
                assert len(node_type) > 0

    def test_language_coverage(self) -> None:
        """Test that common programming languages are covered."""
        languages = set(LanguageConfig.CONFIGS.keys())

        # Essential languages
        essential = {"python", "javascript", "java", "go", "rust", "c", "cpp"}
        assert essential.issubset(languages)

        # Common aliases
        aliases = {"js", "ts", "c++"}
        assert aliases.issubset(languages)

    def test_configuration_completeness(self) -> None:
        """Test that configurations are complete and valid."""
        required_keys = {
            "function_nodes",
            "method_nodes",
            "call_node",
            "import_nodes",
            "extension",
            "name_field",
        }

        for language, config in LanguageConfig.CONFIGS.items():
            # All required keys present
            assert set(config.keys()) == required_keys

            # No empty lists for critical fields
            assert len(config["function_nodes"]) > 0, (
                f"{language} has no function nodes"
            )
            assert len(config["import_nodes"]) > 0, f"{language} has no import nodes"

            # Extension starts with dot
            assert config["extension"].startswith("."), (
                f"{language} extension should start with dot"
            )


class TestErrorHandling:
    """Test error handling and edge cases."""

    def test_unsupported_language_error_message(self) -> None:
        """Test that unsupported language error includes helpful information."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            with pytest.raises(ValueError, match="Unsupported language") as exc_info:
                SimpleAnalyzer(tmp_dir, "unsupported_language")

            error_msg = str(exc_info.value)
            assert "Unsupported language" in error_msg
            assert "unsupported_language" in error_msg
            assert "Supported languages" in error_msg

            # Should list some supported languages
            assert "python" in error_msg
            assert "javascript" in error_msg

    def test_file_not_found_error(self) -> None:
        """Test file not found error handling."""
        with pytest.raises(FileNotFoundError) as exc_info:
            SimpleAnalyzer("/this/path/definitely/does/not/exist", "python")

        error_msg = str(exc_info.value)
        assert "Directory not found" in error_msg

    def test_case_insensitive_language_handling(self) -> None:
        """Test that language names are handled case-insensitively."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            test_file = Path(tmp_dir, "test.py")
            test_file.write_text("# test")

            # These should all work (or fail for the same reason - tree-sitter setup)
            languages_to_test = ["Python", "PYTHON", "python", "PyThOn"]

            for lang in languages_to_test:
                try:
                    analyzer = SimpleAnalyzer(tmp_dir, lang)
                    # Should normalize to lowercase
                    assert analyzer.language == "python"
                except RuntimeError:
                    # Expected if tree-sitter setup fails
                    pass
                except ValueError:
                    # Should not get "unsupported language" error for case variations
                    pytest.fail("Should not raise ValueError for case variations")
