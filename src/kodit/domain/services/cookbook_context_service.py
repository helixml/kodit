"""Service for gathering context to generate cookbook examples."""

from pathlib import Path

from kodit.infrastructure.slicing.code_elements import (
    FunctionDefinition,
    ModuleDefinition,
)

COOKBOOK_SYSTEM_PROMPT = """You are an expert technical writer creating practical \
cookbook examples for code libraries and frameworks.

Your primary responsibility is ACCURACY. You must:
- ONLY use APIs, classes, functions, and signatures that are explicitly provided
- NEVER invent, guess, or fabricate function signatures, parameters, or return \
types
- If information is insufficient, create fewer examples or simpler examples
- Prefer 2 accurate examples over 6 examples with invented details

Quality over quantity. Accuracy over completeness."""

COOKBOOK_TASK_PROMPT = """Based on the following information about a code repository,
generate a practical cookbook with usage examples.

<repository_context>
{repository_context}
</repository_context>

**Your task:**

1. First, analyze the repository to understand:
   - What is the primary purpose of this library/framework?
   - What are the main features or functionalities it provides?
   - What are the typical use cases or problems it solves?

2. Then generate a cookbook with 4-6 examples following this structure:

**Example 1: Quick Start**
- Show the absolute minimal code to get started
- This should be a "Hello World" that demonstrates the core concept

**Example 2-N: Core Functionality Examples**
- Each example should demonstrate ONE specific feature or use case
- Choose the most important/common functionalities based on the API
- Examples should be practical and solve real problems
- Focus on the features that users would most commonly need

**Format each example as:**

## Example [N]: [Specific Feature/Use Case Name]

**What it does:** [One sentence explaining the use case]

**Code:**
```[language]
[Complete, runnable code example]
```

**Explanation:** [Brief explanation of how it works and when to use it]

**Rules:**
- CRITICAL: ONLY use exact APIs, classes, functions, and signatures shown in \
the API Structure section
- NEVER invent or guess function parameters, return types, or method signatures
- Generate 2-6 examples total depending on available API information
- Each example should focus on a DIFFERENT functionality or use case
- Examples should be complete and runnable (include necessary imports)
- Keep code examples concise but realistic (10-40 lines)
- If you cannot create an accurate example due to insufficient information, \
skip it
- Choose examples based on the library's purpose, not arbitrary difficulty \
levels
- If the repository has clear primary features (e.g., routing, authentication,
  data processing), prioritize those in your examples

**IMPORTANT**: Accuracy is more important than completeness. It is better to \
provide 2 accurate examples than 6 examples with guessed or invented APIs.
"""


