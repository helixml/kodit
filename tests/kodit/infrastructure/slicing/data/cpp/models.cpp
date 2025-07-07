#include "models.hpp"

namespace models {

Point::Point(double x, double y) : x_(x), y_(y) {}

double Point::distanceTo(const Point& other) const {
    return utils::calculateDistance(x_, y_, other.x_, other.y_);
}

Product::Product(const std::string& name, double price, int inventory)
    : name_(name), price_(price), inventory_(inventory) {
    utils::validatePositive(price, "price");
    if (inventory < 0) {
        throw utils::ValidationError("inventory cannot be negative");
    }
}

double Product::totalValue() const {
    return price_ * inventory_;
}

void ShoppingCart::addProduct(std::unique_ptr<Product> product) {
    items_.push_back(std::move(product));
}

double ShoppingCart::calculateTotal() const {
    double total = 0.0;
    for (const auto& item : items_) {
        total += item->getPrice();
    }
    return total;
}

size_t ShoppingCart::getItemCount() const {
    return items_.size();
}

} // namespace models