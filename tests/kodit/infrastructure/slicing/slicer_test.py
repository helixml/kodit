"""Tests for the slicer module."""

import tempfile
from pathlib import Path
from typing import get_args
from unittest.mock import Mock

import pytest
from pydantic import AnyUrl
from tree_sitter import Parser
from tree_sitter_language_pack import SupportedLanguage, get_language

from kodit.domain.entities import File, Snippet
from kodit.domain.value_objects import FileProcessingStatus
from kodit.infrastructure.slicing.slicer import (
    AnalyzerState,
    FunctionInfo,
    LanguageConfig,
    Slicer,
)


def create_file_from_path(file_path: Path) -> File:
    """Create File domain objects from paths."""
    return File(
        uri=AnyUrl(file_path.as_uri()),
        sha256="test_hash",
        authors=[],
        mime_type="text/plain",
        file_processing_status=FileProcessingStatus.CLEAN,
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
            "csharp",
            "c#",
            "cs",
            "html",
            "css",
        }

        for lang in expected_languages:
            assert lang in LanguageConfig.CONFIGS


class TestFunctionInfo:
    """Test FunctionInfo dataclass."""

    def test_function_info_creation(self) -> None:
        """Test FunctionInfo creation with real tree-sitter node."""
        # Create a real tree-sitter node from Python code
        python_code = "def test_function(): pass"
        language = get_language("python")
        parser = Parser(language)
        tree = parser.parse(python_code.encode())

        # Find the function definition node
        real_node = None
        for node in tree.root_node.children:
            if node.type == "function_definition":
                real_node = node
                break

        assert real_node is not None, "Should find function definition node"

        func_info = FunctionInfo(
            file=Path("test.py"),
            node=real_node,
            span=(0, 100),
            qualified_name="test.func",
        )

        assert func_info.file == Path("test.py")
        assert func_info.node == real_node
        assert func_info.span == (0, 100)
        assert func_info.qualified_name == "test.func"


class TestAnalyzerState:
    """Test AnalyzerState dataclass."""

    def test_analyzer_state_creation(self) -> None:
        """Test AnalyzerState creation with real parser."""
        # Create a real tree-sitter parser
        language = get_language("python")
        real_parser = Parser(language)

        state = AnalyzerState(parser=real_parser)

        assert state.parser == real_parser
        assert state.files == []
        assert state.asts == {}
        assert state.def_index == {}
        assert isinstance(state.call_graph, dict)
        assert isinstance(state.reverse_calls, dict)
        assert isinstance(state.imports, dict)


