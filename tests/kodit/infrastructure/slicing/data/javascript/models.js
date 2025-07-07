/**
 * Data models for the example project.
 */

const { validatePositive, calculateDistance } = require('./utils');

/**
 * A point in 2D space
 */
class Point {
    constructor(x, y) {
        this.x = x;
        this.y = y;
    }

    /**
     * Calculate distance to another point
     * @param {Point} other - The other point
     * @returns {number} Distance to the other point
     */
    distanceTo(other) {
        return calculateDistance(this.x, this.y, other.x, other.y);
    }
}

/**
 * A product with name, price, and inventory
 */
class Product {
    constructor(name, price, inventory = 0) {
        validatePositive(price, 'price');
        if (inventory < 0) {
            throw new Error('inventory cannot be negative');
        }
        
        this.name = name;
        this.price = price;
        this.inventory = inventory;
    }

    /**
     * Calculate total inventory value
     * @returns {number} Total value of inventory
     */
    totalValue() {
        return this.price * this.inventory;
    }
}

/**
 * A shopping cart that holds products
 */
class ShoppingCart {
    constructor() {
        this.items = [];
    }

    /**
     * Add a product to the cart
     * @param {Product} product - The product to add
     */
    addProduct(product) {
        this.items.push(product);
    }

    /**
     * Calculate total cart value
     * @returns {number} Total value of all items
     */
    calculateTotal() {
        return this.items.reduce((total, item) => total + item.price, 0);
    }

    /**
     * Get number of items in cart
     * @returns {number} Number of items
     */
    getItemCount() {
        return this.items.length;
    }
}

module.exports = {
    Point,
    Product,
    ShoppingCart
};