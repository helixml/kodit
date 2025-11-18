"""Base class and language-specific analyzers."""

from abc import ABC, abstractmethod
from pathlib import Path

from tree_sitter import Node

from kodit.infrastructure.slicing.code_elements import (
    ClassDefinition,
    FunctionDefinition,
    ParsedFile,
    TypeDefinition,
)
from kodit.infrastructure.slicing.language_config import (
    LanguageMetadata,
    LanguageNodeTypes,
)


class LanguageAnalyzer(ABC):
    """Base class for language-specific analysis."""

    @abstractmethod
    def node_types(self) -> LanguageNodeTypes:
        """Node types for this language."""

    @abstractmethod
    def metadata(self) -> LanguageMetadata:
        """Metadata for this language."""

    @abstractmethod
    def extract_function_name(self, node: Node) -> str | None:
        """Extract function name from a function definition node."""

    @abstractmethod
    def is_public(self, node: Node, name: str) -> bool:
        """Determine if a definition is public."""

    @abstractmethod
    def extract_docstring(self, node: Node) -> str | None:
        """Extract documentation comment for a definition."""

    @abstractmethod
    def extract_module_path(self, parsed: ParsedFile) -> str:
        """Extract module/package path based on language conventions."""

    def is_method(self, node: Node) -> bool:  # noqa: ARG002
        """Check if a function node represents a method."""
        return False

    def extract_module_docstring(
        self,
        module_files: list[ParsedFile],  # noqa: ARG002
    ) -> str | None:
        """Extract module-level documentation."""
        return None

    def extract_classes(
        self,
        parsed: ParsedFile,  # noqa: ARG002
        *,
        include_private: bool,  # noqa: ARG002
    ) -> list[ClassDefinition]:
        """Extract class definitions with their methods."""
        return []

    def extract_types(
        self,
        parsed: ParsedFile,  # noqa: ARG002
        *,
        include_private: bool,  # noqa: ARG002
    ) -> list[TypeDefinition]:
        """Extract type definitions."""
        return []

    def extract_constructor_params(self, node: Node) -> list[str]:  # noqa: ARG002
        """Extract constructor parameters from a class or type node."""
        return []


