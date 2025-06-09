// Import statements
import { format } from 'date-fns';
import { v4 as uuidv4 } from 'uuid';

// Global utility functions
function calculateTotal(items) {
    return items.reduce((sum, item) => sum + item.price, 0);
}

function formatCurrency(amount) {
    return new Intl.NumberFormat('en-US', {
        style: 'currency',
        currency: 'USD'
    }).format(amount);
}

// Class definition
class ShoppingCart {
    constructor() {
        this.id = uuidv4();
        this.items = [];
        this.createdAt = new Date();
    }

    addItem(item) {
        this.items.push({
            ...item,
            addedAt: new Date()
        });
    }

    removeItem(itemId) {
        this.items = this.items.filter(item => item.id !== itemId);
    }

    getTotal() {
        return calculateTotal(this.items);
    }

    getFormattedTotal() {
        return formatCurrency(this.getTotal());
    }

    getCreationDate() {
        return format(this.createdAt, 'MMMM do, yyyy');
    }
}

// Usage example
const cart = new ShoppingCart();

// Adding items to the cart
cart.addItem({
    id: uuidv4(),
    name: 'Laptop',
    price: 999.99
});

cart.addItem({
    id: uuidv4(),
    name: 'Mouse',
    price: 29.99
});

// Display cart information
console.log('Cart ID:', cart.id);
console.log('Created on:', cart.getCreationDate());
console.log('Total:', cart.getFormattedTotal());
console.log('Number of items:', cart.items.length);

// Remove an item
const firstItemId = cart.items[0].id;
cart.removeItem(firstItemId);
console.log('Total after removal:', cart.getFormattedTotal());
