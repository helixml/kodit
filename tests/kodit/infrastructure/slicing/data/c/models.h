#ifndef MODELS_H
#define MODELS_H

#include <stddef.h>

/**
 * A point in 2D space
 */
typedef struct {
    double x;
    double y;
} Point;

/**
 * A product with name, price, and inventory
 */
typedef struct {
    char name[100];
    double price;
    int inventory;
} Product;

/**
 * A shopping cart that holds products
 */
typedef struct {
    Product* items;
    size_t capacity;
    size_t count;
} ShoppingCart;

/**
 * Create a new point
 * @param x X coordinate
 * @param y Y coordinate
 * @return New point
 */
Point point_new(double x, double y);

/**
 * Calculate distance to another point
 * @param p1 First point
 * @param p2 Second point
 * @return Distance between points
 */
double point_distance_to(Point p1, Point p2);

/**
 * Create a new product
 * @param name Product name
 * @param price Product price
 * @param inventory Initial inventory
 * @return New product
 */
Product product_new(const char* name, double price, int inventory);

/**
 * Calculate total inventory value
 * @param product The product
 * @return Total value
 */
double product_total_value(const Product* product);

/**
 * Create a new shopping cart
 * @param initial_capacity Initial capacity
 * @return Pointer to new cart or NULL on failure
 */
ShoppingCart* cart_new(size_t initial_capacity);

/**
 * Add a product to the cart
 * @param cart The cart
 * @param product The product to add
 * @return 1 on success, 0 on failure
 */
int cart_add_product(ShoppingCart* cart, Product product);

/**
 * Calculate total cart value
 * @param cart The cart
 * @return Total value
 */
double cart_calculate_total(const ShoppingCart* cart);

/**
 * Get number of items in cart
 * @param cart The cart
 * @return Number of items
 */
size_t cart_get_item_count(const ShoppingCart* cart);

/**
 * Free a shopping cart
 * @param cart The cart to free
 */
void cart_free(ShoppingCart* cart);

#endif // MODELS_H