// Package main provides utility functions for the example project.
package main

import (
	"errors"
	"fmt"
	"math"
)

// ValidatePositive validates that a value is positive
func ValidatePositive(value float64, name string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be positive, got %f", name, value)
	}
	return nil
}

// CalculateDistance calculates Euclidean distance between two points
func CalculateDistance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// FormatCurrency formats an amount as currency
func FormatCurrency(amount float64, currency string) string {
	return fmt.Sprintf("%.2f %s", amount, currency)
}

// ChunkSlice splits a slice into chunks of specified size
func ChunkSlice(items []int, chunkSize int) ([][]int, error) {
	if err := ValidatePositive(float64(chunkSize), "chunkSize"); err != nil {
		return nil, err
	}
	
	var chunks [][]int
	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks, nil
}