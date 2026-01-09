"""Service for discovering repository structure and generating tree summaries."""

from pathlib import Path
from typing import TYPE_CHECKING

import structlog

if TYPE_CHECKING:
    from tree_sitter import Node

    from kodit.infrastructure.slicing.language_analyzer import LanguageAnalyzer

log = structlog.get_logger(__name__)

# Map file extensions to tree-sitter language names
EXTENSION_TO_LANGUAGE = {
    ".py": "python",
    ".go": "go",
    ".js": "javascript",
    ".ts": "typescript",
    ".tsx": "typescript",
    ".jsx": "javascript",
    ".java": "java",
    ".c": "c",
    ".cpp": "cpp",
    ".rs": "rust",
    ".cs": "csharp",
}

REPOSITORY_STRUCTURE_ENRICHMENT_SYSTEM_PROMPT = (
    "You are an expert software architect and code analyst. "
    "Your task is to intelligently collapse and summarize a repository structure "
    "tree to highlight only the most important and interesting components. "
    "Deliver a clean, focused tree that helps developers understand the "
    "codebase structure."
)

REPOSITORY_STRUCTURE_ENRICHMENT_TASK_PROMPT = """Below is a repository structure \
tree. Source files include code signatures (classes, functions) that describe \
their contents. Transform these signatures into brief, readable descriptions.

<repository_tree>
{repository_tree}
</repository_tree>

**Your task:**
For each source file, convert the code signatures into a brief description of \
what the file does. For example:
- Input: "backtesting.py - class Backtest(Algorithm), def run_simulation()"
- Output: "backtesting.py - Backtests trading algorithms with simulation support"

- Input: "user_service.py - class UserService, def create_user(), def get_user()"
- Output: "user_service.py - Manages user creation and retrieval"

**CRITICAL: What to EXPAND (show all files with descriptions):**
- src/, lib/, pkg/, internal/, core/ directories - the main source code
- Domain/business logic directories
- API and service directories

**What to COLLAPSE (summarize with file count):**
- tests/, test/, __tests__/ directories -> "tests/ - N test files"
- examples/, example/ directories -> "examples/ - N example files"
- docs/, documentation/ directories -> "docs/ - N documentation files"
- migrations/ directories -> "migrations/ - N database migrations"

**Guidelines:**
1. ALWAYS expand source directories and describe each file based on its signatures
2. Convert code signatures to human-readable descriptions (what it does, not what it is)
3. Keep important root files: README.md, pyproject.toml, package.json, Dockerfile
4. Preserve tree structure with proper indentation

**Return format:**
- Use tree formatting: ├── for items, └── for last item, │ for continuation
- IMPORTANT: Return only the tree content directly. Do NOT wrap in markdown fences.
"""

# Known configuration files and their descriptions
CONFIG_FILE_DESCRIPTIONS = {
    "dockerfile": "Docker container configuration",
    "docker-compose.yml": "Docker Compose orchestration",
    "docker-compose.yaml": "Docker Compose orchestration",
    "package.json": "Node.js package configuration",
    "requirements.txt": "Python dependencies",
    "pyproject.toml": "Python project configuration",
    "setup.py": "Python package setup",
    "cargo.toml": "Rust package configuration",
    "go.mod": "Go module configuration",
    "makefile": "Build automation",
    "readme.md": "Project documentation",
    "license": "Project license",
    "changelog.md": "Project changelog",
    "contributing.md": "Contribution guidelines",
}

# Non-code file descriptions by extension
SIMPLE_FILE_DESCRIPTIONS = {
    ".html": "HTML template",
    ".css": "Stylesheet",
    ".scss": "SASS stylesheet",
    ".sql": "SQL script",
    ".sh": "Shell script",
    ".yml": "YAML configuration",
    ".yaml": "YAML configuration",
    ".json": "JSON configuration",
    ".xml": "XML configuration",
    ".toml": "TOML configuration",
    ".ini": "INI configuration",
    ".env": "Environment variables",
    ".md": "Documentation",
    ".txt": "Text file",
    ".proto": "Protocol Buffer definition",
    ".graphql": "GraphQL schema",
}

# Common patterns to ignore when walking repository
IGNORE_PATTERNS = {
    # Version control
    ".git",
    ".hg",
    ".svn",
    # Python cache/build
    "__pycache__",
    ".venv",
    "venv",
    ".tox",
    ".pytest_cache",
    ".mypy_cache",
    ".ruff_cache",
    "dist",
    "build",
    "*.egg-info",
    "*.pyc",
    "*.pyo",
    "*.pyd",
    ".coverage",
    "htmlcov",
    ".eggs",
    # Node
    "node_modules",
    # IDE
    ".idea",
    ".vscode",
    # OS
    ".DS_Store",
    # Lock files (dependencies are in config files)
    "uv.lock",
    "package-lock.json",
    "yarn.lock",
    "pnpm-lock.yaml",
    "Cargo.lock",
    "poetry.lock",
    "Gemfile.lock",
    "composer.lock",
    "go.sum",
}


