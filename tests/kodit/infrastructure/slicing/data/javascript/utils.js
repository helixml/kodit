/**
 * Utility functions for the example project.
 */

/**
 * Validate that a value is positive
 * @param {number} value - The value to validate
 * @param {string} name - The name of the value for error messages
 * @throws {Error} If value is not positive
 */
function validatePositive(value, name = 'value') {
    if (value <= 0) {
        throw new Error(`${name} must be positive, got ${value}`);
    }
}

/**
 * Calculate Euclidean distance between two points
 * @param {number} x1 - First point x coordinate
 * @param {number} y1 - First point y coordinate  
 * @param {number} x2 - Second point x coordinate
 * @param {number} y2 - Second point y coordinate
 * @returns {number} The distance between the points
 */
function calculateDistance(x1, y1, x2, y2) {
    const dx = x2 - x1;
    const dy = y2 - y1;
    return Math.sqrt(dx * dx + dy * dy);
}

/**
 * Format an amount as currency
 * @param {number} amount - The amount to format
 * @param {string} currency - The currency symbol
 * @returns {string} Formatted currency string
 */
function formatCurrency(amount, currency = 'USD') {
    return `${amount.toFixed(2)} ${currency}`;
}

/**
 * Split an array into chunks of specified size
 * @param {Array} items - The array to chunk
 * @param {number} chunkSize - Size of each chunk
 * @returns {Array} Array of chunks
 */
function chunkArray(items, chunkSize) {
    validatePositive(chunkSize, 'chunkSize');
    const chunks = [];
    for (let i = 0; i < items.length; i += chunkSize) {
        chunks.push(items.slice(i, i + chunkSize));
    }
    return chunks;
}

module.exports = {
    validatePositive,
    calculateDistance,
    formatCurrency,
    chunkArray
};