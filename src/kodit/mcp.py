"""MCP server for kodit."""

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import Annotated

import structlog
from fastmcp import Context, FastMCP
from pydantic import Field

from kodit._version import version
from kodit.application.factories.server_factory import ServerFactory
from kodit.application.services.code_search_application_service import MultiSearchResult
from kodit.config import AppContext
from kodit.database import Database
from kodit.domain.value_objects import (
    MultiSearchRequest,
    SnippetSearchFilters,
)
from kodit.infrastructure.sqlalchemy.query import QueryBuilder

# Global database connection for MCP server
_mcp_db: Database | None = None
_mcp_server_factory: ServerFactory | None = None


@dataclass
class MCPContext:
    """Context for the MCP server."""

    server_factory: ServerFactory


@asynccontextmanager
async def mcp_lifespan(_: FastMCP) -> AsyncIterator[MCPContext]:
    """Lifespan context manager for the MCP server.

    This is called for each request. The MCP server is designed to work with both
    the CLI and the FastAPI server. Therefore, we must carefully reconstruct the
    application context. uvicorn does not pass through CLI args, so we must rely on
    parsing env vars set in the CLI.

    This lifespan is recreated for each request. See:
    https://github.com/jlowin/fastmcp/issues/166

    Since they don't provide a good way to handle global state, we must use a
    global variable to store the database connection.
    """
    global _mcp_server_factory  # noqa: PLW0603
    if _mcp_server_factory is None:
        app_context = AppContext()
        db = await app_context.get_db()
        _mcp_server_factory = ServerFactory(app_context, db.session_factory)
    yield MCPContext(_mcp_server_factory)


def create_mcp_server(name: str, instructions: str | None = None) -> FastMCP:
    """Create a FastMCP server with common configuration."""
    return FastMCP(
        name,
        lifespan=mcp_lifespan,
        instructions=instructions,
    )


def _format_enrichments(enrichments: list) -> str:
    """Format enrichments in a simple, LLM-friendly format.

    Just returns the content from each enrichment, separated by double newlines.
    """
    if not enrichments:
        return ""

    contents = [e.content for e in enrichments if e.content]
    return "\n\n".join(contents)