class RepositoryStructureService:
    """Service for discovering repository structure and generating trees."""

    def __init__(self, max_files: int = 1000, max_depth: int = 10) -> None:
        """Initialize the service."""
        self.max_files = max_files
        self.max_depth = max_depth
        self._base_path: Path | None = None

    async def discover_structure(self, repo_path: Path, repo_url: str = "") -> str:
        """Discover repository structure and generate a tree with file descriptions."""
        self._base_path = repo_path
        tree_lines: list[str] = []

        # Use repo URL for display if provided, otherwise use path name
        display_name = repo_url if repo_url else repo_path.name
        tree_lines.append(f"Repository: {display_name}")
        tree_lines.append("")

        file_count = await self._build_tree(repo_path, tree_lines, "", 0)

        tree_lines.append("")
        tree_lines.append(f"Total files: {file_count}")

        return "\n".join(tree_lines)

    async def _build_tree(
        self,
        current_path: Path,
        tree_lines: list[str],
        prefix: str,
        depth: int,
    ) -> int:
        """Build tree recursively."""
        if depth > self.max_depth:
            return 0

        file_count = 0

        try:
            # Get all items in current directory
            items = sorted(
                current_path.iterdir(), key=lambda x: (not x.is_dir(), x.name)
            )
        except PermissionError:
            return 0

        # Filter out ignored patterns
        items = [item for item in items if not self._should_ignore(item)]

        for i, item in enumerate(items):
            if file_count >= self.max_files:
                tree_lines.append(f"{prefix}... (truncated, max files reached)")
                break

            is_last = i == len(items) - 1
            connector = "└── " if is_last else "├── "
            extension = "    " if is_last else "│   "

            if item.is_dir():
                tree_lines.append(f"{prefix}{connector}{item.name}/")
                subfile_count = await self._build_tree(
                    item,
                    tree_lines,
                    prefix + extension,
                    depth + 1,
                )
                file_count += subfile_count
            else:
                description = self._get_file_description(item)
                tree_lines.append(f"{prefix}{connector}{item.name} - {description}")
                file_count += 1

        return file_count

    def _should_ignore(self, path: Path) -> bool:
        """Check if path should be ignored."""
        name = path.name

        # Check exact matches in ignore patterns
        if name in IGNORE_PATTERNS:
            return True

        # Ignore dotfiles (files starting with .) but keep dot-directories like .github/
        if name.startswith(".") and path.is_file():
            return True

        # Check pattern matches (simple glob-like matching for extensions)
        for pattern in IGNORE_PATTERNS:
            if "*" in pattern and pattern.startswith("*."):
                ext = pattern[1:]  # includes the dot
                if name.endswith(ext):
                    return True

        return False

    def _get_file_description(self, file_path: Path) -> str:
        """Generate a description with code signatures for source files."""
        name = file_path.name
        name_lower = name.lower()

        # Check configuration files first
        config_desc = self._get_config_description(name_lower)
        if config_desc:
            return config_desc

        # For source code files, extract code signatures
        suffix = file_path.suffix.lower()
        if suffix in EXTENSION_TO_LANGUAGE:
            signatures = self._extract_code_signatures(file_path)
            if signatures:
                return signatures

        # Fall back to simple descriptions for non-code files
        return self._get_simple_description(file_path)

    def _extract_code_signatures(self, file_path: Path) -> str:
        """Extract class and function signatures from a source file."""
        from tree_sitter import Parser
        from tree_sitter_language_pack import get_language

        from kodit.infrastructure.slicing.language_analyzer import (
            language_analyzer_factory,
        )

        suffix = file_path.suffix.lower()
        language = EXTENSION_TO_LANGUAGE.get(suffix)
        if not language:
            return ""

        try:
            analyzer = language_analyzer_factory(language)
            ts_language = get_language(analyzer.metadata().tree_sitter_name)
            parser = Parser(ts_language)

            with file_path.open("rb") as f:
                source_code = f.read()

            tree = parser.parse(source_code)
            signatures = self._collect_signatures(tree.root_node, analyzer)

            if signatures:
                # Limit total length to ~100 chars
                result = ", ".join(signatures)
                if len(result) > 100:
                    result = result[:97] + "..."
                return result

        except (OSError, ValueError, UnicodeDecodeError) as e:
            log.debug("Failed to extract signatures", path=str(file_path), error=str(e))

        return ""

    def _collect_signatures(
        self,
        root_node: "Node",
        analyzer: "LanguageAnalyzer",
    ) -> list[str]:
        """Collect class and function signatures from AST."""
        signatures: list[str] = []
        node_types = analyzer.node_types()
        class_types = {"class_definition", "class_declaration", "type_declaration"}

        # Walk the tree looking for classes and functions
        queue = [root_node]
        visited: set[int] = set()

        while queue and len(signatures) < 5:  # Limit to 5 signatures
            node = queue.pop(0)
            node_id = id(node)
            if node_id in visited:
                continue
            visited.add(node_id)

            # Check for class definitions
            if node.type in class_types:
                class_sig = self._extract_class_signature(node)
                if class_sig:
                    signatures.append(class_sig)

            # Check for function definitions
            elif node.type in node_types.all_function_nodes:
                func_name = analyzer.extract_function_name(node)
                if func_name and not func_name.startswith("_"):
                    signatures.append(f"def {func_name}()")

            queue.extend(node.children)

        return signatures

    def _extract_class_signature(self, node: "Node") -> str:
        """Extract a class signature like 'class Foo(Bar):'."""
        # Find the class name
        class_name = None
        base_classes: list[str] = []

        for child in node.children:
            if child.type == "identifier" and child.text:
                class_name = child.text.decode("utf-8")
            elif child.type == "argument_list":
                # Python base classes
                base_classes.extend(
                    arg.text.decode("utf-8")
                    for arg in child.children
                    if arg.type == "identifier" and arg.text
                )
            elif child.type == "type_identifier" and child.text:
                # Go/other languages
                class_name = child.text.decode("utf-8")

        if not class_name:
            return ""

        if base_classes:
            return f"class {class_name}({', '.join(base_classes)})"
        return f"class {class_name}"

    def _get_config_description(self, name_lower: str) -> str:
        """Return description for known configuration files."""
        return CONFIG_FILE_DESCRIPTIONS.get(name_lower, "")

    def _get_simple_description(self, file_path: Path) -> str:
        """Return simple description for non-code files."""
        return SIMPLE_FILE_DESCRIPTIONS.get(file_path.suffix.lower(), "")
