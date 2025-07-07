#include "utils.hpp"
#include <sstream>
#include <iomanip>

namespace utils {

void validatePositive(double value, const std::string& name) {
    if (value <= 0.0) {
        throw ValidationError(name + " must be positive, got " + std::to_string(value));
    }
}

double calculateDistance(double x1, double y1, double x2, double y2) {
    double dx = x2 - x1;
    double dy = y2 - y1;
    return std::sqrt(dx * dx + dy * dy);
}

std::string formatCurrency(double amount, const std::string& currency) {
    std::ostringstream oss;
    oss << std::fixed << std::setprecision(2) << amount << " " << currency;
    return oss.str();
}

template<typename T>
std::vector<std::vector<T>> chunkVector(const std::vector<T>& items, size_t chunkSize) {
    validatePositive(static_cast<double>(chunkSize), "chunkSize");
    
    std::vector<std::vector<T>> chunks;
    for (size_t i = 0; i < items.size(); i += chunkSize) {
        auto end = std::min(i + chunkSize, items.size());
        chunks.emplace_back(items.begin() + i, items.begin() + end);
    }
    return chunks;
}

// Explicit template instantiation for common types
template std::vector<std::vector<int>> chunkVector(const std::vector<int>&, size_t);

} // namespace utils