class PythonAnalyzer(LanguageAnalyzer):
    """Python-specific analyzer."""

    def node_types(self) -> LanguageNodeTypes:
        """Node types for Python."""
        return LanguageNodeTypes(
            function_nodes=("function_definition",),
            method_nodes=(),
            call_node="call",
            import_nodes=("import_statement", "import_from_statement"),
        )

    def metadata(self) -> LanguageMetadata:
        """Metadata for Python."""
        return LanguageMetadata(
            extension=".py",
            name_field=None,
            tree_sitter_name="python",
        )

    def extract_function_name(self, node: Node) -> str | None:
        """Extract function name from a function definition node."""
        for child in node.children:
            if child.type == "identifier" and child.text is not None:
                return child.text.decode("utf-8")
        return None

    def is_public(self, node: Node, name: str) -> bool:  # noqa: ARG002
        """Determine if a definition is public."""
        return not name.startswith("_")

    def extract_docstring(self, node: Node) -> str | None:  # noqa: C901
        """Extract Python docstring."""
        body_node = None

        if node.type in {"function_definition", "class_definition"}:
            for child in node.children:
                if child.type == "block":
                    body_node = child
                    break
        elif node.type == "module":
            body_node = node

        if not body_node:
            return None

        for child in body_node.children:
            if child.type == "expression_statement":
                for expr_child in child.children:
                    if expr_child.type == "string" and expr_child.text:
                        docstring_bytes = expr_child.text
                        try:
                            docstring_text = docstring_bytes.decode("utf-8")
                            docstring_text = docstring_text.strip()
                            for quote in ['"""', "'''", '"', "'"]:
                                starts = docstring_text.startswith(quote)
                                ends = docstring_text.endswith(quote)
                                if starts and ends:
                                    quote_len = len(quote)
                                    docstring_text = docstring_text[
                                        quote_len:-quote_len
                                    ]
                                    break
                            return docstring_text.strip()
                        except UnicodeDecodeError:
                            return None
                break

        return None

    def extract_module_path(self, parsed: ParsedFile) -> str:
        """Extract Python module path from file path structure."""
        file_path = Path(parsed.git_file.path)
        clean_path = self._clean_path_for_module(file_path)

        if clean_path.name == "__init__.py":
            module_parts = list(clean_path.parts[:-1])
        else:
            module_parts = list(clean_path.parts[:-1])
            module_parts.append(clean_path.stem)

        module_path = ".".join(p for p in module_parts if p and p != ".")

        if not module_path and clean_path.name == "__init__.py":
            parent_dir = file_path.parent.name
            if parent_dir and parent_dir != ".":
                return parent_dir
            return ""

        return module_path if module_path else clean_path.stem

    def extract_module_docstring(self, module_files: list[ParsedFile]) -> str | None:
        """Extract Python module-level documentation."""
        for parsed in module_files:
            if parsed.path.name == "__init__.py":
                return self.extract_docstring(parsed.tree.root_node)
        if module_files:
            return self.extract_docstring(module_files[0].tree.root_node)
        return None

    def _clean_path_for_module(self, file_path: Path) -> Path:
        """Clean a file path to extract a reasonable module path."""
        parts = list(file_path.parts)

        if not file_path.is_absolute():
            return file_path

        common_roots = {"src", "lib", "pkg", "internal", "app"}
        for i, part in enumerate(parts):
            if part in common_roots:
                if i + 1 < len(parts):
                    return Path(*parts[i + 1 :])
                return file_path

        if len(parts) >= 2:
            return Path(*parts[-2:])

        return file_path

    def extract_classes(
        self, parsed: ParsedFile, *, include_private: bool
    ) -> list[ClassDefinition]:
        """Extract Python class definitions."""
        classes = []

        for node in self._walk_tree(parsed.tree.root_node):
            if node.type == "class_definition":
                class_name = None
                for child in node.children:
                    if child.type == "identifier" and child.text:
                        class_name = child.text.decode("utf-8")
                        break

                if not class_name:
                    continue

                is_public = self.is_public(node, class_name)
                if not include_private and not is_public:
                    continue

                docstring = self.extract_docstring(node)
                methods = self._extract_class_methods(node, parsed, include_private)
                base_classes = self._extract_base_classes(node)

                qualified_name = f"{parsed.path.stem}.{class_name}"
                span = (node.start_byte, node.end_byte)

                constructor_params = self.extract_constructor_params(node)

                classes.append(
                    ClassDefinition(
                        file=parsed.path,
                        node=node,
                        span=span,
                        qualified_name=qualified_name,
                        simple_name=class_name,
                        is_public=is_public,
                        docstring=docstring,
                        methods=methods,
                        base_classes=base_classes,
                        constructor_params=constructor_params,
                    )
                )

        return classes

    def _walk_tree(self, node: Node):  # noqa: ANN202
        """Walk the AST tree, yielding all nodes."""
        queue = [node]
        visited: set[int] = set()

        while queue:
            current = queue.pop(0)
            node_id = id(current)
            if node_id in visited:
                continue
            visited.add(node_id)

            yield current
            queue.extend(current.children)

    def _extract_class_methods(  # noqa: ANN202, C901
        self,
        class_node: Node,
        parsed: ParsedFile,
        include_private: bool,  # noqa: FBT001
    ):
        """Extract methods from a class definition."""
        methods = []

        for child in class_node.children:
            if child.type == "block":
                for block_child in child.children:
                    if block_child.type == "function_definition":
                        method_name = None
                        for func_child in block_child.children:
                            if func_child.type == "identifier" and func_child.text:
                                method_name = func_child.text.decode("utf-8")
                                break

                        if not method_name:
                            continue

                        is_public = self.is_public(block_child, method_name)
                        if not include_private and not is_public:
                            continue

                        docstring = self.extract_docstring(block_child)
                        parameters = self._extract_parameters_from_function(block_child)

                        class_name = None
                        for class_child in class_node.children:
                            if class_child.type == "identifier" and class_child.text:
                                class_name = class_child.text.decode("utf-8")
                                break

                        qualified_name = (
                            f"{parsed.path.stem}.{class_name}.{method_name}"
                        )
                        span = (block_child.start_byte, block_child.end_byte)

                        method = FunctionDefinition(
                            file=parsed.path,
                            node=block_child,
                            span=span,
                            qualified_name=qualified_name,
                            simple_name=method_name,
                            is_public=is_public,
                            is_method=True,
                            docstring=docstring,
                            parameters=parameters,
                            return_type=None,
                        )
                        methods.append(method)

        return methods

    def _extract_base_classes(self, class_node: Node) -> list[str]:
        """Extract base class names from a class definition."""
        base_classes: list[str] = []

        for child in class_node.children:
            if child.type == "argument_list":
                base_classes.extend(
                    arg_child.text.decode("utf-8")
                    for arg_child in child.children
                    if arg_child.type == "identifier" and arg_child.text
                )

        return base_classes

    def extract_constructor_params(self, node: Node) -> list[str]:
        """Extract constructor parameters from Python __init__ method."""
        for child in self._walk_tree(node):
            if child.type == "function_definition":
                func_name = self.extract_function_name(child)
                if func_name == "__init__":
                    return self._extract_parameters_from_function(child)
        return []

    def _extract_parameters_from_function(self, func_node: Node) -> list[str]:
        """Extract parameter names and types from a function definition."""
        params: list[str] = []
        for child in func_node.children:
            if child.type == "parameters":
                for param_child in child.children:
                    if param_child.type in {
                        "identifier",
                        "typed_parameter",
                        "typed_default_parameter",
                        "default_parameter",
                    }:
                        param_text = self._extract_param_text(param_child)
                        if param_text and param_text != "self":
                            params.append(param_text)
        return params

    def _extract_param_text(self, param_node: Node) -> str | None:
        """Extract parameter text including type annotation."""
        if param_node.text:
            try:
                return param_node.text.decode("utf-8")
            except UnicodeDecodeError:
                return None
        return None


