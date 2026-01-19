"""Tests for TSX/TypeScript-specific slicer behavior."""

from datetime import UTC, datetime
from pathlib import Path

import pytest

from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.ast_analyzer import ASTAnalyzer
from kodit.infrastructure.slicing.slicer import Slicer


class TestTsxSlicer:
    """Test TSX-specific snippet extraction."""

    @pytest.fixture
    def tsx_git_files(self) -> list[GitFile]:
        """Load the TSX test files."""
        data_dir = Path(__file__).parent / "data" / "tsx"
        files = list(data_dir.glob("*.tsx"))
        return [
            GitFile(
                created_at=datetime.now(tz=UTC),
                blob_sha=f"sha_{f.name}",
                commit_sha="test_commit",
                path=str(f),
                mime_type="text/plain",
                size=f.stat().st_size,
                extension=".tsx",
            )
            for f in files
        ]

    def test_extracts_named_arrow_functions(self, tsx_git_files: list[GitFile]) -> None:
        """Named arrow functions like 'const foo = () => {}' should be extracted."""
        analyzer = ASTAnalyzer("tsx")
        parsed_files = analyzer.parse_files(tsx_git_files)
        functions, _, _ = analyzer.extract_definitions(
            parsed_files, include_private=True
        )

        function_names = [f.simple_name for f in functions]

        assert "addTodo" in function_names
        assert "toggleTodo" in function_names
        assert "deleteTodo" in function_names

    def test_extracts_function_declarations(self, tsx_git_files: list[GitFile]) -> None:
        """Regular function declarations should be extracted."""
        analyzer = ASTAnalyzer("tsx")
        parsed_files = analyzer.parse_files(tsx_git_files)
        functions, _, _ = analyzer.extract_definitions(
            parsed_files, include_private=True
        )

        function_names = [f.simple_name for f in functions]

        assert "App" in function_names

    def test_extracts_interfaces(self, tsx_git_files: list[GitFile]) -> None:
        """TypeScript interfaces should be extracted as types."""
        analyzer = ASTAnalyzer("tsx")
        parsed_files = analyzer.parse_files(tsx_git_files)
        _, _, types = analyzer.extract_definitions(parsed_files, include_private=True)

        type_names = [t.simple_name for t in types]

        assert "Todo" in type_names

    def test_filters_out_inline_callbacks(self, tsx_git_files: list[GitFile]) -> None:
        """Inline arrow functions like '.filter(t => ...)' should NOT be extracted."""
        analyzer = ASTAnalyzer("tsx")
        parsed_files = analyzer.parse_files(tsx_git_files)
        functions, _, _ = analyzer.extract_definitions(
            parsed_files, include_private=True
        )

        # 't' is the parameter name used in inline callbacks like .filter(t => ...)
        # These should not appear as function names
        function_names = [f.simple_name for f in functions]

        assert "t" not in function_names

    def test_arrow_function_snippet_includes_const_declaration(
        self, tsx_git_files: list[GitFile]
    ) -> None:
        """Arrow function snippets should include 'const name = ' prefix."""
        slicer = Slicer()
        snippets = slicer.extract_snippets_from_git_files(tsx_git_files, "tsx")

        # Find a snippet that starts with 'const addTodo' (the addTodo snippet)
        add_todo_snippets = [
            s for s in snippets if s.content.strip().startswith("const addTodo")
        ]
        assert len(add_todo_snippets) == 1

        # Verify it contains the function body
        add_todo_snippet = add_todo_snippets[0]
        assert "setTodos" in add_todo_snippet.content
        assert "localStorage" in add_todo_snippet.content

    def test_interface_snippet_content(self, tsx_git_files: list[GitFile]) -> None:
        """Interface snippets should contain the full interface definition."""
        slicer = Slicer()
        snippets = slicer.extract_snippets_from_git_files(tsx_git_files, "tsx")

        # Find the standalone Todo interface snippet (not combined with a function)
        interface_snippets = [
            s
            for s in snippets
            if s.content.strip().startswith("interface Todo")
            and "function" not in s.content
        ]
        assert len(interface_snippets) == 1

        interface_snippet = interface_snippets[0]
        assert "id: number" in interface_snippet.content
        assert "text: string" in interface_snippet.content
        assert "completed: boolean" in interface_snippet.content

    def test_parent_function_summarizes_nested_functions(
        self, tsx_git_files: list[GitFile]
    ) -> None:
        """Parent functions should summarize nested functions with '{ ... }'."""
        slicer = Slicer()
        snippets = slicer.extract_snippets_from_git_files(tsx_git_files, "tsx")

        # Find the App function snippet (may have interface prepended)
        app_snippets = [s for s in snippets if "function App()" in s.content]
        assert len(app_snippets) == 1

        app_snippet = app_snippets[0]
        # The nested functions should be summarized, not fully included
        assert "const addTodo = () => { ... };" in app_snippet.content
        assert "const toggleTodo = (id: number) => { ... };" in app_snippet.content
        assert "const deleteTodo = (id: number) => { ... };" in app_snippet.content

    def test_extracts_entry_point_render_calls(
        self, tsx_git_files: list[GitFile]
    ) -> None:
        """Entry point render calls (ReactDOM.createRoot) should be extracted."""
        slicer = Slicer()
        snippets = slicer.extract_snippets_from_git_files(tsx_git_files, "tsx")

        # Find the entry point snippet from main.tsx
        entry_snippets = [s for s in snippets if "ReactDOM.createRoot" in s.content]
        assert len(entry_snippets) == 1

        entry_snippet = entry_snippets[0]
        # Should include the render call with JSX
        assert "<React.StrictMode>" in entry_snippet.content
        assert "<App />" in entry_snippet.content

    def test_function_snippet_includes_referenced_types(
        self, tsx_git_files: list[GitFile]
    ) -> None:
        """Function snippets should include referenced type definitions."""
        slicer = Slicer()
        snippets = slicer.extract_snippets_from_git_files(tsx_git_files, "tsx")

        # Find the App function snippet
        app_snippets = [s for s in snippets if "function App()" in s.content]
        assert len(app_snippets) == 1

        app_snippet = app_snippets[0]
        # The Todo interface should be prepended since App uses Todo[]
        assert "interface Todo {" in app_snippet.content
        # The interface should appear before the function
        interface_pos = app_snippet.content.find("interface Todo")
        function_pos = app_snippet.content.find("function App")
        assert interface_pos < function_pos


