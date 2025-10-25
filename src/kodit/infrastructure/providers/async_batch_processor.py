"""Generic async batch processor with semaphore-controlled concurrency."""

import asyncio
from collections.abc import AsyncGenerator, Awaitable, Callable
from typing import TypeVar

T = TypeVar("T")
R = TypeVar("R")


async def process_items_concurrently(
    items: list[T],
    process_fn: Callable[[T], Awaitable[R]],
    max_parallel_tasks: int,
) -> AsyncGenerator[R, None]:
    """Process items concurrently with semaphore-controlled concurrency.

    Args:
        items: List of items to process.
        process_fn: Async function to process each item.
        max_parallel_tasks: Maximum number of concurrent tasks.

    Yields:
        Results as they are completed (not necessarily in order).

    """
    if not items:
        return

    sem = asyncio.Semaphore(max_parallel_tasks)

    async def _process_with_semaphore(item: T) -> R:
        async with sem:
            return await process_fn(item)

    tasks: list[asyncio.Task[R]] = [
        asyncio.create_task(_process_with_semaphore(item)) for item in items
    ]

    try:
        for task in asyncio.as_completed(tasks):
            yield await task
    finally:
        # Cancel any remaining tasks when generator exits
        # (due to exception, Ctrl+C, or early consumer termination)
        for task in tasks:
            if not task.done():
                task.cancel()

        # Wait for all tasks to finish cancelling
        await asyncio.gather(*tasks, return_exceptions=True)
