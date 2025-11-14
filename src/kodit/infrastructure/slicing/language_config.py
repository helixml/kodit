"""Language-specific configuration value objects."""

from dataclasses import dataclass

from tree_sitter_language_pack import SupportedLanguage


@dataclass(frozen=True)
class LanguageNodeTypes:
    """Node types for a specific language."""

    function_nodes: tuple[str, ...]
    method_nodes: tuple[str, ...]
    call_node: str
    import_nodes: tuple[str, ...]

    @property
    def all_function_nodes(self) -> tuple[str, ...]:
        """All function node types including methods."""
        return self.function_nodes + self.method_nodes


@dataclass(frozen=True)
class LanguageMetadata:
    """Metadata for a specific language."""

    extension: str
    name_field: str | None
    tree_sitter_name: SupportedLanguage
