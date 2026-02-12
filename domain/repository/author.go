package repository

import "fmt"

// Author represents a Git commit author or committer.
type Author struct {
	name  string
	email string
}

// NewAuthor creates a new Author.
func NewAuthor(name, email string) Author {
	return Author{
		name:  name,
		email: email,
	}
}

// Name returns the author's name.
func (a Author) Name() string { return a.name }

// Email returns the author's email.
func (a Author) Email() string { return a.email }

// IsEmpty returns true if no name is set.
func (a Author) IsEmpty() bool { return a.name == "" }

// String returns a formatted representation (Name <email>).
func (a Author) String() string {
	if a.email == "" {
		return a.name
	}
	return fmt.Sprintf("%s <%s>", a.name, a.email)
}

// Equal returns true if two Author values are equal.
func (a Author) Equal(other Author) bool {
	return a.name == other.name && a.email == other.email
}
