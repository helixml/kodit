/**
 * Main application entry point.
 */

const { Point, Product, ShoppingCart } = require('./models');
const { formatCurrency, chunkArray } = require('./utils');

/**
 * Create sample products for demonstration
 * @returns {Product[]} Array of sample products
 */
function createSampleProducts() {
    return [
        new Product('Laptop', 999.99, 5),
        new Product('Mouse', 29.99, 20),
        new Product('Keyboard', 79.99, 15),
        new Product('Monitor', 299.99, 8)
    ];
}

/**
 * Demonstrate shopping cart functionality
 */
function demonstrateCart() {
    const cart = new ShoppingCart();
    const products = createSampleProducts();
    
    // Add some products to cart
    cart.addProduct(products[0]); // Laptop
    cart.addProduct(products[1]); // Mouse
    
    const total = cart.calculateTotal();
    console.log(`Cart total: ${formatCurrency(total)}`);
    console.log(`Items in cart: ${cart.getItemCount()}`);
}

/**
 * Demonstrate point calculations
 */
function demonstratePoints() {
    const p1 = new Point(0, 0);
    const p2 = new Point(3, 4);
    
    const distance = p1.distanceTo(p2);
    console.log(`Distance between points: ${distance.toFixed(2)}`);
}

/**
 * Main function
 */
function main() {
    console.log('=== Shopping Cart Demo ===');
    demonstrateCart();
    
    console.log('\n=== Point Distance Demo ===');
    demonstratePoints();
    
    console.log('\n=== Chunk Array Demo ===');
    const numbers = Array.from({ length: 10 }, (_, i) => i + 1);
    const chunks = chunkArray(numbers, 3);
    console.log(`Chunked ${numbers} into:`, chunks);
}

if (require.main === module) {
    main();
}