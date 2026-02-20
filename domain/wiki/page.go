package wiki

// Page represents a single page in a wiki.
// Pages form a tree via children, and are identified by slug.
type Page struct {
	slug     string
	title    string
	content  string
	position int
	children []Page
}

// NewPage creates a new Page.
func NewPage(slug, title, content string, position int, children []Page) Page {
	if children == nil {
		children = []Page{}
	}
	return Page{
		slug:     slug,
		title:    title,
		content:  content,
		position: position,
		children: children,
	}
}

// Slug returns the URL-safe identifier for this page.
func (p Page) Slug() string { return p.slug }

// Title returns the display title of this page.
func (p Page) Title() string { return p.title }

// Content returns the markdown content of this page.
func (p Page) Content() string { return p.content }

// Position returns the sort order of this page among siblings.
func (p Page) Position() int { return p.position }

// Children returns the child pages.
func (p Page) Children() []Page { return p.children }
