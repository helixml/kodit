"""Tests for the Cline prompt integration with user prompts."""

import os
from pathlib import Path
from typing import TYPE_CHECKING, Any

import openai
import pytest
import structlog

if TYPE_CHECKING:
    from openai.types.chat import ChatCompletionMessageParam

log = structlog.get_logger(__name__)

# Load OpenAI API key from environment variable
openai.api_key = os.getenv("OPENAI_API_KEY")
if not openai.api_key:
    raise ValueError("OPENAI_API_KEY environment variable is not set")


def load_cline_prompt() -> str:
    """Load the Cline prompt from file."""
    prompt_path = Path(__file__).parent / "cline_prompt.txt"
    if not prompt_path.exists():
        raise FileNotFoundError(f"Cline prompt file not found at {prompt_path}")
    prompt = prompt_path.read_text()
    assert prompt, "Cline prompt is empty"
    return prompt


def create_code_prompts() -> list[str]:
    """Create a list of code-related test prompts that should trigger kodit usage."""
    return [
        "Using the kodit mcp tool, develop a new pydantic-ai example with tools.",
        "Write a Python function to calculate fibonacci numbers",
        "Create a React component that displays a todo list",
        "Implement a binary search algorithm in JavaScript",
        "Write a SQL query to find all users who made purchases in the last month",
    ]


def create_non_code_prompts() -> list[str]:
    """Create a list of non-code test prompts that should not trigger kodit tool."""
    return [
        "What is the capital of France?",
        "Tell me about the history of artificial intelligence",
        "What are the best practices for project management?",
        "Explain the concept of machine learning to a beginner",
    ]


def create_test_prompts() -> list[str]:
    """Create a list of test prompts to evaluate."""
    return create_code_prompts() + create_non_code_prompts()


def analyze_response(response: str) -> dict[str, Any]:
    """Analyze the response to determine if kodit tool was used."""
    return {
        "used_kodit_tool": "<use_mcp_tool>" in response
        and "<server_name>kodit</server_name>" in response,
        "response_length": len(response),
    }


async def process_prompt(prompt: str, cline_prompt: str) -> dict[str, Any]:
    """Process a single prompt and return the analysis results."""
    messages: list[ChatCompletionMessageParam] = [
        {"role": "developer", "content": cline_prompt},
        {"role": "user", "content": prompt},
    ]

    # Call OpenAI API
    response = openai.chat.completions.create(
        model="o4-mini-2025-04-16",
        messages=messages,
        reasoning_effort="medium",  # Default set in cline request, user configurable
        stream=False,  # Cline streams, but this makes it easier to parse
    )

    # Get the response content
    response_content = response.choices[0].message.content
    if response_content is None:
        raise ValueError("OpenAI API returned None for response content")

    # Analyze the response
    analysis = analyze_response(response_content)
    log.info(
        "Prompt",
        prompt=prompt,
        result=analysis["used_kodit_tool"],
    )

    return {"prompt": prompt, "analysis": analysis, "response": response_content}


async def process_prompts(
    prompts: list[str], cline_prompt: str
) -> list[dict[str, Any]]:
    """Process a list of prompts and return the analysis results."""
    results = []
    for prompt in prompts:
        result = await process_prompt(prompt, cline_prompt)
        results.append(result)
    return results


def print_results(results: list[dict[str, Any]], category: str) -> None:
    """Print the analysis results for a category of prompts."""
    log.info("Results", category=category)
    log.info("=" * 80)
    for result in results:
        log.info("Prompt", prompt=result["prompt"])
        log.info(
            "Used kodit tool", used_kodit_tool=result["analysis"]["used_kodit_tool"]
        )
        log.info(
            "Response length", response_length=result["analysis"]["response_length"]
        )
        log.info("-" * 80)


@pytest.mark.asyncio
async def test_cline_prompt_integration() -> None:
    """Test the integration between Cline prompt and user prompts."""
    cline_prompt = load_cline_prompt()

    # Process code-related prompts
    code_prompts = create_code_prompts()
    code_results = await process_prompts(code_prompts, cline_prompt)

    # Process non-code prompts
    non_code_prompts = create_non_code_prompts()
    non_code_results = await process_prompts(non_code_prompts, cline_prompt)

    # Print results
    print_results(code_results, "Code-Related")
    print_results(non_code_results, "Non-Code")

    # Test code-related prompts
    code_prompt_kodit_usage = sum(
        1 for r in code_results if r["analysis"]["used_kodit_tool"]
    )
    assert code_prompt_kodit_usage == len(code_results), (
        "All code-related prompts should trigger kodit tool usage"
    )

    # Test non-code prompts
    non_code_prompt_kodit_usage = sum(
        1 for r in non_code_results if r["analysis"]["used_kodit_tool"]
    )
    assert non_code_prompt_kodit_usage == 0, (
        "Non-code prompts should not trigger kodit tool usage"
    )