class TestTsxComponentSlicing:
    """Test TSX component extraction with props interfaces."""

    @pytest.fixture
    def component_files(self) -> list[GitFile]:
        """Load the component test files."""
        data_dir = Path(__file__).parent / "data" / "tsx"
        components_file = data_dir / "components.tsx"
        return [
            GitFile(
                created_at=datetime.now(tz=UTC),
                blob_sha="sha_components",
                commit_sha="test_commit",
                path=str(components_file),
                mime_type="text/plain",
                size=components_file.stat().st_size,
                extension=".tsx",
            )
        ]

    def test_button_includes_button_props(self, component_files: list[GitFile]) -> None:
        """Button component should include its ButtonProps interface."""
        slicer = Slicer()
        snippets = slicer.extract_snippets_from_git_files(component_files, "tsx")

        # Find the Button function snippet (export keyword not included in snippet)
        button_snippets = [
            s
            for s in snippets
            if "function Button(" in s.content and "interface" in s.content
        ]
        assert len(button_snippets) == 1

        button_snippet = button_snippets[0]
        # ButtonProps should be prepended
        assert "interface ButtonProps {" in button_snippet.content
        assert "label: string" in button_snippet.content
        assert "onClick: () => void" in button_snippet.content
        # Interface should appear before the function
        props_pos = button_snippet.content.find("interface ButtonProps")
        func_pos = button_snippet.content.find("function Button")
        assert props_pos < func_pos

    def test_select_includes_directly_referenced_props(
        self, component_files: list[GitFile]
    ) -> None:
        """Select component should include directly referenced type (SelectProps)."""
        slicer = Slicer()
        snippets = slicer.extract_snippets_from_git_files(component_files, "tsx")

        # Find the Select function snippet
        select_snippets = [
            s
            for s in snippets
            if "function Select(" in s.content and "SelectProps" in s.content
        ]
        assert len(select_snippets) == 1

        select_snippet = select_snippets[0]
        # SelectProps should be prepended (directly referenced in function signature)
        assert "interface SelectProps {" in select_snippet.content
        # SelectOption is referenced within SelectProps, not directly in function
        # So it won't be included (only direct references are resolved)
