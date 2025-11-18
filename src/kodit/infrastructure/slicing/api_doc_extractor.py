"""API documentation extractor service."""

from collections import defaultdict
from pathlib import Path

import structlog

from kodit.domain.enrichments.usage.api_docs import APIDocEnrichment
from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.ast_analyzer import ASTAnalyzer
from kodit.infrastructure.slicing.code_elements import ModuleDefinition
from kodit.infrastructure.slicing.formatters.factory import create_formatter


class APIDocExtractor:
    """Extract and orchestrate API documentation from code files."""

    # Languages that should have API docs generated
    SUPPORTED_LANGUAGES = frozenset(
        {
            "c",
            "cpp",
            "csharp",
            "go",
            "java",
            "javascript",
            "python",
            "rust",
        }
    )

    def __init__(self) -> None:
        """Initialize the API doc extractor."""
        self.log = structlog.get_logger(__name__)

    def extract_api_docs(
        self,
        files: list[GitFile],
        language: str,
        include_private: bool = False,  # noqa: FBT001, FBT002
    ) -> list[APIDocEnrichment]:
        """Extract API documentation enrichments from files."""
        if not files:
            return []

        # Filter out languages that shouldn't have API docs
        if language not in self.SUPPORTED_LANGUAGES:
            self.log.debug("Language not supported for API docs", language=language)
            return []

        try:
            analyzer = ASTAnalyzer(language)
            parsed_files = analyzer.parse_files(files)
            modules = analyzer.extract_module_definitions(
                parsed_files, include_private=include_private
            )
        except ValueError:
            self.log.debug("Unsupported language", language=language)
            return []

        # Filter modules: must have content, not be tests, and have module_path
        modules_with_content = [
            m
            for m in modules
            if self._has_content(m)
            and not self._is_test_module(m)
            and m.module_path
        ]

        if not modules_with_content:
            return []

        # Merge modules with the same module_path
        merged_modules = self._merge_modules(modules_with_content)

        # Create language-specific formatter
        formatter = create_formatter(language)

        # Generate single markdown document for all modules
        markdown_content = formatter.format_combined_markdown(
            merged_modules,
            language,
        )

        enrichment = APIDocEnrichment(
            language=language,
            content=markdown_content,
        )

        return [enrichment]

    def _has_content(self, module: ModuleDefinition) -> bool:
        """Check if module has any API elements or documentation."""
        return bool(
            module.functions
            or module.classes
            or module.types
            or module.constants
            or module.module_docstring
        )

    def _is_test_module(self, module: ModuleDefinition) -> bool:
        """Check if a module appears to be a test module."""
        # Check module_path for test directories
        module_path_lower = module.module_path.lower()
        module_path_parts = module_path_lower.split("/")

        # Check if any part of the module path is a test directory
        if any(part in ["test", "tests", "__tests__"] for part in module_path_parts):
            return True

        # Check all files in the module for test file name patterns
        for parsed_file in module.files:
            file_path = Path(parsed_file.git_file.path)
            filename = file_path.name.lower()

            # Check for test file name patterns
            if (
                filename.endswith(("_test.go", "_test.py"))
                or filename.startswith("test_")
                or ".test." in filename
                or ".spec." in filename
                or "_mocks." in filename
                or "_mock." in filename
            ):
                return True

        return False

    def _merge_modules(self, modules: list[ModuleDefinition]) -> list[ModuleDefinition]:
        """Merge modules with the same module_path."""
        # Group modules by module_path
        modules_by_path: dict[str, list[ModuleDefinition]] = defaultdict(list)
        for module in modules:
            modules_by_path[module.module_path].append(module)

        # Merge modules with same path
        merged: list[ModuleDefinition] = []
        for module_path, module_group in modules_by_path.items():
            if len(module_group) == 1:
                # No merging needed
                merged.append(module_group[0])
            else:
                # Merge all modules in this group
                merged_module = self._merge_module_group(module_path, module_group)
                merged.append(merged_module)

        return merged

    def _merge_module_group(
        self, module_path: str, module_group: list[ModuleDefinition]
    ) -> ModuleDefinition:
        """Merge a group of modules with the same path into a single module."""
        # Collect all files
        all_files = []
        for mod in module_group:
            all_files.extend(mod.files)

        # Collect all functions
        all_functions = []
        for mod in module_group:
            all_functions.extend(mod.functions)

        # Collect all classes
        all_classes = []
        for mod in module_group:
            all_classes.extend(mod.classes)

        # Collect all types
        all_types = []
        for mod in module_group:
            all_types.extend(mod.types)

        # Collect all constants
        all_constants = []
        for mod in module_group:
            all_constants.extend(mod.constants)

        # Find first non-empty docstring
        module_docstring = ""
        for mod in module_group:
            if mod.module_docstring:
                module_docstring = mod.module_docstring
                break

        # Create merged module
        return ModuleDefinition(
            module_path=module_path,
            module_docstring=module_docstring,
            files=all_files,
            functions=all_functions,
            classes=all_classes,
            types=all_types,
            constants=all_constants,
        )
