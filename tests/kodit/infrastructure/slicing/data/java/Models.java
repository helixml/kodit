/**
 * Data models for the example project.
 */
package com.example;

import java.util.ArrayList;
import java.util.List;

/**
 * A point in 2D space
 */
class Point {
    private final double x;
    private final double y;
    
    public Point(double x, double y) {
        this.x = x;
        this.y = y;
    }
    
    public double getX() { return x; }
    public double getY() { return y; }
    
    /**
     * Calculate distance to another point
     * @param other The other point
     * @return Distance to the other point
     */
    public double distanceTo(Point other) {
        return Utils.calculateDistance(this.x, this.y, other.x, other.y);
    }
}

/**
 * A product with name, price, and inventory
 */
class Product {
    private final String name;
    private final double price;
    private final int inventory;
    
    /**
     * Create a new product with validation
     * @param name Product name
     * @param price Product price
     * @param inventory Initial inventory
     * @throws Utils.ValidationException if price is not positive or inventory is negative
     */
    public Product(String name, double price, int inventory) throws Utils.ValidationException {
        Utils.validatePositive(price, "price");
        if (inventory < 0) {
            throw new Utils.ValidationException("inventory cannot be negative");
        }
        
        this.name = name;
        this.price = price;
        this.inventory = inventory;
    }
    
    public String getName() { return name; }
    public double getPrice() { return price; }
    public int getInventory() { return inventory; }
    
    /**
     * Calculate total inventory value
     * @return Total value of inventory
     */
    public double totalValue() {
        return price * inventory;
    }
}

/**
 * A shopping cart that holds products
 */
class ShoppingCart {
    private final List<Product> items;
    
    public ShoppingCart() {
        this.items = new ArrayList<>();
    }
    
    /**
     * Add a product to the cart
     * @param product The product to add
     */
    public void addProduct(Product product) {
        items.add(product);
    }
    
    /**
     * Calculate total cart value
     * @return Total value of all items
     */
    public double calculateTotal() {
        return items.stream()
                   .mapToDouble(Product::getPrice)
                   .sum();
    }
    
    /**
     * Get number of items in cart
     * @return Number of items
     */
    public int getItemCount() {
        return items.size();
    }
}