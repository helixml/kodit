# Makefile for Kodit

# Generate OpenAPI json schema from the FastAPI app
build:
	uv build

openapi: build
	uv run src/kodit/utils/dump_openapi.py --out docs/reference/api/ kodit.app:app

openapi-check: openapi
	git diff --exit-code docs/reference/api/index.md

generate-api-paths: openapi
	uv run python src/kodit/utils/generate_api_paths.py

type:
	uv run mypy --config-file pyproject.toml .

lint:
	uv run ruff check --fix --unsafe-fixes

test: lint type openapi-check
	uv run pytest -s --cov=src --cov-report=xml tests/kodit