class GoAnalyzer(LanguageAnalyzer):
    """Go-specific analyzer."""

    def node_types(self) -> LanguageNodeTypes:
        """Node types for Go."""
        return LanguageNodeTypes(
            function_nodes=("function_declaration",),
            method_nodes=("method_declaration",),
            call_node="call_expression",
            import_nodes=("import_declaration",),
        )

    def metadata(self) -> LanguageMetadata:
        """Metadata for Go."""
        return LanguageMetadata(
            extension=".go",
            name_field=None,
            tree_sitter_name="go",
        )

    def extract_function_name(self, node: Node) -> str | None:
        """Extract function name from a function definition node."""
        if node.type == "method_declaration":
            for child in node.children:
                if child.type == "field_identifier" and child.text is not None:
                    return child.text.decode("utf-8")
        else:
            for child in node.children:
                if child.type == "identifier" and child.text is not None:
                    return child.text.decode("utf-8")
        return None

    def is_public(self, node: Node, name: str) -> bool:  # noqa: ARG002
        """Determine if a definition is public."""
        return name[0].isupper() if name else False

    def is_method(self, node: Node) -> bool:
        """Check if a function node represents a method."""
        return node.type == "method_declaration"

    def extract_docstring(self, node: Node) -> str | None:
        """Extract comment before a Go function or method declaration."""
        parent = node.parent
        if not parent:
            return None

        func_index = None
        for i, child in enumerate(parent.children):
            if child == node:
                func_index = i
                break

        if func_index is None or func_index == 0:
            return None

        prev_sibling = parent.children[func_index - 1]
        if prev_sibling.type == "comment" and prev_sibling.text:
            comment_text = prev_sibling.text.decode("utf-8")
            return comment_text.lstrip("/").strip()

        return None

    def extract_module_path(self, parsed: ParsedFile) -> str:
        """Extract full Go package path using directory structure."""
        package_name = self._extract_package_name(parsed)

        file_path = Path(parsed.git_file.path)
        clean_path = self._clean_path_for_module(file_path)
        dir_path = clean_path.parent

        if str(dir_path) != ".":
            dir_str = str(dir_path).replace("\\", "/")
            if dir_str.endswith("/" + package_name) or dir_str == package_name:
                return dir_str
            return f"{dir_str}/{package_name}"
        return package_name

    def _extract_package_name(self, parsed: ParsedFile) -> str:
        """Extract Go package name from package declaration."""
        root = parsed.tree.root_node
        for child in root.children:
            if child.type == "package_clause":
                for package_child in child.children:
                    if (
                        package_child.type == "package_identifier"
                        and package_child.text
                    ):
                        return package_child.text.decode("utf-8")
        return parsed.path.stem

    def _clean_path_for_module(self, file_path: Path) -> Path:
        """Clean a file path to extract a reasonable module path."""
        parts = list(file_path.parts)

        if not file_path.is_absolute():
            return file_path

        common_roots = {"src", "lib", "pkg", "internal", "app"}
        for i, part in enumerate(parts):
            if part in common_roots:
                if i + 1 < len(parts):
                    return Path(*parts[i + 1 :])
                return file_path

        if len(parts) >= 2:
            return Path(*parts[-2:])

        return file_path

    def extract_types(
        self, parsed: ParsedFile, *, include_private: bool
    ) -> list[TypeDefinition]:
        """Extract Go type definitions."""
        types = []

        for node in self._walk_tree(parsed.tree.root_node):
            if node.type == "type_declaration":
                for child in node.children:
                    if child.type == "type_spec":
                        type_def = self._extract_type_from_spec(
                            child, parsed, include_private=include_private
                        )
                        if type_def:
                            types.append(type_def)

        return types

    def _walk_tree(self, node: Node):  # noqa: ANN202
        """Walk the AST tree, yielding all nodes."""
        queue = [node]
        visited: set[int] = set()

        while queue:
            current = queue.pop(0)
            node_id = id(current)
            if node_id in visited:
                continue
            visited.add(node_id)

            yield current
            queue.extend(current.children)

    def _extract_type_from_spec(
        self, type_spec_node: Node, parsed: ParsedFile, *, include_private: bool
    ) -> TypeDefinition | None:
        """Extract a single Go type definition from a type_spec node."""
        type_name = None
        type_kind = "type"

        for child in type_spec_node.children:
            if child.type == "type_identifier" and child.text:
                type_name = child.text.decode("utf-8")
            elif child.type == "struct_type":
                type_kind = "struct"
            elif child.type == "interface_type":
                type_kind = "interface"
            elif child.type in ["slice_type", "array_type", "pointer_type"]:
                type_kind = "alias"
            elif child.type == "map_type":
                type_kind = "map"

        if not type_name:
            return None

        is_public = self.is_public(type_spec_node, type_name)
        if not include_private and not is_public:
            return None

        parent_node = type_spec_node.parent
        if not parent_node:
            return None

        docstring = self._extract_type_comment(parent_node)

        qualified_name = f"{parsed.path.stem}.{type_name}"
        span = (parent_node.start_byte, parent_node.end_byte)
        constructor_params = self.extract_constructor_params(type_spec_node)

        return TypeDefinition(
            file=parsed.path,
            node=parent_node,
            span=span,
            qualified_name=qualified_name,
            simple_name=type_name,
            is_public=is_public,
            docstring=docstring,
            kind=type_kind,
            constructor_params=constructor_params,
        )

    def _extract_type_comment(self, type_decl_node: Node) -> str | None:
        """Extract comment before a Go type declaration."""
        parent = type_decl_node.parent
        if not parent:
            return None

        type_decl_index = None
        for i, child in enumerate(parent.children):
            if child == type_decl_node:
                type_decl_index = i
                break

        if type_decl_index is None or type_decl_index == 0:
            return None

        prev_sibling = parent.children[type_decl_index - 1]
        if prev_sibling.type == "comment" and prev_sibling.text:
            comment_text = prev_sibling.text.decode("utf-8")
            return comment_text.lstrip("/").strip()

        return None

    def extract_constructor_params(self, node: Node) -> list[str]:
        """Extract constructor parameters from Go struct fields."""
        params: list[str] = []

        for child in node.children:
            if child.type == "struct_type":
                for struct_child in child.children:
                    if struct_child.type == "field_declaration_list":
                        for field in struct_child.children:
                            if field.type == "field_declaration":
                                field_text = self._extract_go_field_signature(field)
                                if field_text:
                                    params.append(field_text)
        return params

    def _extract_go_field_signature(self, field_node: Node) -> str | None:
        """Extract field signature from a Go field declaration."""
        if field_node.text:
            try:
                return field_node.text.decode("utf-8").strip()
            except UnicodeDecodeError:
                return None
        return None


