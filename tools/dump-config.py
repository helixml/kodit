#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "jinja2",
# ]
# ///
"""Generate configuration docs from Go envconfig struct tags."""

import argparse
import re
from pathlib import Path
from typing import Any

# Go type to documentation type mapping.
TYPE_MAP = {
    "string": "string",
    "int": "int",
    "bool": "bool",
    "float64": "float",
}

# Pattern to match a struct field line with envconfig tag.
FIELD_RE = re.compile(
    r"""
    ^\s+
    (\w+)            # field name
    \s+
    (\S+)            # type
    \s+
    `([^`]+)`        # struct tags
    """,
    re.VERBOSE,
)

TAG_RE = re.compile(r'(\w+):"([^"]*)"')

# Matches Go doc comments like "FieldName is the ..." or "FieldName controls ..."
GODOC_PREFIX_RE = re.compile(
    r"^\w+ (?:is |controls |holds |configures |represents |skips |specifies )"
)


def clean_description(desc: str) -> str:
    """Strip the Go doc convention prefix (e.g. 'Host is the ...') from a comment."""
    m = GODOC_PREFIX_RE.match(desc)
    if m:
        remainder = desc[m.end() :]
        return remainder[0].upper() + remainder[1:] if remainder else desc
    return desc


def parse_struct_tags(raw: str) -> dict[str, str]:
    """Parse Go struct tags into a dict."""
    return dict(TAG_RE.findall(raw))


def parse_structs(source: str) -> dict[str, list[dict[str, Any]]]:
    """Parse Go source to extract struct definitions with envconfig tags."""
    structs: dict[str, list[dict[str, Any]]] = {}
    lines = source.splitlines()
    current_struct = None
    comment_lines: list[str] = []

    for line in lines:
        # Detect struct start.
        m = re.match(r"^type (\w+) struct \{", line)
        if m:
            current_struct = m.group(1)
            structs[current_struct] = []
            comment_lines = []
            continue

        # Detect struct end.
        if current_struct and line.strip() == "}":
            current_struct = None
            comment_lines = []
            continue

        if current_struct is None:
            continue

        # Collect comments.
        stripped = line.strip()
        if stripped.startswith("//"):
            comment_lines.append(stripped.lstrip("/ "))
            continue

        # Try to match a field.
        fm = FIELD_RE.match(line)
        if fm:
            go_type = fm.group(2)
            tags = parse_struct_tags(fm.group(3))
            envconfig_tag = tags.get("envconfig", "")
            default = tags.get("default", "")

            # Extract description and comment-based default from comments.
            description = ""
            comment_default = ""
            for cl in comment_lines:
                if cl.startswith("Default:"):
                    comment_default = cl[len("Default:") :].strip()
                elif not cl.startswith("Env:") and not cl.startswith("WARNING:") and not description:
                    description = cl

            # Struct tag default takes precedence; comment default is fallback.
            if not default and comment_default:
                default = comment_default

            structs[current_struct].append(
                {
                    "go_type": go_type,
                    "envconfig": envconfig_tag,
                    "default": default,
                    "description": description,
                }
            )
            comment_lines = []
        else:
            # Non-field, non-comment line resets comments.
            if stripped:
                comment_lines = []

    return structs


def resolve_env_vars(
    structs: dict[str, list[dict[str, Any]]],
) -> list[dict[str, str]]:
    """Walk EnvConfig and resolve all env var names, flattening nested structs."""
    env_vars: list[dict[str, str]] = []
    root = structs.get("EnvConfig", [])

    for field in root:
        tag = field["envconfig"]
        if not tag:
            continue

        go_type = field["go_type"]

        # If the type is a known nested struct, expand its fields with the prefix.
        if go_type in structs:
            nested = structs[go_type]
            for nf in nested:
                ntag = nf["envconfig"]
                if not ntag:
                    continue
                env_vars.append(_make_entry(nf, f"{tag}_{ntag}"))
        else:
            env_vars.append(_make_entry(field, tag))

    return env_vars


def _make_entry(field: dict[str, Any], name: str) -> dict[str, str]:
    """Build a single env var documentation entry."""
    go_type = field["go_type"]
    doc_type = TYPE_MAP.get(go_type, go_type)
    default = field["default"]
    description = clean_description(field["description"])

    if not default:
        default = "_(empty)_"
    else:
        default = f"`{default}`"

    return {
        "name": name,
        "type": doc_type,
        "default": default,
        "description": description,
    }


def lint_markdown(content: str) -> str:
    """Apply basic markdown linting rules to clean up formatting."""
    lines = content.split("\n")
    cleaned: list[str] = []
    prev_empty = False

    for line in lines:
        line = line.rstrip()
        is_empty = len(line) == 0

        if is_empty and prev_empty:
            continue

        cleaned.append(line)
        prev_empty = is_empty

    result = "\n".join(cleaned)
    result = result.rstrip("\n") + "\n"
    return re.sub(r"\n{3,}", "\n\n", result)


def main() -> None:
    """Generate configuration documentation from Go envconfig structs."""
    import jinja2

    parser = argparse.ArgumentParser(
        prog="dump-config.py",
        description="Generate configuration docs from Go envconfig structs",
    )
    parser.add_argument(
        "--source",
        help="Path to Go env.go source file",
        default="internal/config/env.go",
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

    source_path = Path(args.source)
    if not source_path.exists():
        raise FileNotFoundError(f"Source file not found: {source_path}")

    source = source_path.read_text()
    structs = parse_structs(source)
    env_vars = resolve_env_vars(structs)

    template_path = Path(args.template)
    if not template_path.exists():
        raise FileNotFoundError(f"Template file not found: {template_path}")

    template = jinja2.Template(template_path.read_text())
    rendered = template.render(env_vars=env_vars)
    cleaned = lint_markdown(rendered)

    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(cleaned)

    print(f"Generated {output_path} from {source_path} ({len(env_vars)} variables)")


if __name__ == "__main__":
    main()
