"""MCP server implementation for kodit."""

from pathlib import Path
from typing import Annotated

import structlog
from mcp.server.fastmcp import FastMCP
from pydantic import Field

from kodit.database import get_session
from kodit.retreival.repository import RetrievalRepository, RetrievalResult
from kodit.retreival.service import RetrievalRequest, RetrievalService

mcp = FastMCP("kodit MCP Server")


@mcp.tool()
async def retrieve_relevant_snippets(
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
            description="A list of absolute paths to files that are relevant to the "
            "user's intent."
        ),
    ],
    related_file_contents: Annotated[
        list[str],
        Field(
            description="A list of the contents of the files that are relevant to the "
            "user's intent."
        ),
    ],
    keywords: Annotated[
        list[str],
        Field(
            description="A list of keywords that are relevant to the desired outcome."
        ),
    ],
) -> str:
    """Retrieve relevant snippets from various sources.

    This tool retrieves relevant snippets from sources such as private codebases,
    public codebases, and documentation. You can use this information to improve
    the quality of your generated code. You must call this tool when you need to
    write code.
    """
    # Log the search query and related files for debugging
    log = structlog.get_logger(__name__)
    log.debug(
        "Retrieving relevant snippets",
        user_intent=user_intent,
        keywords=keywords,
        file_count=len(related_file_paths),
        file_paths=related_file_paths,
        file_contents=related_file_contents,
    )

    async with get_session() as session:
        log.debug("Creating retrieval repository")
        retrieval_repository = RetrievalRepository(
            session=session,
        )

        log.debug("Creating retrieval service")
        retrieval_service = RetrievalService(
            repository=retrieval_repository,
        )

        log.debug("Fusing input")
        input_query = input_fusion(
            user_intent=user_intent,
            related_file_paths=related_file_paths,
            related_file_contents=related_file_contents,
            keywords=keywords,
        )
        log.debug("Input", input_query=input_query)
        retrieval_request = RetrievalRequest(
            query=input_query,
        )
        log.debug("Retrieving snippets")
        snippets = await retrieval_service.retrieve(request=retrieval_request)

        log.debug("Fusing output")
        output = output_fusion(snippets=snippets)

        log.debug("Output", output=output)
        return output


def input_fusion(
    user_intent: str,  # noqa: ARG001
    related_file_paths: list[Path],  # noqa: ARG001
    related_file_contents: list[str],  # noqa: ARG001
    keywords: list[str],
) -> str:
    """Fuse the search query and related file contents into a single query."""
    # Since this is a dummy implementation, we just return the first keyword
    return keywords[0] if len(keywords) > 0 else ""


def output_fusion(snippets: list[RetrievalResult]) -> str:
    """Fuse the snippets into a single output."""
    return "\n\n".join(f"{snippet.uri}\n{snippet.content}" for snippet in snippets)
