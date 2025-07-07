#include "utils.h"
#include <stdio.h>
#include <math.h>

int validate_positive(double value) {
    return value > 0.0 ? 1 : 0;
}

double calculate_distance(double x1, double y1, double x2, double y2) {
    double dx = x2 - x1;
    double dy = y2 - y1;
    return sqrt(dx * dx + dy * dy);
}

int format_currency(double amount, const char* currency, char* buffer, size_t buffer_size) {
    return snprintf(buffer, buffer_size, "%.2f %s", amount, currency);
}