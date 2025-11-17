"""C# API documentation formatter."""

from kodit.infrastructure.slicing.code_elements import (
    ClassDefinition,
    FunctionDefinition,
    ModuleDefinition,
)
from kodit.infrastructure.slicing.formatters.cpp_formatter import CppAPIDocFormatter


class CSharpAPIDocFormatter(CppAPIDocFormatter):
    """Formats C# code into API documentation markdown."""

    def __init__(self) -> None:
        """Initialize the formatter."""
        super().__init__()
        self._code_fence = "csharp"

    def _format_method(
        self,
        method: FunctionDefinition,
        cls: ClassDefinition,
        module: ModuleDefinition,
    ) -> list[str]:
        """Format a method with C# dot notation."""
        lines = [f"##### {cls.simple_name}.{method.simple_name}", ""]

        parsed_file = self._find_parsed_file_for_function(module, method)
        if parsed_file:
            signature = self._extract_source(parsed_file, method.node)
            lines.append(f"```{self._code_fence}")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        if method.docstring:
            lines.append(method.docstring)
            lines.append("")

        return lines
