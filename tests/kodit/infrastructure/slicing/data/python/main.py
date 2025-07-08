"""Main application entry point."""

from .models import Point, Product, ShoppingCart
from .utils import chunk_list, format_currency


def create_sample_products() -> list[Product]:
    """Create sample products for demonstration."""
    return [
        Product("Laptop", 999.99, 5),
        Product("Mouse", 29.99, 20),
        Product("Keyboard", 79.99, 15),
        Product("Monitor", 299.99, 8),
    ]


def demonstrate_cart() -> None:
    """Demonstrate shopping cart functionality."""
    cart = ShoppingCart()
    products = create_sample_products()

    # Add some products to cart
    cart.add_product(products[0])  # Laptop
    cart.add_product(products[1])  # Mouse

    total = cart.calculate_total()
    print(f"Cart total: {format_currency(total)}")
    print(f"Items in cart: {cart.get_item_count()}")


def demonstrate_points() -> None:
    """Demonstrate point calculations."""
    p1 = Point(0, 0)
    p2 = Point(3, 4)

    distance = p1.distance_to(p2)
    print(f"Distance between points: {distance:.2f}")


def main() -> None:
    """Main function."""
    print("=== Shopping Cart Demo ===")
    demonstrate_cart()

    print("\n=== Point Distance Demo ===")
    demonstrate_points()

    print("\n=== Chunk List Demo ===")
    numbers = list(range(1, 11))
    chunks = chunk_list(numbers, 3)
    print(f"Chunked {numbers} into: {chunks}")


if __name__ == "__main__":
    main()
