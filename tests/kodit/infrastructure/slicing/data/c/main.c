#include <stdio.h>
#include <stdlib.h>
#include "models.h"
#include "utils.h"

/**
 * Create sample products for demonstration
 * @param products Array to store products
 * @param max_products Maximum number of products
 * @return Number of products created
 */
int create_sample_products(Product* products, int max_products) {
    if (max_products < 4) return 0;
    
    products[0] = product_new("Laptop", 999.99, 5);
    products[1] = product_new("Mouse", 29.99, 20);
    products[2] = product_new("Keyboard", 79.99, 15);
    products[3] = product_new("Monitor", 299.99, 8);
    
    return 4;
}

/**
 * Demonstrate shopping cart functionality
 */
void demonstrate_cart(void) {
    ShoppingCart* cart = cart_new(10);
    if (!cart) {
        printf("Failed to create cart\n");
        return;
    }
    
    Product products[4];
    int count = create_sample_products(products, 4);
    
    // Add some products to cart
    cart_add_product(cart, products[0]); // Laptop
    cart_add_product(cart, products[1]); // Mouse
    
    double total = cart_calculate_total(cart);
    char currency_str[50];
    format_currency(total, "USD", currency_str, sizeof(currency_str));
    
    printf("Cart total: %s\n", currency_str);
    printf("Items in cart: %zu\n", cart_get_item_count(cart));
    
    cart_free(cart);
}

/**
 * Demonstrate point calculations
 */
void demonstrate_points(void) {
    Point p1 = point_new(0.0, 0.0);
    Point p2 = point_new(3.0, 4.0);
    
    double distance = point_distance_to(p1, p2);
    printf("Distance between points: %.2f\n", distance);
}

/**
 * Main function
 */
int main(void) {
    printf("=== Shopping Cart Demo ===\n");
    demonstrate_cart();
    
    printf("\n=== Point Distance Demo ===\n");
    demonstrate_points();
    
    return 0;
}