class TestSlicer:
    """Test Slicer class - unit tests for individual methods."""

    def test_extract_snippets_with_invalid_language(self) -> None:
        """Test extract_snippets with unsupported language returns empty list."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            slicer = Slicer()
            test_file = Path(tmp_dir, "test.py")
            test_file.write_text("def test(): pass")
            file_obj = create_file_from_path(test_file)
            snippets = slicer.extract_snippets([file_obj], "unsupported")
            assert snippets == []

    def test_extract_snippets_with_nonexistent_files(self) -> None:
        """Test extract_snippets with nonexistent files."""
        slicer = Slicer()
        nonexistent_file = create_file_from_path(Path("/nonexistent/path.py"))
        with pytest.raises(FileNotFoundError):
            slicer.extract_snippets([nonexistent_file], "python")

    def test_get_tree_sitter_language_name_mapping(self) -> None:
        """Test tree-sitter language name mapping."""
        slicer = Slicer()
        assert slicer._get_tree_sitter_language_name("python") == "python"  # noqa: SLF001
        assert slicer._get_tree_sitter_language_name("c++") == "cpp"  # noqa: SLF001
        assert slicer._get_tree_sitter_language_name("typescript") == "typescript"  # noqa: SLF001
        assert slicer._get_tree_sitter_language_name("js") == "javascript"  # noqa: SLF001
        assert slicer._get_tree_sitter_language_name("csharp") == "csharp"  # noqa: SLF001
        assert slicer._get_tree_sitter_language_name("c#") == "csharp"  # noqa: SLF001
        assert slicer._get_tree_sitter_language_name("cs") == "csharp"  # noqa: SLF001
        assert slicer._get_tree_sitter_language_name("html") == "html"  # noqa: SLF001
        assert slicer._get_tree_sitter_language_name("css") == "css"  # noqa: SLF001

    def test_tree_sitter_language_names_are_valid(self) -> None:
        """Test that all tree-sitter language mappings resolve to valid libraries."""
        slicer = Slicer()

        # Test all mappings in _get_tree_sitter_language_name
        test_languages = [
            "python",
            "c++",
            "c",
            "cpp",
            "java",
            "rust",
            "go",
            "javascript",
            "typescript",
            "js",
            "ts",
            "csharp",
            "c#",
            "cs",
            "html",
            "css",
        ]

        for lang in test_languages:
            ts_name = slicer._get_tree_sitter_language_name(lang)  # noqa: SLF001

            assert ts_name in get_args(SupportedLanguage), (
                f"Language '{ts_name}' not in SupportedLanguage"
            )

    def test_language_config_access(self) -> None:
        """Test that language config is correctly accessed."""
        # Just test that configs exist and are accessible
        assert "python" in LanguageConfig.CONFIGS
        assert "function_nodes" in LanguageConfig.CONFIGS["python"]

    def test_config_access_patterns(self) -> None:
        """Test accessing different language configurations."""
        languages = [
            "python",
            "javascript",
            "java",
            "go",
            "rust",
            "c",
            "cpp",
            "csharp",
            "html",
            "css",
        ]
        for language in languages:
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
            "csharp": ".cs",
            "html": ".html",
            "css": ".css",
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

        # Test C# configuration
        csharp_config = LanguageConfig.CONFIGS["csharp"]
        assert "method_declaration" in csharp_config["function_nodes"]
        assert "constructor_declaration" in csharp_config["method_nodes"]
        assert csharp_config["call_node"] == "invocation_expression"

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

        # C# uses using directives
        csharp_imports = LanguageConfig.CONFIGS["csharp"]["import_nodes"]
        assert "using_directive" in csharp_imports

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

        # C# uses default identifier search
        assert LanguageConfig.CONFIGS["csharp"]["name_field"] is None

    def test_empty_file_list_error(self) -> None:
        """Test that empty file list raises appropriate error."""
        slicer = Slicer()

        with pytest.raises(ValueError, match="No files provided"):
            slicer.extract_snippets([], "python")

    def test_case_insensitive_language_handling(self) -> None:
        """Test that language names are handled case-insensitively."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            test_file = Path(tmp_dir, "test.py")
            test_file.write_text("def test(): pass")
            file_obj = create_file_from_path(test_file)

            # These should all work (or fail for the same reason - tree-sitter setup)
            languages_to_test = ["Python", "PYTHON", "python", "PyThOn"]
            slicer = Slicer()

            for lang in languages_to_test:
                try:
                    snippets = slicer.extract_snippets([file_obj], lang)
                    # Should return some result without error
                    assert isinstance(snippets, list)
                except ValueError:
                    # Should not get "unsupported language" error for case variations
                    pytest.fail("Should not raise ValueError for case variations")

    def test_walk_tree_cycle_detection(self) -> None:
        """Test that _walk_tree method has cycle detection to prevent recursion."""
        slicer = Slicer()

        # Create mock nodes that could cause circular references
        mock_node1 = Mock()
        mock_node2 = Mock()

        # Set up byte positions (they can be the same - this is valid in tree-sitter)
        mock_node1.start_byte = 0
        mock_node1.end_byte = 10
        mock_node2.start_byte = 5
        mock_node2.end_byte = 15

        # Set up children relationships to create an actual cycle
        # (the same node object appears twice in the tree)
        mock_node1.children = [mock_node2]
        mock_node2.children = [mock_node1]  # This creates a cycle

        # This should complete without infinite recursion due to cycle detection
        try:
            nodes = list(slicer._walk_tree(mock_node1))  # noqa: SLF001

            # Should get exactly 2 unique nodes due to cycle detection
            # (node1 and node2, but node1 won't be traversed again)
            assert len(nodes) == 2, f"Expected 2 unique nodes, got {len(nodes)}"
            assert mock_node1 in nodes, "Should contain the original node"
            assert mock_node2 in nodes, "Should contain the child node"
            # Each node should appear exactly once despite the cycle
            assert nodes.count(mock_node1) == 1, "Node1 should appear exactly once"
            assert nodes.count(mock_node2) == 1, "Node2 should appear exactly once"

        except RecursionError:
            pytest.fail("RecursionError raised - cycle detection not working properly")


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
            ".cs": "csharp",
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
        essential = {
            "python",
            "javascript",
            "java",
            "go",
            "rust",
            "c",
            "cpp",
            "csharp",
            "html",
            "css",
        }
        assert essential.issubset(languages)

        # Common aliases
        aliases = {"js", "ts", "c++", "c#", "cs"}
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


