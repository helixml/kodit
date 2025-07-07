/**
 * Main application entry point.
 */
package com.example;

import java.util.Arrays;
import java.util.List;

public class Main {
    
    /**
     * Create sample products for demonstration
     * @return List of sample products
     * @throws Utils.ValidationException if product creation fails
     */
    private static List<Product> createSampleProducts() throws Utils.ValidationException {
        return Arrays.asList(
            new Product("Laptop", 999.99, 5),
            new Product("Mouse", 29.99, 20),
            new Product("Keyboard", 79.99, 15),
            new Product("Monitor", 299.99, 8)
        );
    }
    
    /**
     * Demonstrate shopping cart functionality
     * @throws Utils.ValidationException if validation fails
     */
    private static void demonstrateCart() throws Utils.ValidationException {
        ShoppingCart cart = new ShoppingCart();
        List<Product> products = createSampleProducts();
        
        // Add some products to cart
        cart.addProduct(products.get(0)); // Laptop
        cart.addProduct(products.get(1)); // Mouse
        
        double total = cart.calculateTotal();
        System.out.println("Cart total: " + Utils.formatCurrency(total, "USD"));
        System.out.println("Items in cart: " + cart.getItemCount());
    }
    
    /**
     * Demonstrate point calculations
     */
    private static void demonstratePoints() {
        Point p1 = new Point(0.0, 0.0);
        Point p2 = new Point(3.0, 4.0);
        
        double distance = p1.distanceTo(p2);
        System.out.printf("Distance between points: %.2f%n", distance);
    }
    
    /**
     * Main method
     * @param args Command line arguments
     */
    public static void main(String[] args) {
        try {
            System.out.println("=== Shopping Cart Demo ===");
            demonstrateCart();
            
            System.out.println("\n=== Point Distance Demo ===");
            demonstratePoints();
            
            System.out.println("\n=== Chunk List Demo ===");
            List<Integer> numbers = Arrays.asList(1, 2, 3, 4, 5, 6, 7, 8, 9, 10);
            List<List<Integer>> chunks = Utils.chunkList(numbers, 3);
            System.out.println("Chunked " + numbers + " into: " + chunks);
            
        } catch (Utils.ValidationException e) {
            System.err.println("Validation error: " + e.getMessage());
            System.exit(1);
        }
    }
}