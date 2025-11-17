"""Code elements extracted from parsed files."""

from dataclasses import dataclass
from pathlib import Path

from tree_sitter import Node, Tree

from kodit.domain.entities.git import GitFile


@dataclass
class ParsedFile:
    """Result of parsing a single file with tree-sitter."""

    path: Path
    git_file: GitFile
    tree: Tree
    source_code: bytes


@dataclass
class FunctionDefinition:
    """Information about a function or method definition."""

    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str
    simple_name: str
    is_public: bool
    is_method: bool
    docstring: str | None
    parameters: list[str]
    return_type: str | None


@dataclass
class ClassDefinition:
    """Information about a class definition."""

    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str
    simple_name: str
    is_public: bool
    docstring: str | None
    methods: list[FunctionDefinition]
    base_classes: list[str]
    constructor_params: list[str]


@dataclass
class TypeDefinition:
    """Information about a type definition (enum, interface, type alias)."""

    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str
    simple_name: str
    is_public: bool
    docstring: str | None
    kind: str
    constructor_params: list[str]


@dataclass
class ModuleDefinition:
    """All definitions in a module, grouped by language conventions."""

    module_path: str
    files: list[ParsedFile]
    functions: list[FunctionDefinition]
    classes: list[ClassDefinition]
    types: list[TypeDefinition]
    constants: list[tuple[str, Node]]
    module_docstring: str | None


@dataclass
class FunctionInfo:
    """Information about a function definition."""

    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str
