// Package main provides data models for the example project.
package main

import (
	"errors"
)

// Point represents a point in 2D space
type Point struct {
	X, Y float64
}

// DistanceTo calculates distance to another point
func (p Point) DistanceTo(other Point) float64 {
	return CalculateDistance(p.X, p.Y, other.X, other.Y)
}

// Product represents a product with name, price, and inventory
type Product struct {
	Name      string
	Price     float64
	Inventory int
}

// NewProduct creates a new product with validation
func NewProduct(name string, price float64, inventory int) (*Product, error) {
	if err := ValidatePositive(price, "price"); err != nil {
		return nil, err
	}
	if inventory < 0 {
		return nil, errors.New("inventory cannot be negative")
	}
	
	return &Product{
		Name:      name,
		Price:     price,
		Inventory: inventory,
	}, nil
}

// TotalValue calculates total inventory value
func (p *Product) TotalValue() float64 {
	return p.Price * float64(p.Inventory)
}

// ShoppingCart represents a shopping cart that holds products
type ShoppingCart struct {
	Items []*Product
}

// NewShoppingCart creates a new shopping cart
func NewShoppingCart() *ShoppingCart {
	return &ShoppingCart{
		Items: make([]*Product, 0),
	}
}

// AddProduct adds a product to the cart
func (sc *ShoppingCart) AddProduct(product *Product) {
	sc.Items = append(sc.Items, product)
}

// CalculateTotal calculates total cart value
func (sc *ShoppingCart) CalculateTotal() float64 {
	total := 0.0
	for _, item := range sc.Items {
		total += item.Price
	}
	return total
}

// GetItemCount gets number of items in cart
func (sc *ShoppingCart) GetItemCount() int {
	return len(sc.Items)
}