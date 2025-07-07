#pragma once

#include <stdexcept>
#include <string>
#include <vector>
#include <cmath>

namespace utils {

/**
 * Exception for validation failures
 */
class ValidationError : public std::runtime_error {
public:
    explicit ValidationError(const std::string& message)
        : std::runtime_error(message) {}
};

/**
 * Validate that a value is positive
 * @param value The value to validate
 * @param name The name of the value for error messages
 * @throws ValidationError if value is not positive
 */
void validatePositive(double value, const std::string& name = "value");

/**
 * Calculate Euclidean distance between two points
 * @param x1 First point x coordinate
 * @param y1 First point y coordinate
 * @param x2 Second point x coordinate
 * @param y2 Second point y coordinate
 * @return The distance between the points
 */
double calculateDistance(double x1, double y1, double x2, double y2);

/**
 * Format an amount as currency
 * @param amount The amount to format
 * @param currency The currency symbol
 * @return Formatted currency string
 */
std::string formatCurrency(double amount, const std::string& currency = "USD");

/**
 * Split a vector into chunks of specified size
 * @tparam T The type of elements in the vector
 * @param items The vector to chunk
 * @param chunkSize Size of each chunk
 * @return Vector of chunks
 * @throws ValidationError if chunkSize is not positive
 */
template<typename T>
std::vector<std::vector<T>> chunkVector(const std::vector<T>& items, size_t chunkSize);

} // namespace utils