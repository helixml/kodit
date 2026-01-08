"""Service for discovering repository structure and generating intelligent tree summaries."""  # noqa: E501

from pathlib import Path

REPOSITORY_STRUCTURE_ENRICHMENT_SYSTEM_PROMPT = """You are an expert software architect and code analyst.  # noqa: E501
Your task is to intelligently collapse and summarize a repository structure tree to highlight only the most important and interesting components.  # noqa: E501
Deliver a clean, focused tree that helps developers understand the codebase structure.
"""

REPOSITORY_STRUCTURE_ENRICHMENT_TASK_PROMPT = """Below is a complete repository structure tree with descriptions for each file.  # noqa: E501
Your task is to intelligently collapse and summarize this tree to create a focused view that highlights key components.  # noqa: E501

<repository_tree>
{repository_tree}
</repository_tree>

**Guidelines for collapsing:**
1. Keep important files like:
   - Main entry points (main.py, index.js, app.py, etc.)
   - Configuration files (config files, docker-compose, package.json, etc.)
   - Key domain/business logic files
   - Important documentation files (README, ARCHITECTURE, etc.)

2. Collapse/summarize less interesting parts like:
   - Test directories (can be shown as "tests/ - [N files, test coverage for X, Y, Z]")
   - Build artifacts and generated code directories
   - Large directories with repetitive files (show pattern instead of all files)
   - Vendor/dependencies directories
   - Migration directories (can be shown as "migrations/ - [N migrations]")

3. For collapsed sections, provide a brief summary like:
   - "tests/ - 45 test files covering domain, API, and infrastructure layers"
   - "migrations/ - 12 database migrations"
   - "components/ - 23 React components for UI"

4. Preserve the tree structure but reduce noise
5. Keep the tree depth reasonable (collapse deep nested structures)
6. Use your judgment to balance detail with readability

**Return format:**
- Use proper tree formatting with indentation
- Show collapsed sections with a summary in brackets
- Keep important files with their descriptions
- Maintain clear hierarchical structure
- IMPORTANT: Return only the tree content directly. Do NOT wrap your response in markdown code fences.  # noqa: E501
"""

# Common patterns to ignore when walking repository
IGNORE_PATTERNS = {
    ".git",
    ".hg",
    ".svn",
    "__pycache__",
    "node_modules",
    ".venv",
    "venv",
    ".tox",
    ".pytest_cache",
    ".mypy_cache",
    ".ruff_cache",
    "dist",
    "build",
    "*.egg-info",
    ".idea",
    ".vscode",
    ".DS_Store",
    "*.pyc",
    "*.pyo",
    "*.pyd",
    ".coverage",
    "htmlcov",
    ".eggs",
}


class RepositoryStructureService:
    """Service for discovering repository structure and generating trees."""

    def __init__(self, max_files: int = 1000) -> None:
        """Initialize the service."""
        self.max_files = max_files

    async def discover_structure(self, repo_path: Path) -> str:
        """Discover repository structure and generate a tree with file descriptions."""
        tree_lines = []
        file_count = 0

        # Build the tree recursively
        tree_lines.append(f"Repository: {repo_path.name}")
        tree_lines.append("=" * 80)
        tree_lines.append("")

        file_count = await self._build_tree(repo_path, repo_path, tree_lines, "", 0)

        tree_lines.append("")
        tree_lines.append("=" * 80)
        tree_lines.append(f"Total files: {file_count}")

        return "\n".join(tree_lines)

    async def _build_tree(
        self,
        base_path: Path,
        current_path: Path,
        tree_lines: list[str],
        prefix: str,
        depth: int,
        max_depth: int = 10,
    ) -> int:
        """Build tree recursively."""
        if depth > max_depth:
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
        items = [item for item in items if not self._should_ignore(item, base_path)]

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
                    base_path,
                    item,
                    tree_lines,
                    prefix + extension,
                    depth + 1,
                    max_depth,
                )
                file_count += subfile_count
            else:
                description = self._get_file_description(item, base_path)
                tree_lines.append(f"{prefix}{connector}{item.name} - {description}")
                file_count += 1

        return file_count

    def _should_ignore(self, path: Path, base_path: Path) -> bool:
        """Check if path should be ignored."""
        name = path.name

        # Check exact matches
        if name in IGNORE_PATTERNS:
            return True

        # Check pattern matches (simple glob-like matching)
        for pattern in IGNORE_PATTERNS:
            if "*" in pattern:
                # Simple glob matching for extensions
                if pattern.startswith("*."):
                    ext = pattern[1:]  # includes the dot
                    if name.endswith(ext):
                        return True
            elif name == pattern:
                return True

        return False

    def _get_file_description(self, file_path: Path, base_path: Path) -> str:
        """Generate a short description for a file based on its path and extension."""
        name = file_path.name
        suffix = file_path.suffix.lower()
        rel_path = file_path.relative_to(base_path)

        # Configuration files
        config_files = {
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
            ".gitignore": "Git ignore patterns",
            ".dockerignore": "Docker ignore patterns",
            "readme.md": "Project documentation",
            "license": "Project license",
            "changelog.md": "Project changelog",
            "contributing.md": "Contribution guidelines",
        }

        name_lower = name.lower()
        if name_lower in config_files:
            return config_files[name_lower]

        # Extension-based descriptions
        ext_descriptions = {
            ".py": "Python source file",
            ".js": "JavaScript source file",
            ".ts": "TypeScript source file",
            ".tsx": "TypeScript React component",
            ".jsx": "JavaScript React component",
            ".go": "Go source file",
            ".rs": "Rust source file",
            ".java": "Java source file",
            ".cpp": "C++ source file",
            ".c": "C source file",
            ".h": "C/C++ header file",
            ".hpp": "C++ header file",
            ".cs": "C# source file",
            ".rb": "Ruby source file",
            ".php": "PHP source file",
            ".html": "HTML template/page",
            ".css": "Stylesheet",
            ".scss": "SASS stylesheet",
            ".sql": "SQL script",
            ".sh": "Shell script",
            ".yml": "YAML configuration",
            ".yaml": "YAML configuration",
            ".json": "JSON data/configuration",
            ".xml": "XML data/configuration",
            ".toml": "TOML configuration",
            ".ini": "INI configuration",
            ".env": "Environment variables",
            ".md": "Markdown documentation",
            ".txt": "Text file",
            ".proto": "Protocol Buffer definition",
            ".graphql": "GraphQL schema",
        }

        if suffix in ext_descriptions:
            base_desc = ext_descriptions[suffix]
            # Add context from path
            if "test" in str(rel_path).lower():
                return f"{base_desc} (test)"
            if "api" in str(rel_path).lower():
                return f"{base_desc} (API)"
            if "model" in str(rel_path).lower():
                return f"{base_desc} (model)"
            if "service" in str(rel_path).lower():
                return f"{base_desc} (service)"
            if "controller" in str(rel_path).lower():
                return f"{base_desc} (controller)"
            if "util" in str(rel_path).lower() or "helper" in str(rel_path).lower():
                return f"{base_desc} (utility)"
            return base_desc

        # Special patterns
        if name.endswith(("_test.py", "_test.go")):
            return "Unit test file"
        if name.endswith((".test.js", ".test.ts")):
            return "Unit test file"
        if name.endswith((".spec.js", ".spec.ts")):
            return "Test specification"

        # Default
        return "Source file"