class CookbookContextService:
    """Service for gathering context needed to generate cookbook examples."""

    async def gather_context(
        self,
        repo_path: Path,
        language: str,
        api_modules: list[ModuleDefinition] | None = None,
    ) -> str:
        """Gather all relevant context for cookbook generation.

        Args:
            repo_path: Path to the repository
            language: Primary programming language
            api_modules: Optional pre-parsed API module definitions from AST analysis

        """
        sections = []

        # Language
        sections.append(f"## Primary Language\n{language}")

        # README content (important for understanding the library purpose)
        readme = await self._extract_readme_content(repo_path)
        if readme:
            sections.append(f"## README\n{readme}")

        # API Structure from AST analysis (most important!)
        if api_modules:
            api_structure = self._format_api_structure(api_modules)
            if api_structure:
                sections.append(f"## API Structure\n{api_structure}")

        # Package manifest info (version, dependencies)
        manifest = await self._extract_package_manifest(repo_path)
        if manifest:
            sections.append(f"## Package Information\n{manifest}")

        # Existing examples (optional, helps LLM understand patterns)
        examples = await self._find_existing_examples(repo_path)
        if examples:
            sections.append(f"## Existing Examples Found\n{examples}")

        return "\n\n".join(sections) if sections else "No context available"

    def _format_api_structure(  # noqa: C901, PLR0912
        self, modules: list[ModuleDefinition]
    ) -> str:
        """Format API structure from AST-parsed modules.

        Provides a concise overview of the API for the LLM to understand
        what functions/classes are available to use in examples.
        """
        if not modules:
            return ""

        lines = []

        # Limit to top 10 most important modules (sorted by content richness)
        important_modules = sorted(
            modules,
            key=lambda m: (
                len(m.classes) * 3  # Classes are very important
                + len(m.functions) * 2  # Functions are important
                + len(m.types)  # Types are moderately important
            ),
            reverse=True,
        )[:10]

        for module in important_modules:
            lines.append(f"### {module.module_path}")

            if module.module_docstring:
                # Include first line of module docstring
                first_line = module.module_docstring.split("\n")[0]
                lines.append(f"*{first_line}*")

            # List important classes
            if module.classes:
                lines.append("\n**Classes:**")
                for cls in module.classes[:5]:  # Top 5 classes
                    class_info = f"- `{cls.simple_name}`"
                    if cls.docstring:
                        # Add first line of docstring
                        first_line = cls.docstring.split("\n")[0]
                        class_info += f": {first_line}"
                    lines.append(class_info)

                    # Show important methods with signatures
                    if cls.methods:
                        public_methods = [
                            m for m in cls.methods if not m.simple_name.startswith("_")
                        ][:3]
                        if public_methods:
                            for method in public_methods:
                                sig = self._format_signature(method)
                                method_info = f"  - `{sig}`"
                                if method.docstring:
                                    first_line = method.docstring.split("\n")[0]
                                    method_info += f": {first_line}"
                                lines.append(method_info)

            # List important functions with signatures
            if module.functions:
                public_funcs = [
                    f
                    for f in module.functions
                    if not f.simple_name.startswith("_")
                ][:5]
                if public_funcs:
                    lines.append("\n**Functions:**")
                    for func in public_funcs:
                        sig = self._format_signature(func)
                        func_info = f"- `{sig}`"
                        if func.docstring:
                            first_line = func.docstring.split("\n")[0]
                            func_info += f": {first_line}"
                        lines.append(func_info)

            # List types (for languages like Go/Rust)
            if module.types:
                lines.append("\n**Types:**")
                for typ in module.types[:5]:
                    type_info = f"- `{typ.simple_name}`"
                    if typ.docstring:
                        first_line = typ.docstring.split("\n")[0]
                        type_info += f": {first_line}"
                    lines.append(type_info)

            lines.append("")  # Blank line between modules

        return "\n".join(lines)

    def _format_signature(self, func: FunctionDefinition) -> str:
        """Format function signature with parameters and return type."""
        # Build parameter string
        params = ", ".join(func.parameters) if func.parameters else ""

        # Build return type string
        return_type = f" -> {func.return_type}" if func.return_type else ""

        return f"{func.simple_name}({params}){return_type}"

    async def _extract_readme_content(self, repo_path: Path) -> str:
        """Extract and summarize README content."""
        readme_names = ["README.md", "README.rst", "README.txt", "README"]
        readme_content = ""

        for readme_name in readme_names:
            readme_path = repo_path / readme_name
            if readme_path.exists():
                try:
                    content = readme_path.read_text(encoding="utf-8", errors="ignore")
                    # Limit to first 3000 characters to avoid token bloat
                    readme_content = content[:3000]
                    if len(content) > 3000:
                        readme_content += "\n...[truncated]"
                    break
                except Exception:  # noqa: S112, BLE001
                    continue

        return readme_content if readme_content else "No README found"

    async def _extract_package_manifest(self, repo_path: Path) -> str:  # noqa: C901
        """Extract key information from package manifests."""
        manifest_info = []

        # Python
        pyproject = repo_path / "pyproject.toml"
        if pyproject.exists():
            try:
                content = pyproject.read_text(encoding="utf-8", errors="ignore")
                # Extract basic info (simple parsing)
                manifest_info.append(
                    f"Python project (pyproject.toml):\n{content[:500]}"
                )
            except Exception:  # noqa: S110, BLE001
                pass

        setup_py = repo_path / "setup.py"
        if setup_py.exists() and not manifest_info:
            try:
                content = setup_py.read_text(encoding="utf-8", errors="ignore")
                manifest_info.append(f"Python project (setup.py):\n{content[:500]}")
            except Exception:  # noqa: S110, BLE001
                pass

        # JavaScript/TypeScript
        package_json = repo_path / "package.json"
        if package_json.exists():
            try:
                content = package_json.read_text(encoding="utf-8", errors="ignore")
                manifest_info.append(
                    f"Node.js project (package.json):\n{content[:500]}"
                )
            except Exception:  # noqa: S110, BLE001
                pass

        # Go
        go_mod = repo_path / "go.mod"
        if go_mod.exists():
            try:
                content = go_mod.read_text(encoding="utf-8", errors="ignore")
                manifest_info.append(f"Go project (go.mod):\n{content[:500]}")
            except Exception:  # noqa: S110, BLE001
                pass

        # Rust
        cargo_toml = repo_path / "Cargo.toml"
        if cargo_toml.exists():
            try:
                content = cargo_toml.read_text(encoding="utf-8", errors="ignore")
                manifest_info.append(f"Rust project (Cargo.toml):\n{content[:500]}")
            except Exception:  # noqa: S110, BLE001
                pass

        return (
            "\n\n".join(manifest_info)
            if manifest_info
            else "No package manifest found"
        )

    async def _find_existing_examples(self, repo_path: Path) -> str:
        """Find and extract existing example code."""
        example_locations = []

        # Common example directories
        example_dirs = ["examples", "example", "docs/examples", "samples"]
        for example_dir in example_dirs:
            example_path = repo_path / example_dir
            if example_path.exists() and example_path.is_dir():
                # List example files
                example_files = list(example_path.rglob("*.py")) + list(
                    example_path.rglob("*.js")
                )
                example_files += list(example_path.rglob("*.ts"))
                example_files += list(example_path.rglob("*.go"))
                example_files += list(example_path.rglob("*.rs"))

                if example_files:
                    example_locations.append(
                        f"Found {len(example_files)} example files in {example_dir}/"
                    )

                    # Show first example file content
                    if example_files:
                        first_example = example_files[0]
                        try:
                            content = first_example.read_text(
                                encoding="utf-8", errors="ignore"
                            )
                            example_locations.append(
                                f"Sample from {first_example.name}:\n"
                                f"```\n{content[:500]}\n```"
                            )
                        except Exception:  # noqa: S110, BLE001
                            pass

        return (
            "\n".join(example_locations)
            if example_locations
            else "No examples directory found"
        )
