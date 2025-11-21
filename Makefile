# Makefile for Kodit

build:
	uv build

openapi: build
	uv run python src/kodit/utils/dump_openapi.py --out docs/reference/api/ kodit.app:app

generate-go-client: openapi
	./scripts/generate-go-client.sh

generate-clients: generate-go-client

docs: build openapi
	uv run python src/kodit/utils/dump_config.py

check: openapi generate-clients
	git diff --exit-code docs/reference/api/index.md
	git diff --exit-code docs/reference/configuration/index.md
	git diff --exit-code clients/go/types.gen.go
	git diff --exit-code clients/go/client.gen.go

generate-api-paths: build
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

integration-test: test-migrations no-database-changes-check

smoke:
	uv run tests/smoke.py
