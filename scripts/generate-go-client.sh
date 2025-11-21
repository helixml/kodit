#!/bin/bash
set -e

# Script to generate Go client from OpenAPI specification
# This script uses oapi-codegen to generate a Go client library

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
OPENAPI_FILE="$ROOT_DIR/docs/reference/api/openapi.json"
OPENAPI_30_FILE="$ROOT_DIR/docs/reference/api/openapi.3.0.json"
GO_CLIENT_DIR="$ROOT_DIR/clients/go"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}==> Generating Go client from OpenAPI specification${NC}"

# Check if OpenAPI file exists
if [ ! -f "$OPENAPI_FILE" ]; then
    echo -e "${RED}Error: OpenAPI file not found at $OPENAPI_FILE${NC}"
    echo -e "${BLUE}Run 'make openapi' first to generate the OpenAPI specification${NC}"
    exit 1
fi

# Check if oapi-codegen is installed
OAPI_CODEGEN="oapi-codegen"
if ! command -v oapi-codegen &> /dev/null; then
    echo -e "${BLUE}oapi-codegen not found. Installing...${NC}"
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
    # Try to find it in GOPATH/bin
    if [ -n "$GOPATH" ]; then
        OAPI_CODEGEN="$GOPATH/bin/oapi-codegen"
    else
        OAPI_CODEGEN="$HOME/go/bin/oapi-codegen"
    fi
fi

# Convert OpenAPI 3.1 to 3.0 for compatibility with oapi-codegen
echo -e "${BLUE}==> Converting OpenAPI 3.1 to 3.0${NC}"
python3 "$SCRIPT_DIR/convert_openapi_to_30.py" "$OPENAPI_FILE" "$OPENAPI_30_FILE"

# Create output directory if it doesn't exist
mkdir -p "$GO_CLIENT_DIR"

echo -e "${BLUE}==> Generating types${NC}"
"$OAPI_CODEGEN" -package kodit -generate types \
    "$OPENAPI_30_FILE" > "$GO_CLIENT_DIR/types.gen.go"

echo -e "${BLUE}==> Generating client${NC}"
"$OAPI_CODEGEN" -package kodit -generate client \
    "$OPENAPI_30_FILE" > "$GO_CLIENT_DIR/client.gen.go"

# Run go mod tidy to clean up dependencies
echo -e "${BLUE}==> Running go mod tidy${NC}"
cd "$GO_CLIENT_DIR"
go mod tidy

# Format the generated code
echo -e "${BLUE}==> Formatting generated code${NC}"
go fmt ./...

echo -e "${GREEN}==> Go client generation complete!${NC}"
echo -e "${GREEN}Generated files:${NC}"
echo -e "  - $GO_CLIENT_DIR/types.gen.go"
echo -e "  - $GO_CLIENT_DIR/client.gen.go"
