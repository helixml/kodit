#pragma once

#include "utils.hpp"
#include <string>
#include <vector>
#include <memory>

namespace models {

/**
 * A point in 2D space
 */
class Point {
private:
    double x_, y_;

public:
    Point(double x, double y);
    
    double getX() const { return x_; }
    double getY() const { return y_; }
    
    /**
     * Calculate distance to another point
     * @param other The other point
     * @return Distance to the other point
     */
    double distanceTo(const Point& other) const;
};

/**
 * A product with name, price, and inventory
 */
class Product {
private:
    std::string name_;
    double price_;
    int inventory_;

public:
    /**
     * Create a new product with validation
     * @param name Product name
     * @param price Product price
     * @param inventory Initial inventory
     * @throws utils::ValidationError if price is not positive or inventory is negative
     */
    Product(const std::string& name, double price, int inventory = 0);
    
    const std::string& getName() const { return name_; }
    double getPrice() const { return price_; }
    int getInventory() const { return inventory_; }
    
    /**
     * Calculate total inventory value
     * @return Total value of inventory
     */
    double totalValue() const;
};

/**
 * A shopping cart that holds products
 */
class ShoppingCart {
private:
    std::vector<std::unique_ptr<Product>> items_;

public:
    ShoppingCart() = default;
    
    // Disable copy constructor and assignment operator
    ShoppingCart(const ShoppingCart&) = delete;
    ShoppingCart& operator=(const ShoppingCart&) = delete;
    
    // Enable move constructor and assignment operator
    ShoppingCart(ShoppingCart&&) = default;
    ShoppingCart& operator=(ShoppingCart&&) = default;
    
    /**
     * Add a product to the cart
     * @param product The product to add
     */
    void addProduct(std::unique_ptr<Product> product);
    
    /**
     * Calculate total cart value
     * @return Total value of all items
     */
    double calculateTotal() const;
    
    /**
     * Get number of items in cart
     * @return Number of items
     */
    size_t getItemCount() const;
};

} // namespace models