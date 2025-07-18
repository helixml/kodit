"""Dump the OpenAPI json schema to a file."""

import argparse
import json
from pathlib import Path
from typing import Any

from openapi_markdown.generator import to_markdown  # type: ignore[import-untyped]
from uvicorn.importer import import_from_string

parser = argparse.ArgumentParser(prog="dump-openapi.py")
parser.add_argument(
    "app", help='App import string. Eg. "kodit.app:app"', default="kodit.app:app"
)
parser.add_argument("--out-dir", help="Output directory", default="docs/reference/api")

if __name__ == "__main__":
    args = parser.parse_args()

    app = import_from_string(args.app)
    openapi = app.openapi()
    version = openapi.get("openapi", "unknown version")

    # Remove any dev tags from the version by retaining only the semver part
    git_tag = openapi["info"]["version"].split(".")[:3]
    openapi["info"]["version"] = ".".join(git_tag)

    output_json_file = Path(args.out_dir) / "openapi.json"

    with output_json_file.open("w") as f:
        json.dump(openapi, f, indent=2)

    output_md_file = Path(args.out_dir) / "index.md"
    templates_dir = Path(args.out_dir) / "templates"
    options: dict[str, Any] = {}

    to_markdown(str(output_json_file), str(output_md_file), str(templates_dir), options)
