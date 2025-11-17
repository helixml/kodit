"""Factory for creating language-specific API documentation formatters."""

from kodit.infrastructure.slicing.formatters.template_formatter import (
    TemplateAPIDocFormatter,
)


def create_formatter(language: str) -> TemplateAPIDocFormatter:
    """Create a formatter for the given language.

    Args:
        language: The programming language (e.g., 'python', 'go', 'java')

    Returns:
        A template-based formatter for the language

    """
    return TemplateAPIDocFormatter(language)
