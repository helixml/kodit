package dto

// WikiTreeNode represents a wiki page in the navigation tree (no content).
type WikiTreeNode struct {
	Slug     string         `json:"slug"`
	Title    string         `json:"title"`
	Path     string         `json:"path"`
	Children []WikiTreeNode `json:"children,omitempty"`
}

// WikiTreeResponse is the JSON response for the wiki tree endpoint.
type WikiTreeResponse struct {
	Data []WikiTreeNode `json:"data"`
}
