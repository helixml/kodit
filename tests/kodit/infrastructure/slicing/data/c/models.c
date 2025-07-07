#include "models.h"
#include "utils.h"
#include <stdlib.h>
#include <string.h>

Point point_new(double x, double y) {
    Point p = {x, y};
    return p;
}

double point_distance_to(Point p1, Point p2) {
    return calculate_distance(p1.x, p1.y, p2.x, p2.y);
}

Product product_new(const char* name, double price, int inventory) {
    Product p;
    strncpy(p.name, name, sizeof(p.name) - 1);
    p.name[sizeof(p.name) - 1] = '\0';
    p.price = price;
    p.inventory = inventory;
    return p;
}

double product_total_value(const Product* product) {
    return product->price * product->inventory;
}

ShoppingCart* cart_new(size_t initial_capacity) {
    ShoppingCart* cart = malloc(sizeof(ShoppingCart));
    if (!cart) return NULL;
    
    cart->items = malloc(sizeof(Product) * initial_capacity);
    if (!cart->items) {
        free(cart);
        return NULL;
    }
    
    cart->capacity = initial_capacity;
    cart->count = 0;
    return cart;
}

int cart_add_product(ShoppingCart* cart, Product product) {
    if (cart->count >= cart->capacity) {
        // Resize if needed
        size_t new_capacity = cart->capacity * 2;
        Product* new_items = realloc(cart->items, sizeof(Product) * new_capacity);
        if (!new_items) return 0;
        
        cart->items = new_items;
        cart->capacity = new_capacity;
    }
    
    cart->items[cart->count++] = product;
    return 1;
}

double cart_calculate_total(const ShoppingCart* cart) {
    double total = 0.0;
    for (size_t i = 0; i < cart->count; i++) {
        total += cart->items[i].price;
    }
    return total;
}

size_t cart_get_item_count(const ShoppingCart* cart) {
    return cart->count;
}

void cart_free(ShoppingCart* cart) {
    if (cart) {
        free(cart->items);
        free(cart);
    }
}