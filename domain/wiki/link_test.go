package wiki

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewrittenContent_KnownSlug(t *testing.T) {
	pathIndex := map[string]string{
		"arch": "arch",
		"db":   "arch/db",
	}
	content := "See [Architecture](arch) and [Database](db) for details."
	result := NewRewrittenContent(content, pathIndex, "/api/v1/repositories/1/wiki", ".md")

	expected := "See [Architecture](/api/v1/repositories/1/wiki/arch.md) and [Database](/api/v1/repositories/1/wiki/arch/db.md) for details."
	assert.Equal(t, expected, result.String())
}

func TestRewrittenContent_AbsoluteURL(t *testing.T) {
	pathIndex := map[string]string{"arch": "arch"}
	content := "See [Google](https://google.com) and [Local](/path)."
	result := NewRewrittenContent(content, pathIndex, "/api/v1/repositories/1/wiki", ".md")

	assert.Equal(t, content, result.String(), "absolute URLs should be unchanged")
}

func TestRewrittenContent_UnknownSlug(t *testing.T) {
	pathIndex := map[string]string{"arch": "arch"}
	content := "See [Missing](unknown-page) for details."
	result := NewRewrittenContent(content, pathIndex, "/api/v1/repositories/1/wiki", ".md")

	assert.Equal(t, content, result.String(), "unknown slugs should be unchanged")
}

func TestRewrittenContent_NoLinks(t *testing.T) {
	pathIndex := map[string]string{"arch": "arch"}
	content := "Plain text with no links."
	result := NewRewrittenContent(content, pathIndex, "/prefix", ".md")

	assert.Equal(t, content, result.String())
}

func TestRewrittenContent_EmptyContent(t *testing.T) {
	result := NewRewrittenContent("", map[string]string{}, "/prefix", ".md")
	assert.Equal(t, "", result.String())
}

func TestRewrittenContent_HttpURL(t *testing.T) {
	pathIndex := map[string]string{"arch": "arch"}
	content := "See [Old](http://example.com)."
	result := NewRewrittenContent(content, pathIndex, "/prefix", ".md")

	assert.Equal(t, content, result.String(), "http URLs should be unchanged")
}
