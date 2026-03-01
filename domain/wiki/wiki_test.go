package wiki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWiki_NilPages(t *testing.T) {
	w := NewWiki(nil)
	assert.Empty(t, w.Pages())
}

func TestNewWiki_Pages(t *testing.T) {
	pages := []Page{
		NewPage("overview", "Overview", "# Overview", 0, nil),
		NewPage("api", "API", "# API", 1, nil),
	}
	w := NewWiki(pages)
	assert.Len(t, w.Pages(), 2)
	assert.Equal(t, "overview", w.Pages()[0].Slug())
	assert.Equal(t, "api", w.Pages()[1].Slug())
}

func TestWiki_PageBySlug(t *testing.T) {
	child := NewPage("db", "Database", "# DB", 0, nil)
	parent := NewPage("arch", "Architecture", "# Arch", 0, []Page{child})
	w := NewWiki([]Page{parent})

	found, ok := w.Page("arch")
	assert.True(t, ok)
	assert.Equal(t, "Architecture", found.Title())

	found, ok = w.Page("db")
	assert.True(t, ok)
	assert.Equal(t, "Database", found.Title())

	_, ok = w.Page("nonexistent")
	assert.False(t, ok)
}

func TestWiki_PageByPath(t *testing.T) {
	child := NewPage("db", "Database", "# DB", 0, nil)
	parent := NewPage("arch", "Architecture", "# Arch", 0, []Page{child})
	top := NewPage("overview", "Overview", "# Overview", 0, nil)
	w := NewWiki([]Page{top, parent})

	found, ok := w.PageByPath("overview")
	assert.True(t, ok)
	assert.Equal(t, "Overview", found.Title())

	found, ok = w.PageByPath("arch/db")
	assert.True(t, ok)
	assert.Equal(t, "Database", found.Title())

	found, ok = w.PageByPath("arch")
	assert.True(t, ok)
	assert.Equal(t, "Architecture", found.Title())

	_, ok = w.PageByPath("db")
	assert.False(t, ok, "bare child slug should not match at top level")

	_, ok = w.PageByPath("")
	assert.False(t, ok)

	_, ok = w.PageByPath("arch/nonexistent")
	assert.False(t, ok)
}

func TestWiki_PathIndex(t *testing.T) {
	child := NewPage("db", "Database", "# DB", 0, nil)
	parent := NewPage("arch", "Architecture", "# Arch", 0, []Page{child})
	top := NewPage("overview", "Overview", "# Overview", 0, nil)
	w := NewWiki([]Page{top, parent})

	idx := w.PathIndex()
	assert.Equal(t, "overview", idx["overview"])
	assert.Equal(t, "arch", idx["arch"])
	assert.Equal(t, "arch/db", idx["db"])
}

func TestWiki_PathIndex_DuplicateSlug(t *testing.T) {
	// Two pages with the same slug at different levels.
	// First-writer-wins, matching findPage behaviour.
	topFoo := NewPage("foo", "Top Foo", "top", 0, nil)
	child := NewPage("foo", "Child Foo", "child", 0, nil)
	parent := NewPage("bar", "Bar", "bar", 1, []Page{child})
	w := NewWiki([]Page{topFoo, parent})

	idx := w.PathIndex()
	assert.Equal(t, "foo", idx["foo"], "first-writer-wins: top-level foo")

	// findPage should also return the top-level foo.
	found, ok := w.Page("foo")
	assert.True(t, ok)
	assert.Equal(t, "Top Foo", found.Title())
}

func TestWiki_JSONRoundTrip(t *testing.T) {
	child := NewPage("db", "Database", "# DB content", 0, nil)
	parent := NewPage("arch", "Architecture", "# Arch content", 0, []Page{child})
	top := NewPage("overview", "Overview", "# Overview content", 1, nil)
	original := NewWiki([]Page{top, parent})

	jsonStr, err := original.JSON()
	require.NoError(t, err)

	parsed, err := ParseWiki(jsonStr)
	require.NoError(t, err)

	assert.Len(t, parsed.Pages(), 2)
	assert.Equal(t, "overview", parsed.Pages()[0].Slug())
	assert.Equal(t, "# Overview content", parsed.Pages()[0].Content())
	assert.Equal(t, 1, parsed.Pages()[0].Position())

	assert.Equal(t, "arch", parsed.Pages()[1].Slug())
	assert.Len(t, parsed.Pages()[1].Children(), 1)
	assert.Equal(t, "db", parsed.Pages()[1].Children()[0].Slug())
	assert.Equal(t, "# DB content", parsed.Pages()[1].Children()[0].Content())
}

func TestParseWiki_InvalidJSON(t *testing.T) {
	_, err := ParseWiki("not json")
	assert.Error(t, err)
}
