package main

import "fmt"

// Person represents a person with a name and age
type Person struct {
	Name string
	Age  int
}

// add returns the sum of two integers
func add(a, b int) int {
	return a + b
}

func main() {
	// Create a new person
	person := Person{
		Name: "John",
		Age:  30,
	}

	// Print the person's information
	fmt.Printf("Person: %+v\n", person)

	// Print a simple message
	fmt.Println("Hello, Go!")
}