class JavaAnalyzer(LanguageAnalyzer):
    """Java-specific analyzer."""

    def node_types(self) -> LanguageNodeTypes:
        """Node types for Java."""
        return LanguageNodeTypes(
            function_nodes=("method_declaration",),
            method_nodes=(),
            call_node="method_invocation",
            import_nodes=("import_declaration",),
        )

    def metadata(self) -> LanguageMetadata:
        """Metadata for Java."""
        return LanguageMetadata(
            extension=".java",
            name_field=None,
            tree_sitter_name="java",
        )

    def extract_function_name(self, node: Node) -> str | None:
        """Extract function name from a function definition node."""
        for child in node.children:
            if child.type == "identifier" and child.text is not None:
                return child.text.decode("utf-8")
        return None

    def is_public(self, node: Node, name: str) -> bool:  # noqa: ARG002
        """Determine if a definition is public."""
        return True

    def extract_docstring(self, node: Node) -> str | None:  # noqa: ARG002
        """Extract documentation comment for a definition."""
        return None

    def extract_module_path(self, parsed: ParsedFile) -> str:
        """Extract Java package name from package declaration."""
        root = parsed.tree.root_node
        for child in root.children:
            if child.type == "package_declaration":
                for package_child in child.children:
                    if package_child.type == "scoped_identifier" and package_child.text:
                        return package_child.text.decode("utf-8")
                    if package_child.type == "identifier" and package_child.text:
                        return package_child.text.decode("utf-8")
        return parsed.path.stem


