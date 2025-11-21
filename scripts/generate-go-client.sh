#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
OPENAPI_FILE="$ROOT_DIR/docs/reference/api/openapi.json"
OPENAPI_30_FILE="$ROOT_DIR/docs/reference/api/openapi.3.0.json"
GO_CLIENT_DIR="$ROOT_DIR/clients/go"

if [ ! -f "$OPENAPI_FILE" ]; then
    echo "Error: OpenAPI file not found at $OPENAPI_FILE"
    echo "Run 'make openapi' first"
    exit 1
fi

OAPI_CODEGEN="oapi-codegen"
if ! command -v oapi-codegen &> /dev/null; then
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
    if [ -n "$GOPATH" ]; then
        OAPI_CODEGEN="$GOPATH/bin/oapi-codegen"
    else
        OAPI_CODEGEN="$HOME/go/bin/oapi-codegen"
    fi
fi

python3 "$SCRIPT_DIR/convert_openapi_to_30.py" "$OPENAPI_FILE" "$OPENAPI_30_FILE"

mkdir -p "$GO_CLIENT_DIR"

"$OAPI_CODEGEN" -package kodit -generate types "$OPENAPI_30_FILE" > "$GO_CLIENT_DIR/types.gen.go"
"$OAPI_CODEGEN" -package kodit -generate client "$OPENAPI_30_FILE" > "$GO_CLIENT_DIR/client.gen.go"

cd "$GO_CLIENT_DIR"
go mod tidy
go fmt ./...
