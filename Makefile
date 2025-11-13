# Makefile for Kodit

# Generate OpenAPI json schema from the FastAPI app
build:
	uv build

docs: build
	uv run src/kodit/utils/dump_openapi.py --out docs/reference/api/ kodit.app:app
	uv run python src/kodit/utils/dump_config.py

docs-check: docs
	git diff --exit-code docs/reference/api/index.md
	git diff --exit-code docs/reference/configuration/index.md

generate-api-paths: openapi
	uv run python src/kodit/utils/generate_api_paths.py

type:
	uv run mypy --config-file pyproject.toml .

lint:
	uv run ruff check --fix --unsafe-fixes

test: lint type
	uv run pytest -s --cov=src --cov-report=xml tests/kodit

no-database-changes-check:
	rm -f .kodit.db
	uv run alembic upgrade head
	uv run alembic check

test-migrations:
	uv run python tests/migrations.py

smoke:
	uv run tests/smoke.py