def register_mcp_tools(mcp_server: FastMCP) -> None:  # noqa: C901, PLR0915
    """Register MCP tools on the provided FastMCP instance."""

    @mcp_server.tool()
    async def search(  # noqa: PLR0913
        ctx: Context,
        user_intent: Annotated[
            str,
            Field(
                description="Think about what the user wants to achieve. Describe the "
                "user's intent in one sentence."
            ),
        ],
        related_file_paths: Annotated[
            list[Path],
            Field(
                description=(
                    "A list of absolute paths to files that are relevant to the "
                    "user's intent."
                )
            ),
        ],
        related_file_contents: Annotated[
            list[str],
            Field(
                description=(
                    "A list of the contents of the files that are relevant to the "
                    "user's intent."
                )
            ),
        ],
        keywords: Annotated[
            list[str],
            Field(
                description=(
                    "A list of keywords that are relevant to the desired outcome."
                )
            ),
        ],
        language: Annotated[
            str | None,
            Field(
                description="Filter by language (e.g., 'python', 'go', 'javascript')."
            ),
        ] = None,
        author: Annotated[
            str | None,
            Field(description=("Filter to search for snippets by a specific author.")),
        ] = None,
        created_after: Annotated[
            str | None,
            Field(
                description=(
                    "Filter for snippets created after this date "
                    "(ISO format: YYYY-MM-DD)."
                )
            ),
        ] = None,
        created_before: Annotated[
            str | None,
            Field(
                description=(
                    "Filter for snippets created before this date "
                    "(ISO format: YYYY-MM-DD)."
                )
            ),
        ] = None,
        source_repo: Annotated[
            str | None,
            Field(
                description=(
                    "Filter results by project source repository (e.g., "
                    "github.com/example/repo)"
                )
            ),
        ] = None,
    ) -> str:
        """Search for pre-existing examples of relevant code.

        ⚠️ **ENFORCEMENT:**
        For *every* user request that involves writing or modifying code (of any
        language or domain), the assistant's *first* action **must** be to call the
        kodit.search MCP tool. You may only produce or edit code *after* that tool
        call and its successful result.
        """
        # This docstring is used by the AI assistant to decide when to call the tool.
        # If you want to update it, please make sure you thoroughly test the
        # assistant's response to the updated tool call. See:
        # tests/experiments/cline-prompt-regression-tests/cline_prompt_test.py

        log = structlog.get_logger(__name__)

        log.debug(
            "Searching for relevant snippets",
            user_intent=user_intent,
            keywords=keywords,
            file_count=len(related_file_paths),
            file_paths=related_file_paths,
            file_contents=related_file_contents,
        )

        mcp_context: MCPContext = ctx.request_context.lifespan_context

        # Use the unified application service
        service = mcp_context.server_factory.code_search_application_service()

        log.debug("Searching for snippets")

        # Create filters if any filter parameters are provided
        filters = SnippetSearchFilters.from_cli_params(
            language=language,
            author=author,
            created_after=created_after,
            created_before=created_before,
            source_repo=source_repo,
        )

        search_request = MultiSearchRequest(
            keywords=keywords,
            code_query="\n".join(related_file_contents),
            text_query=user_intent,
            filters=filters,
        )

        log.debug("Searching for snippets")
        snippets = await service.search(request=search_request)

        log.debug("Fusing output")
        output = MultiSearchResult.to_jsonlines(results=snippets)

        log.debug("Output", output=output)
        return output

    @mcp_server.tool()
    async def get_version() -> str:
        """Get the version of the kodit project."""
        return version

    @mcp_server.tool()
    async def list_repositories(ctx: Context) -> str:
        """List all repositories available in the system.

        Returns a simple list of repository URLs.
        """
        mcp_context: MCPContext = ctx.request_context.lifespan_context
        repo_repository = mcp_context.server_factory.repo_repository()

        repos = await repo_repository.find(QueryBuilder())

        if not repos:
            return "No repositories found."

        lines = ["Available repositories:"]
        lines.extend(f"- {repo.sanitized_remote_uri}" for repo in repos)

        return "\n".join(lines)

    @mcp_server.tool()
    async def get_architecture_docs(
        ctx: Context,
        repo_url: Annotated[
            str,
            Field(description="The repository URL (e.g., github.com/user/repo)"),
        ],
        commit_sha: Annotated[
            str | None,
            Field(
                description=(
                    "Optional commit SHA. If not provided, uses most recent "
                    "commit with architecture docs"
                )
            ),
        ] = None,
    ) -> str:
        """Get architecture documentation enrichments for a repository.

        Returns architecture docs describing the physical structure and
        organization of the codebase.
        """
        mcp_context: MCPContext = ctx.request_context.lifespan_context
        repo_query_service = mcp_context.server_factory.repository_query_service()

        # Find the repository by URL
        repo_id = await repo_query_service.find_repo_by_url(repo_url)
        if not repo_id:
            return ""

        if not commit_sha:
            commit_sha = await repo_query_service.find_latest_commit(
                repo_id=repo_id,
                max_commits_to_check=100,
            )
            if not commit_sha:
                return ""

        enrichment_service = mcp_context.server_factory.enrichment_query_service()
        enrichments = await enrichment_service.get_architecture_docs_for_commit(
            commit_sha
        )
        return _format_enrichments(enrichments)

    @mcp_server.tool()
    async def get_api_docs(
        ctx: Context,
        repo_url: Annotated[
            str,
            Field(description="The repository URL (e.g., github.com/user/repo)"),
        ],
        commit_sha: Annotated[
            str | None,
            Field(
                description=(
                    "Optional commit SHA. If not provided, uses most recent "
                    "commit with API docs"
                )
            ),
        ] = None,
    ) -> str:
        """Get API documentation enrichments for a repository.

        Returns API docs describing public interfaces and usage patterns.
        """
        mcp_context: MCPContext = ctx.request_context.lifespan_context
        repo_query_service = mcp_context.server_factory.repository_query_service()

        # Find the repository by URL
        repo_id = await repo_query_service.find_repo_by_url(repo_url)
        if not repo_id:
            return ""

        if not commit_sha:
            commit_sha = await repo_query_service.find_latest_commit(
                repo_id=repo_id,
                max_commits_to_check=100,
            )
            if not commit_sha:
                return ""

        enrichment_service = mcp_context.server_factory.enrichment_query_service()
        enrichments = await enrichment_service.get_api_docs_for_commit(commit_sha)
        return _format_enrichments(enrichments)

    @mcp_server.tool()
    async def get_commit_description(
        ctx: Context,
        repo_url: Annotated[
            str,
            Field(description="The repository URL (e.g., github.com/user/repo)"),
        ],
        commit_sha: Annotated[
            str | None,
            Field(
                description=(
                    "Optional commit SHA. If not provided, uses most recent "
                    "commit with description"
                )
            ),
        ] = None,
    ) -> str:
        """Get commit description enrichments for a repository.

        Returns human-readable descriptions explaining what changed and why.
        """
        mcp_context: MCPContext = ctx.request_context.lifespan_context
        repo_query_service = mcp_context.server_factory.repository_query_service()

        # Find the repository by URL
        repo_id = await repo_query_service.find_repo_by_url(repo_url)
        if not repo_id:
            return ""

        if not commit_sha:
            commit_sha = await repo_query_service.find_latest_commit(
                repo_id=repo_id,
                max_commits_to_check=100,
            )
            if not commit_sha:
                return ""

        enrichment_service = mcp_context.server_factory.enrichment_query_service()
        enrichments = await enrichment_service.get_commit_description_for_commit(
            commit_sha
        )
        return _format_enrichments(enrichments)

    @mcp_server.tool()
    async def get_database_schema(
        ctx: Context,
        repo_url: Annotated[
            str,
            Field(description="The repository URL (e.g., github.com/user/repo)"),
        ],
        commit_sha: Annotated[
            str | None,
            Field(
                description=(
                    "Optional commit SHA. If not provided, uses most recent "
                    "commit with database schema"
                )
            ),
        ] = None,
    ) -> str:
        """Get database schema enrichments for a repository.

        Returns database schema docs from ORM models, migrations, or schema
        definitions.
        """
        mcp_context: MCPContext = ctx.request_context.lifespan_context
        repo_query_service = mcp_context.server_factory.repository_query_service()

        # Find the repository by URL
        repo_id = await repo_query_service.find_repo_by_url(repo_url)
        if not repo_id:
            return ""

        if not commit_sha:
            commit_sha = await repo_query_service.find_latest_commit(
                repo_id=repo_id,
                max_commits_to_check=100,
            )
            if not commit_sha:
                return ""

        enrichment_service = mcp_context.server_factory.enrichment_query_service()
        enrichments = await enrichment_service.get_database_schema_for_commit(
            commit_sha
        )
        return _format_enrichments(enrichments)

    @mcp_server.tool()
    async def get_cookbook(
        ctx: Context,
        repo_url: Annotated[
            str,
            Field(description="The repository URL (e.g., github.com/user/repo)"),
        ],
        commit_sha: Annotated[
            str | None,
            Field(
                description=(
                    "Optional commit SHA. If not provided, uses most recent "
                    "commit with cookbook examples"
                )
            ),
        ] = None,
    ) -> str:
        """Get cookbook enrichments for a repository.

        Returns cookbook-style code examples with context showing how to use
        various parts of the codebase.
        """
        mcp_context: MCPContext = ctx.request_context.lifespan_context
        repo_query_service = mcp_context.server_factory.repository_query_service()

        # Find the repository by URL
        repo_id = await repo_query_service.find_repo_by_url(repo_url)
        if not repo_id:
            return ""

        if not commit_sha:
            commit_sha = await repo_query_service.find_latest_commit(
                repo_id=repo_id,
                max_commits_to_check=100,
            )
            if not commit_sha:
                return ""

        enrichment_service = mcp_context.server_factory.enrichment_query_service()
        enrichments = await enrichment_service.get_cookbook_for_commit(commit_sha)
        return _format_enrichments(enrichments)


# FastAPI-integrated MCP server
mcp = create_mcp_server(
    name="Kodit",
    instructions=(
        "This server is used to assist with code generation by retrieving "
        "code examples related to the user's intent."
        "Call search() to retrieve relevant code examples."
    ),
)

# Register the MCP tools
register_mcp_tools(mcp)


def create_stdio_mcp_server() -> None:
    """Create and run a STDIO MCP server for kodit."""
    mcp.run(transport="stdio", show_banner=False)
