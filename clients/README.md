# Kodit API Client Libraries

This directory contains auto-generated client libraries for the Kodit API in multiple languages.

## Available Clients

- **Go**: [`./go/`](./go/) - Go client library

## Generating Clients

Client libraries are generated from the OpenAPI specification located at `docs/reference/api/openapi.json`.

To regenerate all clients:

```bash
make generate-clients
```

To regenerate a specific client:

```bash
make generate-go-client
```

Or run the generation scripts directly:

```bash
./scripts/generate-go-client.sh
```

## Adding New Language Support

To add support for a new language:

1. Create a new directory under `clients/` for your language
2. Add a generation script in `scripts/` (e.g., `generate-python-client.sh`)
3. Update the Makefile with a new target
4. Add documentation in the language-specific README

## Requirements

Each client generation script may have different requirements. See the individual client READMEs for details.

For Go client generation:
- Go 1.21 or later
- `oapi-codegen` tool
