#!/usr/bin/env python3
"""Convert OpenAPI 3.1 spec to 3.0 for better tool compatibility."""

import json
import sys
from pathlib import Path


def convert_31_to_30(spec: dict) -> dict:
    """Convert OpenAPI 3.1 spec to 3.0.

    This handles the main compatibility issues:
    - Change version from 3.1.x to 3.0.3
    - Convert nullable types from type arrays to nullable property
    """
    spec = spec.copy()

    # Update version
    spec["openapi"] = "3.0.3"

    # Process components
    if "components" in spec and "schemas" in spec["components"]:
        spec["components"]["schemas"] = convert_schemas(spec["components"]["schemas"])

    # Process paths
    if "paths" in spec:
        spec["paths"] = convert_paths(spec["paths"])

    return spec


def convert_schemas(schemas: dict) -> dict:
    """Convert schema definitions."""
    result = {}
    for name, schema in schemas.items():
        result[name] = convert_schema(schema)
    return result


def _handle_type_array(schema: dict) -> dict:
    """Handle type arrays with null (OpenAPI 3.1 feature)."""
    if "type" not in schema or not isinstance(schema["type"], list):
        return schema

    types = schema["type"]
    if "null" not in types:
        return schema

    schema["nullable"] = True
    non_null_types = [t for t in types if t != "null"]
    if len(non_null_types) == 1:
        schema["type"] = non_null_types[0]
    elif non_null_types:
        schema.pop("type")
        schema["anyOf"] = [{"type": t} for t in non_null_types]

    return schema


def _handle_any_of(schema: dict) -> dict:
    """Handle anyOf with null."""
    if "anyOf" not in schema:
        return schema

    any_of = schema["anyOf"]
    null_schemas = [s for s in any_of if s.get("type") == "null"]
    non_null_schemas = [s for s in any_of if s.get("type") != "null"]

    if not null_schemas:
        return schema

    schema["nullable"] = True
    if non_null_schemas:
        schema["anyOf"] = [convert_schema(s) for s in non_null_schemas]
    else:
        schema.pop("anyOf", None)

    return schema


def convert_schema(schema: dict | list) -> dict | list:
    """Convert a single schema, handling nullable types."""
    if isinstance(schema, list):
        return [convert_schema(s) for s in schema]

    if not isinstance(schema, dict):
        return schema

    schema = schema.copy()

    schema = _handle_type_array(schema)
    schema = _handle_any_of(schema)

    # Recursively process nested schemas
    for key in ["properties", "items", "additionalProperties"]:
        if key in schema:
            if key == "properties":
                schema[key] = {k: convert_schema(v) for k, v in schema[key].items()}
            else:
                schema[key] = convert_schema(schema[key])

    if "allOf" in schema:
        schema["allOf"] = [convert_schema(s) for s in schema["allOf"]]

    return schema


def convert_paths(paths: dict) -> dict:
    """Convert path definitions."""
    result = {}
    for path, path_item in paths.items():
        result[path] = convert_path_item(path_item)
    return result


def convert_path_item(path_item: dict) -> dict:
    """Convert a path item."""
    path_item = path_item.copy()

    for method in ["get", "post", "put", "delete", "patch", "options", "head", "trace"]:
        if method in path_item:
            path_item[method] = convert_operation(path_item[method])

    return path_item


def convert_operation(operation: dict) -> dict:
    """Convert an operation."""
    operation = operation.copy()

    # Convert parameters
    if "parameters" in operation:
        operation["parameters"] = [
            convert_parameter(p) for p in operation["parameters"]
        ]

    # Convert request body
    if "requestBody" in operation:
        operation["requestBody"] = convert_request_body(operation["requestBody"])

    # Convert responses
    if "responses" in operation:
        operation["responses"] = {
            status: convert_response(resp)
            for status, resp in operation["responses"].items()
        }

    return operation


def convert_parameter(param: dict) -> dict:
    """Convert a parameter."""
    param = param.copy()
    if "schema" in param:
        param["schema"] = convert_schema(param["schema"])
    return param


def convert_request_body(request_body: dict) -> dict:
    """Convert a request body."""
    request_body = request_body.copy()
    if "content" in request_body:
        request_body["content"] = convert_content(request_body["content"])
    return request_body


def convert_response(response: dict) -> dict:
    """Convert a response."""
    response = response.copy()
    if "content" in response:
        response["content"] = convert_content(response["content"])
    return response


def convert_content(content: dict) -> dict:
    """Convert content definitions."""
    result = {}
    for media_type, media_type_obj in content.items():
        converted_obj = media_type_obj.copy()
        if "schema" in converted_obj:
            converted_obj["schema"] = convert_schema(converted_obj["schema"])
        result[media_type] = converted_obj
    return result


def main() -> None:
    """Convert OpenAPI file from 3.1 to 3.0 format."""
    if len(sys.argv) != 3:
        sys.exit(1)

    input_file = Path(sys.argv[1])
    output_file = Path(sys.argv[2])

    with input_file.open() as f:
        spec = json.load(f)

    converted = convert_31_to_30(spec)

    with output_file.open("w") as f:
        json.dump(converted, f, indent=2)



if __name__ == "__main__":
    main()