class CAnalyzer(LanguageAnalyzer):
    """C-specific analyzer."""

    def node_types(self) -> LanguageNodeTypes:
        """Node types for C."""
        return LanguageNodeTypes(
            function_nodes=("function_definition",),
            method_nodes=(),
            call_node="call_expression",
            import_nodes=("preproc_include",),
        )

    def metadata(self) -> LanguageMetadata:
        """Metadata for C."""
        return LanguageMetadata(
            extension=".c",
            name_field="declarator",
            tree_sitter_name="c",
        )

    def extract_function_name(self, node: Node) -> str | None:
        """Extract function name from a C function definition."""
        declarator = node.child_by_field_name("declarator")
        if not declarator:
            return None

        if declarator.type == "function_declarator":
            for child in declarator.children:
                if child.type == "identifier" and child.text is not None:
                    return child.text.decode("utf-8")
        elif declarator.type == "identifier" and declarator.text is not None:
            return declarator.text.decode("utf-8")
        return None

    def is_public(self, node: Node, name: str) -> bool:  # noqa: ARG002
        """Determine if a definition is public."""
        return True

    def extract_docstring(self, node: Node) -> str | None:  # noqa: ARG002
        """Extract documentation comment for a definition."""
        return None

    def extract_module_path(self, parsed: ParsedFile) -> str:
        """Extract module path based on file path."""
        file_path = Path(parsed.git_file.path)
        clean_path = self._clean_path_for_module(file_path)
        module_parts = list(clean_path.parts[:-1])
        module_parts.append(clean_path.stem)
        module_path = ".".join(p for p in module_parts if p and p != ".")
        return module_path if module_path else clean_path.stem

    def _clean_path_for_module(self, file_path: Path) -> Path:
        """Clean a file path to extract a reasonable module path."""
        parts = list(file_path.parts)

        if not file_path.is_absolute():
            return file_path

        common_roots = {"src", "lib", "pkg", "internal", "app"}
        for i, part in enumerate(parts):
            if part in common_roots:
                if i + 1 < len(parts):
                    return Path(*parts[i + 1 :])
                return file_path

        if len(parts) >= 2:
            return Path(*parts[-2:])

        return file_path