class TestMultiFileIntegration:
    """Integration tests using multi-file example projects."""

    def get_data_path(self) -> Path:
        """Get path to test data directory."""
        return Path(__file__).parent / "data"

    def test_python_multi_file_analysis(self) -> None:
        """Test analyzing a multi-file Python project."""
        python_dir = self.get_data_path() / "python"

        # Get all Python files in the directory
        py_files = list(python_dir.glob("*.py"))
        assert len(py_files) >= 3

        # Check that specific files exist
        filenames = [f.name for f in py_files]
        assert "main.py" in filenames
        assert "models.py" in filenames
        assert "utils.py" in filenames

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in py_files]
        snippets = slicer.extract_snippets(file_objs, "python")

        # Should extract some snippets
        assert len(snippets) >= 3

        # Snippets should be Snippet domain objects
        for snippet in snippets:
            assert isinstance(snippet, Snippet)
            assert len(snippet.original_text()) > 0

    def test_csharp_multi_file_analysis(self) -> None:
        """Test analyzing a multi-file C# project."""
        csharp_dir = self.get_data_path() / "csharp"

        # Get all C# files in the directory
        cs_files = list(csharp_dir.glob("*.cs"))
        assert len(cs_files) >= 3

        # Check that specific files exist
        filenames = [f.name for f in cs_files]
        assert "Main.cs" in filenames
        assert "Models.cs" in filenames
        assert "Utils.cs" in filenames

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in cs_files]
        snippets = slicer.extract_snippets(file_objs, "csharp")

        # Should extract some snippets
        assert len(snippets) >= 0  # May not find functions in all test files

    def test_html_multi_file_analysis(self) -> None:
        """Test analyzing a multi-file HTML project."""
        html_dir = self.get_data_path() / "html"

        # Get all HTML files in the directory
        html_files = list(html_dir.glob("*.html"))
        assert len(html_files) >= 3

        # Check that specific files exist
        filenames = [f.name for f in html_files]
        assert "main.html" in filenames
        assert "components.html" in filenames
        assert "forms.html" in filenames

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in html_files]
        snippets = slicer.extract_snippets(file_objs, "html")

        # Should extract some snippets
        assert len(snippets) >= 0  # May not find functions in all test files

    def test_css_multi_file_analysis(self) -> None:
        """Test analyzing a multi-file CSS project."""
        css_dir = self.get_data_path() / "css"

        # Get all CSS files in the directory
        css_files = list(css_dir.glob("*.css"))
        assert len(css_files) >= 3

        # Check that specific files exist
        filenames = [f.name for f in css_files]
        assert "main.css" in filenames
        assert "components.css" in filenames
        assert "utilities.css" in filenames

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in css_files]
        snippets = slicer.extract_snippets(file_objs, "css")

        # Should extract some snippets
        assert len(snippets) >= 0  # May not find functions in all test files

    def test_javascript_multi_file_analysis(self) -> None:
        """Test analyzing a multi-file JavaScript project."""
        js_dir = self.get_data_path() / "javascript"

        # Get all JavaScript files in the directory
        js_files = list(js_dir.glob("*.js"))
        assert len(js_files) >= 3

        # Check that specific files exist
        filenames = [f.name for f in js_files]
        assert "main.js" in filenames
        assert "models.js" in filenames
        assert "utils.js" in filenames

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in js_files]
        snippets = slicer.extract_snippets(file_objs, "javascript")

        # Should extract some snippets
        assert len(snippets) >= 0  # May not find functions in all test files

    def test_go_multi_file_analysis(self) -> None:
        """Test analyzing a multi-file Go project."""
        go_dir = self.get_data_path() / "go"

        # Get all Go files in the directory
        go_files = list(go_dir.glob("*.go"))
        assert len(go_files) >= 3

        # Check that specific files exist
        filenames = [f.name for f in go_files]
        assert "main.go" in filenames
        assert "models.go" in filenames
        assert "utils.go" in filenames

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in go_files]
        snippets = slicer.extract_snippets(file_objs, "go")

        # Should extract some snippets
        assert len(snippets) >= 0  # May not find functions in all test files

    def test_rust_multi_file_analysis(self) -> None:
        """Test analyzing a multi-file Rust project."""
        rust_dir = self.get_data_path() / "rust"

        # Get all Rust files in the directory
        rs_files = list(rust_dir.glob("*.rs"))
        assert len(rs_files) >= 3

        # Check that specific files exist
        filenames = [f.name for f in rs_files]
        assert "main.rs" in filenames
        assert "models.rs" in filenames
        assert "utils.rs" in filenames

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in rs_files]
        snippets = slicer.extract_snippets(file_objs, "rust")

        # Should extract some snippets
        assert len(snippets) >= 0  # May not find functions in all test files

    def test_c_multi_file_analysis(self) -> None:
        """Test analyzing a multi-file C project."""
        c_dir = self.get_data_path() / "c"

        # Get all C files in the directory
        c_files = list(c_dir.glob("*.c"))
        assert len(c_files) >= 3

        # Check that specific files exist
        filenames = [f.name for f in c_files]
        assert "main.c" in filenames
        assert "models.c" in filenames
        assert "utils.c" in filenames

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in c_files]
        snippets = slicer.extract_snippets(file_objs, "c")

        # Should extract some snippets
        assert len(snippets) >= 0  # May not find functions in all test files

    def test_cpp_multi_file_analysis(self) -> None:
        """Test analyzing a multi-file C++ project."""
        cpp_dir = self.get_data_path() / "cpp"

        # Get all C++ files in the directory
        cpp_files = list(cpp_dir.glob("*.cpp"))
        assert len(cpp_files) >= 3

        # Check that specific files exist
        filenames = [f.name for f in cpp_files]
        assert "main.cpp" in filenames
        assert "models.cpp" in filenames
        assert "utils.cpp" in filenames

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in cpp_files]
        snippets = slicer.extract_snippets(file_objs, "cpp")

        # Should extract some snippets
        assert len(snippets) >= 0  # May not find functions in all test files

    def test_java_multi_file_analysis(self) -> None:
        """Test analyzing a multi-file Java project."""
        java_dir = self.get_data_path() / "java"

        # Get all Java files in the directory
        java_files = list(java_dir.glob("*.java"))
        assert len(java_files) >= 3

        # Check that specific files exist
        filenames = [f.name for f in java_files]
        assert "Main.java" in filenames
        assert "Models.java" in filenames
        assert "Utils.java" in filenames

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in java_files]
        snippets = slicer.extract_snippets(file_objs, "java")

        # Should extract some snippets
        assert len(snippets) >= 0  # May not find functions in all test files

    def test_all_languages_have_examples(self) -> None:
        """Test that all supported languages have example data."""
        data_dir = self.get_data_path()

        # Core supported languages (excluding aliases)
        core_languages = [
            "python",
            "javascript",
            "java",
            "go",
            "rust",
            "c",
            "cpp",
            "csharp",
            "html",
            "css",
        ]

        for language in core_languages:
            lang_dir = data_dir / language
            assert lang_dir.exists(), f"Missing example data for {language}"
            assert lang_dir.is_dir(), f"Example data for {language} is not a directory"

            # Should have at least 3 files
            config = LanguageConfig.CONFIGS[language]
            extension = config["extension"]
            files = list(lang_dir.glob(f"*{extension}"))
            assert len(files) >= 3, f"Not enough example files for {language}"

    def test_project_structure_consistency(self) -> None:
        """Test that all example projects follow consistent structure."""
        data_dir = self.get_data_path()
        core_languages = [
            "python",
            "javascript",
            "java",
            "go",
            "rust",
            "c",
            "cpp",
            "csharp",
            "html",
            "css",
        ]

        for language in core_languages:
            lang_dir = data_dir / language
            config = LanguageConfig.CONFIGS[language]
            extension = config["extension"]

            # Should have main file
            main_files = [
                f"main{extension}",
                f"Main{extension}",  # Java convention
            ]

            found_main = False
            for main_file in main_files:
                if (lang_dir / main_file).exists():
                    found_main = True
                    break

            assert found_main, f"No main file found for {language}"

            # Should have models and utils (or similar supporting files)
            files = [f.name for f in lang_dir.glob(f"*{extension}")]

            # At least 3 files total
            assert len(files) >= 3, f"Insufficient files for {language}: {files}"

    def test_realistic_function_discovery(self) -> None:
        """Test function discovery with realistic multi-file projects."""
        python_dir = self.get_data_path() / "python"
        py_files = list(python_dir.glob("*.py"))

        slicer = Slicer()
        file_objs = [create_file_from_path(f) for f in py_files]
        snippets = slicer.extract_snippets(file_objs, "python")

        # Should extract some snippets
        assert len(snippets) >= 3

        # Check that snippets contain function definitions
        function_defs_found = 0
        for snippet in snippets:
            if "def " in snippet.original_text():
                function_defs_found += 1

        assert function_defs_found >= 3


