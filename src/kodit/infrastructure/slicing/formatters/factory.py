"""Factory for creating language-specific API documentation formatters."""

from kodit.infrastructure.slicing.formatters.c_formatter import CAPIDocFormatter
from kodit.infrastructure.slicing.formatters.cpp_formatter import CppAPIDocFormatter
from kodit.infrastructure.slicing.formatters.csharp_formatter import (
    CSharpAPIDocFormatter,
)
from kodit.infrastructure.slicing.formatters.go_formatter import GoAPIDocFormatter
from kodit.infrastructure.slicing.formatters.java_formatter import JavaAPIDocFormatter
from kodit.infrastructure.slicing.formatters.javascript_formatter import (
    JavaScriptAPIDocFormatter,
)
from kodit.infrastructure.slicing.formatters.python_formatter import (
    PythonAPIDocFormatter,
)
from kodit.infrastructure.slicing.formatters.rust_formatter import RustAPIDocFormatter


def create_formatter(  # noqa: PLR0911
    language: str,
) -> (
    CAPIDocFormatter
    | CppAPIDocFormatter
    | CSharpAPIDocFormatter
    | GoAPIDocFormatter
    | JavaAPIDocFormatter
    | JavaScriptAPIDocFormatter
    | PythonAPIDocFormatter
    | RustAPIDocFormatter
):
    """Create a formatter for the given language.

    Args:
        language: The programming language (e.g., 'python', 'go', 'java')

    Returns:
        A language-specific formatter

    """
    language = language.lower()

    match language:
        case "c":
            return CAPIDocFormatter()
        case "cpp":
            return CppAPIDocFormatter()
        case "csharp":
            return CSharpAPIDocFormatter()
        case "go":
            return GoAPIDocFormatter()
        case "java":
            return JavaAPIDocFormatter()
        case "javascript":
            return JavaScriptAPIDocFormatter()
        case "python":
            return PythonAPIDocFormatter()
        case "rust":
            return RustAPIDocFormatter()
        case _:
            # Default to Go formatter for unknown languages
            return GoAPIDocFormatter()
