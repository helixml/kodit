"""Dump Pydantic Settings configuration to markdown."""

import argparse
import inspect
from pathlib import Path
from typing import Any, get_args, get_origin

import jinja2
from pydantic import BaseModel
from pydantic_settings import BaseSettings


def get_model_info(model_class: type[BaseModel]) -> dict[str, Any]:
    """Extract information from a Pydantic model."""
    model_info: dict[str, Any] = {
        "description": inspect.getdoc(model_class) or "",
        "env_vars": [],
    }

    # Extract environment variables if it's a BaseSettings class
    if issubclass(model_class, BaseSettings):
        model_info["env_vars"] = _extract_env_vars(model_class)

    return model_info


def _format_type(type_annotation: Any) -> str:  # noqa: C901, PLR0911
    """Format type annotation for display."""
    if hasattr(type_annotation, "__name__"):
        return type_annotation.__name__

    origin = get_origin(type_annotation)
    args = get_args(type_annotation)

    if origin is None:
        return str(type_annotation)

    if origin is list:
        if args:
            return f"list[{_format_type(args[0])}]"
        return "list"

    if origin is dict:
        if len(args) >= 2:
            return f"dict[{_format_type(args[0])}, {_format_type(args[1])}]"
        return "dict"

    if origin is type(None) or origin is type:
        return str(type_annotation)

    # Handle Union types (including Optional)
    has_union_name = hasattr(origin, "__name__") and origin.__name__ in (
        "UnionType",
        "_UnionGenericAlias",
    )
    is_union = has_union_name or str(origin).startswith("typing.Union")
    if is_union:
        if len(args) == 2 and type(None) in args:
            # Optional type
            non_none_type = next(arg for arg in args if arg is not type(None))
            return f"`{_format_type(non_none_type)} | None`"
        # Union type
        type_names = [_format_type(arg) for arg in args]
        return f"`{' | '.join(type_names)}`"

    if origin and hasattr(origin, "__name__"):
        if args:
            arg_names = [_format_type(arg) for arg in args]
            return f"{origin.__name__}[{', '.join(arg_names)}]"
        return origin.__name__

    return str(type_annotation)


def _extract_env_vars(
    settings_class: type[BaseSettings], prefix: str = ""
) -> list[dict[str, str]]:
    """Extract environment variable names from a BaseSettings class with inheritance."""
    env_vars: list[dict[str, str]] = []

    # Get the model config
    config = getattr(settings_class, "model_config", None)
    if config:
        env_prefix = getattr(config, "env_prefix", "")
        env_nested_delimiter = getattr(config, "env_nested_delimiter", "_")
    else:
        env_prefix = ""
        env_nested_delimiter = "_"

    # Generate env vars for each field
    for field_name, field_info in settings_class.model_fields.items():
        current_prefix = f"{prefix}{env_prefix}{field_name.upper()}"

        # Extract description and default
        description = field_info.description or ""

        # Extract default value
        from pydantic_core import PydanticUndefined
        if field_info.default is not PydanticUndefined:
            if field_info.default is None:
                default_value = "None"
            else:
                default_value = _format_default_value(field_info.default, field_name)
        elif field_info.default_factory is not None:
            try:
                factory_result = field_info.default_factory()  # type: ignore[call-arg]
                default_value = _format_default_value(factory_result, field_name)
            except (TypeError, ValueError, AttributeError):
                default_value = f"{field_info.default_factory.__name__}()"
        else:
            default_value = "Required"

        # Extract type
        field_type = field_info.annotation
        type_name = _format_type(field_type) if field_type else "Any"

        env_vars.append({
            "name": current_prefix,
            "type": type_name,
            "default": default_value,
            "description": description,
        })

        # Handle nested models (inheritance)
        if field_info.annotation:
            nested_vars = _extract_nested_env_vars(
                field_info.annotation, current_prefix + env_nested_delimiter
            )
            env_vars.extend(nested_vars)

    return env_vars


