#ifndef UTILS_H
#define UTILS_H

#include <math.h>

/**
 * Validate that a value is positive
 * @param value The value to validate
 * @return 1 if valid, 0 if invalid
 */
int validate_positive(double value);

/**
 * Calculate Euclidean distance between two points
 * @param x1 First point x coordinate
 * @param y1 First point y coordinate
 * @param x2 Second point x coordinate
 * @param y2 Second point y coordinate
 * @return The distance between the points
 */
double calculate_distance(double x1, double y1, double x2, double y2);

/**
 * Format an amount as currency string
 * @param amount The amount to format
 * @param currency The currency symbol
 * @param buffer Buffer to store the result
 * @param buffer_size Size of the buffer
 * @return Number of characters written
 */
int format_currency(double amount, const char* currency, char* buffer, size_t buffer_size);

#endif // UTILS_H