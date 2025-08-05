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
- `uv run kodit index <path>` - Index a codebase
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

### Key Components

**Advanced Indexing Pipeline:**

1. Clone/read source code with Git metadata extraction
2. Language detection for 20+ programming languages
3. Advanced snippet extraction using Tree-sitter with dependency analysis
4. Build call graphs and import maps for context-aware extraction
5. Generate embeddings and BM25 indices
6. Store in database with selective reindexing for performance

**Advanced Search System:**

- Hybrid search combining semantic (embeddings) and keyword (BM25) with Reciprocal Rank Fusion
- Multi-dimensional filtering: language, author, date range, source, file path
- Context-aware results with dependency tracking and usage examples
- Multiple providers: local models, OpenAI, custom APIs
- Configurable via environment variables
- Support for 20+ programming languages including HTML/CSS

**MCP Server:**

- FastMCP-based server exposing search tools
- Integrates with Cursor, Cline, and other AI assistants

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

Migrations managed with Alembic in `migrations/` directory. DO NOT EDIT THESE FILES.

## Testing Strategy

- Unit tests for domain services and repositories
- Integration tests for database operations
- E2E tests for full indexing pipeline
- Smoke tests for deployment validation
- Performance tests for similarity search

Test file names should mirror the source structure under `tests/` directory and end in
the name `_test.py`.
