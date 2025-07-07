// Package main is the main application entry point.
package main

import (
	"fmt"
	"log"
)

// createSampleProducts creates sample products for demonstration
func createSampleProducts() ([]*Product, error) {
	laptop, err := NewProduct("Laptop", 999.99, 5)
	if err != nil {
		return nil, err
	}
	
	mouse, err := NewProduct("Mouse", 29.99, 20)
	if err != nil {
		return nil, err
	}
	
	keyboard, err := NewProduct("Keyboard", 79.99, 15)
	if err != nil {
		return nil, err
	}
	
	monitor, err := NewProduct("Monitor", 299.99, 8)
	if err != nil {
		return nil, err
	}
	
	return []*Product{laptop, mouse, keyboard, monitor}, nil
}

// demonstrateCart demonstrates shopping cart functionality
func demonstrateCart() error {
	cart := NewShoppingCart()
	products, err := createSampleProducts()
	if err != nil {
		return err
	}
	
	// Add some products to cart
	cart.AddProduct(products[0]) // Laptop
	cart.AddProduct(products[1]) // Mouse
	
	total := cart.CalculateTotal()
	fmt.Printf("Cart total: %s\n", FormatCurrency(total, "USD"))
	fmt.Printf("Items in cart: %d\n", cart.GetItemCount())
	
	return nil
}

// demonstratePoints demonstrates point calculations
func demonstratePoints() {
	p1 := Point{X: 0, Y: 0}
	p2 := Point{X: 3, Y: 4}
	
	distance := p1.DistanceTo(p2)
	fmt.Printf("Distance between points: %.2f\n", distance)
}

// main is the main function
func main() {
	fmt.Println("=== Shopping Cart Demo ===")
	if err := demonstrateCart(); err != nil {
		log.Fatalf("Cart demo failed: %v", err)
	}
	
	fmt.Println("\n=== Point Distance Demo ===")
	demonstratePoints()
	
	fmt.Println("\n=== Chunk Slice Demo ===")
	numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	chunks, err := ChunkSlice(numbers, 3)
	if err != nil {
		log.Fatalf("Chunk slice failed: %v", err)
	}
	fmt.Printf("Chunked %v into: %v\n", numbers, chunks)
}