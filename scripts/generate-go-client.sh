#!/usr/bin/env bash
set -euo pipefail

OPENAPI_SPEC="./infrastructure/api/openapi.json"
OUTPUT_DIR="./clients/go"
PACKAGE="kodit"

echo "Generating Go client from ${OPENAPI_SPEC}..."

oapi-codegen -package "${PACKAGE}" -generate types \
  -o "${OUTPUT_DIR}/types.gen.go" "${OPENAPI_SPEC}"

oapi-codegen -package "${PACKAGE}" -generate client \
  -o "${OUTPUT_DIR}/client.gen.go" "${OPENAPI_SPEC}"

echo "Go client generated in ${OUTPUT_DIR}"