def _extract_nested_env_vars(  # noqa: C901, PLR0912
    model_class: type, prefix: str
) -> list[dict[str, str]]:
    """Extract environment variables from nested Pydantic models."""
    env_vars: list[dict[str, str]] = []

    # Handle Optional types
    origin = get_origin(model_class)
    args = get_args(model_class)

    if origin and args:
        # Check if it's Optional[SomeModel]
        if len(args) == 2 and type(None) in args:
            actual_model = next(arg for arg in args if arg is not type(None))
            if _is_pydantic_model(actual_model):
                model_class = actual_model
        # Check if it's Union but not Optional
        elif (
            (
                hasattr(origin, "__name__")
                and origin.__name__ in ("Union", "UnionType", "_UnionGenericAlias")
            )
            or str(origin) == "<class 'types.UnionType'>"
        ):
            # For Union types, we'll use the first non-None type
            for arg in args:
                if arg is not type(None) and _is_pydantic_model(arg):
                    model_class = arg
                    break
            else:
                return env_vars

    if not _is_pydantic_model(model_class):
        return env_vars

    if not hasattr(model_class, "model_fields"):
        return env_vars

    for field_name, field_info in model_class.model_fields.items():  # type: ignore[attr-defined]
        nested_var_name = f"{prefix}{field_name.upper()}"

        # Extract field information
        description = field_info.description or ""

        # Extract default value
        from pydantic_core import PydanticUndefined
        if field_info.default is not PydanticUndefined:
            if field_info.default is None:
                default_value = "None"
            else:
                default_value = _format_default_value(field_info.default, field_name)
        elif field_info.default_factory is not None:
            try:
                factory_result = field_info.default_factory()  # type: ignore[call-arg]
                default_value = _format_default_value(factory_result, field_name)
            except (TypeError, ValueError, AttributeError):
                default_value = f"{field_info.default_factory.__name__}()"
        else:
            default_value = "Required"

        # Extract type
        field_type = field_info.annotation
        type_name = _format_type(field_type) if field_type else "Any"

        env_vars.append({
            "name": nested_var_name,
            "type": type_name,
            "default": default_value,
            "description": description,
        })

        # Handle further nesting
        if _is_pydantic_model(field_info.annotation):
            further_nested = _extract_nested_env_vars(
                field_info.annotation, nested_var_name + "_"
            )
            env_vars.extend(further_nested)

    return env_vars


def _is_pydantic_model(type_annotation: Any) -> bool:
    """Check if a type annotation represents a Pydantic model."""
    try:
        return (
            hasattr(type_annotation, "model_fields")
            and hasattr(type_annotation, "__mro__")
            and BaseModel in type_annotation.__mro__
        )
    except (TypeError, AttributeError):
        return False


def _format_default_value(value: Any, field_name: str) -> str:
    """Format default values for documentation, handling special cases."""
    from pathlib import Path

    # Handle Path objects that contain user home directory
    if isinstance(value, Path):
        path_str = str(value)
        # Replace actual home directory with generic placeholder
        home_dir = str(Path.home())
        if path_str.startswith(home_dir):
            return path_str.replace(home_dir, "~")

    # Handle special field names that we know represent dynamic defaults
    if field_name.lower() == "data_dir" and isinstance(value, Path):
        return "~/.kodit"

    return str(value)


def _lint_markdown(content: str) -> str:
    """Apply basic markdown linting rules to clean up formatting."""
    import re

    lines = content.split("\n")
    cleaned_lines: list[str] = []
    previous_line_empty = False
    in_table = False

    for original_line in lines:
        # Remove trailing whitespace
        cleaned_line = original_line.rstrip()

        # Check if current line is empty
        current_line_empty = len(cleaned_line) == 0

        # Check if we're in a table
        is_table_line = cleaned_line.startswith("|")

        # Handle table state
        if is_table_line and not in_table:
            # Starting a table - ensure blank line before
            if cleaned_lines and not previous_line_empty:
                cleaned_lines.append("")
            in_table = True
        elif not is_table_line and in_table:
            # Ending a table
            in_table = False

        # Skip multiple consecutive empty lines (keep only one)
        # But don't skip empty lines in tables
        if current_line_empty and previous_line_empty and not in_table:
            continue

        # Don't add empty lines within tables
        if current_line_empty and in_table:
            continue

        cleaned_lines.append(cleaned_line)
        previous_line_empty = current_line_empty

    # Join lines back together
    result = "\n".join(cleaned_lines)

    # Ensure file ends with exactly one newline
    result = result.rstrip("\n") + "\n"

    # Fix spacing around headers (ensure one blank line before, no blank line after)
    result = re.sub(r"\n+(?=^##)", "\n\n", result, flags=re.MULTILINE)

    # Clean up any remaining multiple newlines (max 2 consecutive)
    return re.sub(r"\n{3,}", "\n\n", result)




def extract_all_models() -> dict[str, Any]:
    """Extract all Pydantic models from config module."""
    from kodit import config

    models = {}

    # Get all classes from the config module
    for name, obj in inspect.getmembers(config):
        if inspect.isclass(obj) and issubclass(obj, BaseModel) and obj is not BaseModel:
            models[name] = get_model_info(obj)

    return models


def main() -> None:
    """Generate configuration documentation from Pydantic Settings."""
    parser = argparse.ArgumentParser(
        prog="dump-config.py",
        description="Generate configuration documentation from Pydantic Settings",
    )
    parser.add_argument(
        "--template",
        help="Jinja2 template file path",
        default="docs/reference/configuration/templates/template.j2",
    )
    parser.add_argument(
        "--output",
        help="Output markdown file path",
        default="docs/reference/configuration/index.md",
    )

    args = parser.parse_args()

    # Extract model information
    models = extract_all_models()

    # Load and render template
    template_path = Path(args.template)
    if not template_path.exists():
        raise FileNotFoundError(f"Template file not found: {template_path}")

    with template_path.open("r") as f:
        template_content = f.read()

    template = jinja2.Template(template_content)
    rendered = template.render(models=models)

    # Apply markdown linting
    cleaned_content = _lint_markdown(rendered)

    # Write output
    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)

    with output_path.open("w") as f:
        f.write(cleaned_content)


if __name__ == "__main__":
    main()
