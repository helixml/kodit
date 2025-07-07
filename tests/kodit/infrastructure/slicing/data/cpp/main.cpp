#include "models.hpp"
#include "utils.hpp"
#include <iostream>
#include <vector>
#include <memory>

using namespace models;
using namespace utils;

/**
 * Create sample products for demonstration
 * @return Vector of sample products
 */
std::vector<std::unique_ptr<Product>> createSampleProducts() {
    std::vector<std::unique_ptr<Product>> products;
    
    products.push_back(std::make_unique<Product>("Laptop", 999.99, 5));
    products.push_back(std::make_unique<Product>("Mouse", 29.99, 20));
    products.push_back(std::make_unique<Product>("Keyboard", 79.99, 15));
    products.push_back(std::make_unique<Product>("Monitor", 299.99, 8));
    
    return products;
}

/**
 * Demonstrate shopping cart functionality
 */
void demonstrateCart() {
    ShoppingCart cart;
    auto products = createSampleProducts();
    
    // Add some products to cart (move ownership)
    cart.addProduct(std::make_unique<Product>("Laptop", 999.99, 5));
    cart.addProduct(std::make_unique<Product>("Mouse", 29.99, 20));
    
    double total = cart.calculateTotal();
    std::cout << "Cart total: " << formatCurrency(total) << std::endl;
    std::cout << "Items in cart: " << cart.getItemCount() << std::endl;
}

/**
 * Demonstrate point calculations
 */
void demonstratePoints() {
    Point p1(0.0, 0.0);
    Point p2(3.0, 4.0);
    
    double distance = p1.distanceTo(p2);
    std::cout << "Distance between points: " << std::fixed << std::setprecision(2) 
              << distance << std::endl;
}

/**
 * Main function
 */
int main() {
    try {
        std::cout << "=== Shopping Cart Demo ===" << std::endl;
        demonstrateCart();
        
        std::cout << "\n=== Point Distance Demo ===" << std::endl;
        demonstratePoints();
        
        std::cout << "\n=== Chunk Vector Demo ===" << std::endl;
        std::vector<int> numbers = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10};
        auto chunks = chunkVector(numbers, 3);
        
        std::cout << "Chunked vector into: ";
        for (const auto& chunk : chunks) {
            std::cout << "[";
            for (size_t i = 0; i < chunk.size(); ++i) {
                std::cout << chunk[i];
                if (i < chunk.size() - 1) std::cout << ", ";
            }
            std::cout << "] ";
        }
        std::cout << std::endl;
        
    } catch (const std::exception& e) {
        std::cerr << "Error: " << e.what() << std::endl;
        return 1;
    }
    
    return 0;
}