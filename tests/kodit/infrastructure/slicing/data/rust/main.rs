/// Main application entry point.

mod utils;
mod models;

use models::{Point, Product, ShoppingCart};
use utils::{format_currency, chunk_vec};

/// Create sample products for demonstration
fn create_sample_products() -> Result<Vec<Product>, utils::ValidationError> {
    Ok(vec![
        Product::new("Laptop".to_string(), 999.99, 5)?,
        Product::new("Mouse".to_string(), 29.99, 20)?,
        Product::new("Keyboard".to_string(), 79.99, 15)?,
        Product::new("Monitor".to_string(), 299.99, 8)?,
    ])
}

/// Demonstrate shopping cart functionality
fn demonstrate_cart() -> Result<(), utils::ValidationError> {
    let mut cart = ShoppingCart::new();
    let products = create_sample_products()?;
    
    // Add some products to cart
    cart.add_product(products[0].clone()); // Laptop
    cart.add_product(products[1].clone()); // Mouse
    
    let total = cart.calculate_total();
    println!("Cart total: {}", format_currency(total, "USD"));
    println!("Items in cart: {}", cart.get_item_count());
    
    Ok(())
}

/// Demonstrate point calculations
fn demonstrate_points() {
    let p1 = Point::new(0.0, 0.0);
    let p2 = Point::new(3.0, 4.0);
    
    let distance = p1.distance_to(&p2);
    println!("Distance between points: {:.2}", distance);
}

/// Main function
fn main() -> Result<(), utils::ValidationError> {
    println!("=== Shopping Cart Demo ===");
    demonstrate_cart()?;
    
    println!("\n=== Point Distance Demo ===");
    demonstrate_points();
    
    println!("\n=== Chunk Vector Demo ===");
    let numbers: Vec<i32> = (1..=10).collect();
    let chunks = chunk_vec(&numbers, 3)?;
    println!("Chunked {:?} into: {:?}", numbers, chunks);
    
    Ok(())
}