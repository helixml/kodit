package wiki

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPage(t *testing.T) {
	p := NewPage("getting-started", "Getting Started", "# Getting Started\nWelcome", 2, nil)

	assert.Equal(t, "getting-started", p.Slug())
	assert.Equal(t, "Getting Started", p.Title())
	assert.Equal(t, "# Getting Started\nWelcome", p.Content())
	assert.Equal(t, 2, p.Position())
	assert.Empty(t, p.Children())
}

func TestNewPage_WithChildren(t *testing.T) {
	child1 := NewPage("a", "A", "a", 0, nil)
	child2 := NewPage("b", "B", "b", 1, nil)
	parent := NewPage("parent", "Parent", "parent", 0, []Page{child1, child2})

	assert.Len(t, parent.Children(), 2)
	assert.Equal(t, "a", parent.Children()[0].Slug())
	assert.Equal(t, "b", parent.Children()[1].Slug())
}

func TestNewPage_NilChildrenBecomesEmpty(t *testing.T) {
	p := NewPage("x", "X", "x", 0, nil)
	assert.NotNil(t, p.Children())
	assert.Empty(t, p.Children())
}
