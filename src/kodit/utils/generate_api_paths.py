#!/usr/bin/env python3
"""Generate API path constants from OpenAPI specification."""

import argparse
import json
from pathlib import Path


def to_python_constant(path: str) -> str:
    """Convert API path to Python constant name.

    Args:
        path: API path like '/api/v1/indexes' or '/api/v1/indexes/{index_id}'

    Returns:
        Python constant name like 'API_V1_INDEXES' or 'API_V1_INDEXES_INDEX_ID'

    """
    import re

    # Replace any non-alphanumeric characters with underscores and convert to uppercase
    clean_path = re.sub(r"[^a-zA-Z0-9]", "_", path)
    clean_path = clean_path.strip("_").upper()

    # Remove consecutive underscores
    clean_path = re.sub(r"_+", "_", clean_path)

    return clean_path or "ROOT"


def generate_api_paths_file(openapi_spec_path: Path, output_path: Path) -> None:
    """Generate Python file with API path constants from OpenAPI spec.

    Args:
        openapi_spec_path: Path to the OpenAPI JSON specification
        output_path: Path where to write the generated Python file

    """
    with openapi_spec_path.open() as f:
        spec = json.load(f)

    paths = spec.get("paths", {})

    # Sort paths for consistent output
    sorted_paths = sorted(paths.keys())

    # Generate Python constants
    constants = []
    for path in sorted_paths:
        constant_name = to_python_constant(path)
        constants.append((constant_name, path))

    # Generate the Python file content
    content = generate_python_file_content(constants, openapi_spec_path)

    # Write the file
    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w") as f:
        f.write(content)



def generate_python_file_content(
    constants: list[tuple[str, str]], spec_path: Path
) -> str:
    """Generate the content of the Python API paths file.

    Args:
        constants: List of (constant_name, path_value) tuples
        spec_path: Path to the OpenAPI spec (for documentation)

    Returns:
        Generated Python file content

    """
    # Header
    content = '''"""API endpoint constants generated from OpenAPI specification.

This file is auto-generated. Do not edit manually.
Run `make generate-api-paths` to regenerate.
"""

from typing import Final


class APIEndpoints:
    """API endpoint constants extracted from OpenAPI specification."""

'''

    # Add constants with comments
    for constant_name, path_value in constants:
        # Add a comment describing the endpoint
        comment = f"    # {path_value}"
        constant = f'    {constant_name}: Final[str] = "{path_value}"'

        content += f"{comment}\n{constant}\n\n"

    # Add footer comment
    content += f"""
# Generated from: {spec_path.name}
# Total endpoints: {len(constants)}
"""

    return content


def main() -> None:
    """Generate API path constants from OpenAPI specification."""
    parser = argparse.ArgumentParser(
        description="Generate API path constants from OpenAPI specification"
    )
    parser.add_argument(
        "--openapi-spec",
        type=Path,
        default=Path("docs/reference/api/openapi.json"),
        help="Path to OpenAPI JSON specification file"
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=Path("src/kodit/infrastructure/api/client/generated_endpoints.py"),
        help="Output path for generated Python file"
    )

    args = parser.parse_args()

    if not args.openapi_spec.exists():
        return

    generate_api_paths_file(args.openapi_spec, args.output)


if __name__ == "__main__":
    main()
