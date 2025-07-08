"""Utility functions for the example project."""

import math


def validate_positive(value: float, name: str = "value") -> None:
    """Validate that a value is positive."""
    if value <= 0:
        raise ValueError(f"{name} must be positive, got {value}")


def calculate_distance(x1: float, y1: float, x2: float, y2: float) -> float:
    """Calculate Euclidean distance between two points."""
    dx = x2 - x1
    dy = y2 - y1
    return math.sqrt(dx * dx + dy * dy)


def format_currency(amount: float, currency: str = "USD") -> str:
    """Format an amount as currency."""
    return f"{amount:.2f} {currency}"


def chunk_list(items: list, chunk_size: int) -> list[list]:
    """Split a list into chunks of specified size."""
    validate_positive(chunk_size, "chunk_size")
    return [items[i : i + chunk_size] for i in range(0, len(items), chunk_size)]
