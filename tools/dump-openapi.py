#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "openapi-markdown",
# ]
# ///
"""Generate markdown API docs from the Go-generated OpenAPI spec."""

import argparse
import json
import shutil
from pathlib import Path
from typing import Any

from openapi_markdown.generator import to_markdown  # type: ignore[import-untyped]

parser = argparse.ArgumentParser(prog="dump-openapi.py")
parser.add_argument(
    "--spec",
    help="Path to OpenAPI 3.0 JSON spec",
    default="docs/swagger/openapi.json",
)
parser.add_argument("--out-dir", help="Output directory", default="docs/reference/api")

if __name__ == "__main__":
    args = parser.parse_args()

    spec_path = Path(args.spec)
    if not spec_path.exists():
        raise FileNotFoundError(
            f"OpenAPI spec not found at {spec_path}. Run 'make openapi' first."
        )

    with spec_path.open() as f:
        openapi = json.load(f)

    version = openapi.get("info", {}).get("version", "")
    if not version:
        raise ValueError(f"Invalid version in spec: {openapi.get('info')}")

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    output_json_file = out_dir / "openapi.json"
    shutil.copy2(spec_path, output_json_file)

    output_md_file = out_dir / "index.md"
    templates_dir = out_dir / "templates"
    options: dict[str, Any] = {}

    to_markdown(str(output_json_file), str(output_md_file), str(templates_dir), options)
    print(f"Generated {output_md_file} from {spec_path} (version {version})")
