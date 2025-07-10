"""Data models for the example project."""

from dataclasses import dataclass

from .utils import calculate_distance, validate_positive


@dataclass
class Point:
    """A point in 2D space."""

    x: float
    y: float

    def distance_to(self, other: "Point") -> float:
        """Calculate distance to another point."""
        return calculate_distance(self.x, self.y, other.x, other.y)


@dataclass
class Product:
    """A product with name, price, and inventory."""

    name: str
    price: float
    inventory: int = 0

    def __post_init__(self):
        validate_positive(self.price, "price")
        if self.inventory < 0:
            raise ValueError("inventory cannot be negative")

    def total_value(self) -> float:
        """Calculate total inventory value."""
        return self.price * self.inventory


class ShoppingCart:
    """A shopping cart that holds products."""

    def __init__(self) -> None:
        self.items: list[Product] = []

    def add_product(self, product: Product) -> None:
        """Add a product to the cart."""
        self.items.append(product)

    def calculate_total(self) -> float:
        """Calculate total cart value."""
        return sum(item.price for item in self.items)

    def get_item_count(self) -> int:
        """Get number of items in cart."""
        return len(self.items)
