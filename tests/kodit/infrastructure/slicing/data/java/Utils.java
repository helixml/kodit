/**
 * Utility functions for the example project.
 */
package com.example;

import java.util.ArrayList;
import java.util.List;

public class Utils {
    
    /**
     * Exception for validation failures
     */
    public static class ValidationException extends Exception {
        public ValidationException(String message) {
            super(message);
        }
    }
    
    /**
     * Validate that a value is positive
     * @param value The value to validate
     * @param name The name of the value for error messages
     * @throws ValidationException if value is not positive
     */
    public static void validatePositive(double value, String name) throws ValidationException {
        if (value <= 0) {
            throw new ValidationException(name + " must be positive, got " + value);
        }
    }
    
    /**
     * Calculate Euclidean distance between two points
     * @param x1 First point x coordinate
     * @param y1 First point y coordinate
     * @param x2 Second point x coordinate
     * @param y2 Second point y coordinate
     * @return The distance between the points
     */
    public static double calculateDistance(double x1, double y1, double x2, double y2) {
        double dx = x2 - x1;
        double dy = y2 - y1;
        return Math.sqrt(dx * dx + dy * dy);
    }
    
    /**
     * Format an amount as currency
     * @param amount The amount to format
     * @param currency The currency symbol
     * @return Formatted currency string
     */
    public static String formatCurrency(double amount, String currency) {
        return String.format("%.2f %s", amount, currency);
    }
    
    /**
     * Split a list into chunks of specified size
     * @param <T> The type of elements in the list
     * @param items The list to chunk
     * @param chunkSize Size of each chunk
     * @return List of chunks
     * @throws ValidationException if chunkSize is not positive
     */
    public static <T> List<List<T>> chunkList(List<T> items, int chunkSize) throws ValidationException {
        validatePositive(chunkSize, "chunkSize");
        
        List<List<T>> chunks = new ArrayList<>();
        for (int i = 0; i < items.size(); i += chunkSize) {
            int end = Math.min(i + chunkSize, items.size());
            chunks.add(new ArrayList<>(items.subList(i, end)));
        }
        return chunks;
    }
}