class CppAnalyzer(CAnalyzer):
    """C++-specific analyzer."""

    def node_types(self) -> LanguageNodeTypes:
        """Node types for C++."""
        return LanguageNodeTypes(
            function_nodes=("function_definition",),
            method_nodes=(),
            call_node="call_expression",
            import_nodes=("preproc_include", "using_declaration"),
        )

    def metadata(self) -> LanguageMetadata:
        """Metadata for C++."""
        return LanguageMetadata(
            extension=".cpp",
            name_field="declarator",
            tree_sitter_name="cpp",
        )


class RustAnalyzer(LanguageAnalyzer):
    """Rust-specific analyzer."""

    def node_types(self) -> LanguageNodeTypes:
        """Node types for Rust."""
        return LanguageNodeTypes(
            function_nodes=("function_item",),
            method_nodes=(),
            call_node="call_expression",
            import_nodes=("use_declaration", "extern_crate_declaration"),
        )

    def metadata(self) -> LanguageMetadata:
        """Metadata for Rust."""
        return LanguageMetadata(
            extension=".rs",
            name_field="name",
            tree_sitter_name="rust",
        )

    def extract_function_name(self, node: Node) -> str | None:
        """Extract function name from a Rust function definition."""
        name_node = node.child_by_field_name("name")
        if name_node and name_node.type == "identifier" and name_node.text is not None:
            return name_node.text.decode("utf-8")
        return None

    def is_public(self, node: Node, name: str) -> bool:  # noqa: ARG002
        """Determine if a definition is public."""
        return True

    def extract_docstring(self, node: Node) -> str | None:  # noqa: ARG002
        """Extract documentation comment for a definition."""
        return None

    def extract_module_path(self, parsed: ParsedFile) -> str:
        """Extract module path based on file path."""
        file_path = Path(parsed.git_file.path)
        clean_path = self._clean_path_for_module(file_path)
        module_parts = list(clean_path.parts[:-1])
        module_parts.append(clean_path.stem)
        module_path = ".".join(p for p in module_parts if p and p != ".")
        return module_path if module_path else clean_path.stem

    def _clean_path_for_module(self, file_path: Path) -> Path:
        """Clean a file path to extract a reasonable module path."""
        parts = list(file_path.parts)

        if not file_path.is_absolute():
            return file_path

        common_roots = {"src", "lib", "pkg", "internal", "app"}
        for i, part in enumerate(parts):
            if part in common_roots:
                if i + 1 < len(parts):
                    return Path(*parts[i + 1 :])
                return file_path

        if len(parts) >= 2:
            return Path(*parts[-2:])

        return file_path


class JavaScriptAnalyzer(LanguageAnalyzer):
    """JavaScript-specific analyzer."""

    def node_types(self) -> LanguageNodeTypes:
        """Node types for JavaScript."""
        return LanguageNodeTypes(
            function_nodes=(
                "function_declaration",
                "function_expression",
                "arrow_function",
            ),
            method_nodes=(),
            call_node="call_expression",
            import_nodes=("import_statement", "import_declaration"),
        )

    def metadata(self) -> LanguageMetadata:
        """Metadata for JavaScript."""
        return LanguageMetadata(
            extension=".js",
            name_field=None,
            tree_sitter_name="javascript",
        )

    def extract_function_name(self, node: Node) -> str | None:
        """Extract function name from a function definition node."""
        for child in node.children:
            if child.type == "identifier" and child.text is not None:
                return child.text.decode("utf-8")
        return None

    def is_public(self, node: Node, name: str) -> bool:  # noqa: ARG002
        """Determine if a definition is public."""
        return True

    def extract_docstring(self, node: Node) -> str | None:  # noqa: ARG002
        """Extract documentation comment for a definition."""
        return None

    def extract_module_path(self, parsed: ParsedFile) -> str:
        """Extract module path based on file path."""
        file_path = Path(parsed.git_file.path)
        clean_path = self._clean_path_for_module(file_path)
        module_parts = list(clean_path.parts[:-1])
        module_parts.append(clean_path.stem)
        module_path = ".".join(p for p in module_parts if p and p != ".")
        return module_path if module_path else clean_path.stem

    def _clean_path_for_module(self, file_path: Path) -> Path:
        """Clean a file path to extract a reasonable module path."""
        parts = list(file_path.parts)

        if not file_path.is_absolute():
            return file_path

        common_roots = {"src", "lib", "pkg", "internal", "app"}
        for i, part in enumerate(parts):
            if part in common_roots:
                if i + 1 < len(parts):
                    return Path(*parts[i + 1 :])
                return file_path

        if len(parts) >= 2:
            return Path(*parts[-2:])

        return file_path


