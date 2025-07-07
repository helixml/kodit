/// Data models for the example project.

use crate::utils::{validate_positive, calculate_distance, ValidationError};

/// A point in 2D space
#[derive(Debug, Clone)]
pub struct Point {
    pub x: f64,
    pub y: f64,
}

impl Point {
    /// Create a new point
    pub fn new(x: f64, y: f64) -> Self {
        Point { x, y }
    }

    /// Calculate distance to another point
    pub fn distance_to(&self, other: &Point) -> f64 {
        calculate_distance(self.x, self.y, other.x, other.y)
    }
}

/// A product with name, price, and inventory
#[derive(Debug, Clone)]
pub struct Product {
    pub name: String,
    pub price: f64,
    pub inventory: u32,
}

impl Product {
    /// Create a new product with validation
    pub fn new(name: String, price: f64, inventory: u32) -> Result<Self, ValidationError> {
        validate_positive(price, "price")?;
        
        Ok(Product {
            name,
            price,
            inventory,
        })
    }

    /// Calculate total inventory value
    pub fn total_value(&self) -> f64 {
        self.price * self.inventory as f64
    }
}

/// A shopping cart that holds products
#[derive(Debug)]
pub struct ShoppingCart {
    items: Vec<Product>,
}

impl ShoppingCart {
    /// Create a new shopping cart
    pub fn new() -> Self {
        ShoppingCart {
            items: Vec::new(),
        }
    }

    /// Add a product to the cart
    pub fn add_product(&mut self, product: Product) {
        self.items.push(product);
    }

    /// Calculate total cart value
    pub fn calculate_total(&self) -> f64 {
        self.items.iter().map(|item| item.price).sum()
    }

    /// Get number of items in cart
    pub fn get_item_count(&self) -> usize {
        self.items.len()
    }
}

impl Default for ShoppingCart {
    fn default() -> Self {
        Self::new()
    }
}