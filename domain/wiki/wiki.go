package wiki

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Wiki represents a complete wiki generated from a repository.
// It is a value object constructed by deserializing an enrichment's content.
type Wiki struct {
	pages []Page
}

// NewWiki creates a Wiki from a list of pages.
func NewWiki(pages []Page) Wiki {
	if pages == nil {
		pages = []Page{}
	}
	return Wiki{pages: pages}
}

// Pages returns the full page tree.
func (w Wiki) Pages() []Page { return w.pages }

// Page finds a single page by slug, searching the entire tree.
// Returns the page and true if found, or an empty page and false otherwise.
func (w Wiki) Page(slug string) (Page, bool) {
	return findPage(w.pages, slug)
}

// PathIndex returns a map from slug to hierarchical path for every page.
// For example, a child page "database-layer" under "architecture" maps to
// "architecture/database-layer". Top-level pages map to their slug.
func (w Wiki) PathIndex() map[string]string {
	index := make(map[string]string)
	buildPathIndex(w.pages, "", index)
	return index
}

// PageByPath resolves a page by its hierarchical path (e.g. "architecture/database-layer").
// It walks the tree level by level, splitting on "/".
func (w Wiki) PageByPath(path string) (Page, bool) {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return Page{}, false
	}

	segments := strings.Split(path, "/")
	pages := w.pages
	for i, seg := range segments {
		found := false
		for _, p := range pages {
			if p.slug == seg {
				if i == len(segments)-1 {
					return p, true
				}
				pages = p.children
				found = true
				break
			}
		}
		if !found {
			return Page{}, false
		}
	}
	return Page{}, false
}

// JSON serializes the wiki to JSON for storage in enrichment content.
func (w Wiki) JSON() (string, error) {
	data := wikiJSON{Pages: pagesToJSON(w.pages)}
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal wiki: %w", err)
	}
	return string(bytes), nil
}

// ParseWiki deserializes a wiki from JSON content stored in an enrichment.
func ParseWiki(content string) (Wiki, error) {
	var data wikiJSON
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return Wiki{}, fmt.Errorf("unmarshal wiki: %w", err)
	}
	return NewWiki(pagesFromJSON(data.Pages)), nil
}

func buildPathIndex(pages []Page, prefix string, index map[string]string) {
	for _, p := range pages {
		path := p.slug
		if prefix != "" {
			path = prefix + "/" + p.slug
		}
		index[p.slug] = path
		buildPathIndex(p.children, path, index)
	}
}

func findPage(pages []Page, slug string) (Page, bool) {
	for _, p := range pages {
		if p.slug == slug {
			return p, true
		}
		if found, ok := findPage(p.children, slug); ok {
			return found, true
		}
	}
	return Page{}, false
}

// JSON serialization types (private).

type wikiJSON struct {
	Pages []pageJSON `json:"pages"`
}

type pageJSON struct {
	Slug     string     `json:"slug"`
	Title    string     `json:"title"`
	Position int        `json:"position"`
	Content  string     `json:"content"`
	Children []pageJSON `json:"children,omitempty"`
}

func pagesToJSON(pages []Page) []pageJSON {
	result := make([]pageJSON, len(pages))
	for i, p := range pages {
		result[i] = pageJSON{
			Slug:     p.slug,
			Title:    p.title,
			Position: p.position,
			Content:  p.content,
			Children: pagesToJSON(p.children),
		}
	}
	return result
}

func pagesFromJSON(data []pageJSON) []Page {
	result := make([]Page, len(data))
	for i, d := range data {
		result[i] = NewPage(d.Slug, d.Title, d.Content, d.Position, pagesFromJSON(d.Children))
	}
	return result
}