class TypeScriptAnalyzer(JavaScriptAnalyzer):
    """TypeScript-specific analyzer."""

    def metadata(self) -> LanguageMetadata:
        """Metadata for TypeScript."""
        return LanguageMetadata(
            extension=".ts",
            name_field=None,
            tree_sitter_name="typescript",
        )


class CSharpAnalyzer(LanguageAnalyzer):
    """C#-specific analyzer."""

    def node_types(self) -> LanguageNodeTypes:
        """Node types for C#."""
        return LanguageNodeTypes(
            function_nodes=("method_declaration",),
            method_nodes=("constructor_declaration",),
            call_node="invocation_expression",
            import_nodes=("using_directive",),
        )

    def metadata(self) -> LanguageMetadata:
        """Metadata for C#."""
        return LanguageMetadata(
            extension=".cs",
            name_field=None,
            tree_sitter_name="csharp",
        )

    def extract_function_name(self, node: Node) -> str | None:
        """Extract function name from a function definition node."""
        for child in node.children:
            if child.type == "identifier" and child.text is not None:
                return child.text.decode("utf-8")
        return None

    def is_public(self, node: Node, name: str) -> bool:  # noqa: ARG002
        """Determine if a definition is public."""
        return True

    def extract_docstring(self, node: Node) -> str | None:  # noqa: ARG002
        """Extract documentation comment for a definition."""
        return None

    def extract_module_path(self, parsed: ParsedFile) -> str:
        """Extract module path based on file path."""
        file_path = Path(parsed.git_file.path)
        clean_path = self._clean_path_for_module(file_path)
        module_parts = list(clean_path.parts[:-1])
        module_parts.append(clean_path.stem)
        module_path = ".".join(p for p in module_parts if p and p != ".")
        return module_path if module_path else clean_path.stem

    def _clean_path_for_module(self, file_path: Path) -> Path:
        """Clean a file path to extract a reasonable module path."""
        parts = list(file_path.parts)

        if not file_path.is_absolute():
            return file_path

        common_roots = {"src", "lib", "pkg", "internal", "app"}
        for i, part in enumerate(parts):
            if part in common_roots:
                if i + 1 < len(parts):
                    return Path(*parts[i + 1 :])
                return file_path

        if len(parts) >= 2:
            return Path(*parts[-2:])

        return file_path


def language_analyzer_factory(language: str) -> LanguageAnalyzer:
    """Create language-specific analyzer."""
    language_lower = language.lower()

    analyzers = {
        "python": PythonAnalyzer(),
        "go": GoAnalyzer(),
        "java": JavaAnalyzer(),
        "c": CAnalyzer(),
        "cpp": CppAnalyzer(),
        "c++": CppAnalyzer(),
        "rust": RustAnalyzer(),
        "javascript": JavaScriptAnalyzer(),
        "js": JavaScriptAnalyzer(),
        "typescript": TypeScriptAnalyzer(),
        "ts": TypeScriptAnalyzer(),
        "csharp": CSharpAnalyzer(),
        "c#": CSharpAnalyzer(),
        "cs": CSharpAnalyzer(),
    }

    analyzer = analyzers.get(language_lower)
    if not analyzer:
        raise ValueError(f"Unsupported language: {language}")

    return analyzer