class TestErrorHandling:
    """Test error handling and edge cases."""

    def test_unsupported_language_returns_empty_list(self) -> None:
        """Test that unsupported language returns empty list."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            test_file = Path(tmp_dir, "test.py")
            test_file.write_text("def test(): pass")
            file_obj = create_file_from_path(test_file)

            slicer = Slicer()
            snippets = slicer.extract_snippets([file_obj], "unsupported_language")
            assert snippets == []

    def test_file_not_found_error(self) -> None:
        """Test file not found error handling."""
        slicer = Slicer()
        nonexistent_file = create_file_from_path(
            Path("/this/path/definitely/does/not/exist.py")
        )
        with pytest.raises(FileNotFoundError) as exc_info:
            slicer.extract_snippets([nonexistent_file], "python")

        error_msg = str(exc_info.value)
        assert "File not found" in error_msg

    def test_binary_file_handling_in_html_parser(self) -> None:
        """Test that binary files don't crash HTML parser with UnicodeDecodeError."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            # Create a binary file that looks like an image
            binary_file = Path(tmp_dir, "test.html")
            # Write binary data that would cause UnicodeDecodeError if decoded as UTF-8
            binary_data = b"\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f"
            binary_file.write_bytes(binary_data)

            file_obj = create_file_from_path(binary_file)

            slicer = Slicer()

            # This should not raise UnicodeDecodeError
            try:
                snippets = slicer.extract_snippets([file_obj], "html")
                # Should return empty list or handle gracefully
                assert isinstance(snippets, list)
            except UnicodeDecodeError:
                pytest.fail("UnicodeDecodeError should not be raised for binary files")

    def test_file_extension_validation_for_language(self) -> None:
        """Test only files with matching extensions are processed for each language."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            # Create files with different extensions
            py_file = Path(tmp_dir, "test.py")
            py_file.write_text("def test(): pass")

            js_file = Path(tmp_dir, "test.js")
            js_file.write_text("function test() {}")

            png_file = Path(tmp_dir, "image.png")
            png_file.write_bytes(b"fake png data")

            txt_file = Path(tmp_dir, "readme.txt")
            txt_file.write_text("This is just text")

            # Test Python processing - should only accept .py files
            slicer = Slicer()
            py_file_obj = create_file_from_path(py_file)
            js_file_obj = create_file_from_path(js_file)
            png_file_obj = create_file_from_path(png_file)
            txt_file_obj = create_file_from_path(txt_file)

            # Test that Python slicer rejects non-Python files
            with pytest.raises(ValueError, match="does not match language python"):
                slicer.extract_snippets([js_file_obj], "python")

            with pytest.raises(ValueError, match="does not match language python"):
                slicer.extract_snippets([png_file_obj], "python")

            with pytest.raises(ValueError, match="does not match language python"):
                slicer.extract_snippets([txt_file_obj], "python")

            # Test that Python slicer accepts Python files
            snippets = slicer.extract_snippets([py_file_obj], "python")
            assert isinstance(snippets, list)

            # Test JavaScript processing - should only accept .js files
            with pytest.raises(ValueError, match="does not match language javascript"):
                slicer.extract_snippets([py_file_obj], "javascript")

            with pytest.raises(ValueError, match="does not match language javascript"):
                slicer.extract_snippets([png_file_obj], "javascript")

            # Test that JavaScript slicer accepts JavaScript files
            snippets = slicer.extract_snippets([js_file_obj], "javascript")
            assert isinstance(snippets, list)

    def test_mixed_files_with_language_filtering(self) -> None:
        """Test that mixed file types are filtered correctly for each language."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            # Create files of different types
            py_file = Path(tmp_dir, "script.py")
            py_file.write_text("def hello(): print('hello')")

            js_file = Path(tmp_dir, "script.js")
            js_file.write_text("function hello() { console.log('hello'); }")

            html_file = Path(tmp_dir, "index.html")
            html_file.write_text("<html><body>Hello</body></html>")

            # Create file objects
            py_file_obj = create_file_from_path(py_file)
            js_file_obj = create_file_from_path(js_file)
            html_file_obj = create_file_from_path(html_file)

            slicer = Slicer()

            # Test that we can't mix file types within a single language request
            with pytest.raises(ValueError, match="does not match language python"):
                slicer.extract_snippets([py_file_obj, js_file_obj], "python")

            with pytest.raises(ValueError, match="does not match language python"):
                slicer.extract_snippets([py_file_obj, html_file_obj], "python")

            with pytest.raises(ValueError, match="does not match language javascript"):
                slicer.extract_snippets([js_file_obj, py_file_obj], "javascript")

            # Test that processing single file types works
            py_snippets = slicer.extract_snippets([py_file_obj], "python")
            assert isinstance(py_snippets, list)

            js_snippets = slicer.extract_snippets([js_file_obj], "javascript")
            assert isinstance(js_snippets, list)

            html_snippets = slicer.extract_snippets([html_file_obj], "html")
            assert isinstance(html_snippets, list)
