# Makefile for Kodit

# Generate OpenAPI json schema from the FastAPI app
openapi:
	uv run src/kodit/utils/dump_openapi.py --out docs/reference/api/ kodit.app:app