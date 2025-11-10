# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kodit is a code indexing MCP (Model Context Protocol) server that connects AI coding assistants to external codebases. It provides semantic and keyword search capabilities over indexed code snippets to improve AI-assisted development.

## Development Commands

### Testing

- `uv run pytest src/kodit` - Run all unit tests with coverage
- `uv run pytest tests/path/to/test.py` - Run specific test file
- `uv run pytest -k "test_name"` - Run specific test by name

### Linting and Typing

- `uv run ruff check --fix --unsafe-fixes` - Run linting and format code
- `uv run mypy --config-file pyproject.toml ...` - Type checking

### Application

- `uv run kodit --help` - Show CLI help
- `uv run kodit serve` - Start MCP server

### Database

- `uv run alembic upgrade head` - Apply database migrations
- `uv run alembic revision --autogenerate -m "description"` - Generate new migration

## Architecture

### Domain-Driven Design Structure

The codebase follows Domain-Driven Design (DDD) with clean architecture:

- `domain/` - Core business logic and interfaces
  - `entities.py` - Domain entities (Snippet, File, etc.)
  - `repositories.py` - Repository interfaces
  - `services/` - Domain services for business logic
- `application/` - Application services and factories
- `infrastructure/` - External concerns and implementations
  - `sqlalchemy/` - Database repositories
  - `embedding/` - Vector embedding providers
  - `bm25/` - BM25 search implementations
  - `indexing/` - Code indexing services

## Configuration

Key environment variables:

- `DB_URL` - database connection string
- `LOG_LEVEL` - logging level
- `DEFAULT_SEARCH_PROVIDER` - deciding whether to use vectorchord or sqlite

See `config.py` for full configuration options.

## Database

Uses SQLAlchemy with async support. Supports:

- SQLite (default, local development)
- PostgreSQL with Vectorchord (production)

Migrations managed with Alembic in `src/kodit/migrations/` directory. DO NOT EDIT THESE FILES.

## Refactoring Strategy

- When refactoring, follow the following strategy:
  - Always use Martin Fowler's Refactoring Catalog to guide your changes.
  - After each change, run linting and typing on the files to ensure your changes are correct.

## Testing Strategy

When writing tests for my code, follow these guidelines:

**Test Strategy:**

- Focus on testing critical paths and core functionality, not on achieving high coverage percentages
- Prioritize practical, valuable tests over comprehensive coverage

**Test Types & Architecture:**

- Write integration tests that use the real database layer
- Write end-to-end tests that mock the database layer to keep tests fast
- Avoid tests that take a long time to run

**Testing Workflow:**

- Write tests after the implementation is complete
- Tests should validate the working implementation

**Test Scope:**

- Focus primarily on happy path scenarios
- Test only public interfaces, not private methods or implementation details
- Only add edge case tests when bugs are discovered that need regression protection

**Mocking Philosophy:**

- Minimize mocking - only mock when it provides clear benefits
- Database layer mocking is acceptable for end-to-end tests
- Avoid mocking everything else
- Do not use patches in tests because they are fragile to changes. If you must use them,
  ONLY use them on core python libraries.

**Test Organization:**

- Test file structure should mirror the production codebase structure
- Keep test organization consistent with the code being tested

## Coding Style

- Docstrings should be very minimal. Descriptions only. No arguments or return types. No
  examples. No deprecation notices.
- Do not store imports imports in `__init__.py` files.
