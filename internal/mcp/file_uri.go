package mcp

import "fmt"

// FileURI builds file resource URIs for MCP resource templates.
// Immutable value object — methods return copies.
type FileURI struct {
	repoID    int64
	blobName  string
	path      string
	startLine int
	endLine   int
	page      int
}

// NewFileURI creates a FileURI with the required fields.
func NewFileURI(repoID int64, blobName, path string) FileURI {
	return FileURI{
		repoID:   repoID,
		blobName: blobName,
		path:     path,
	}
}

// WithLineRange returns a copy with line range set.
func (u FileURI) WithLineRange(start, end int) FileURI {
	u.startLine = start
	u.endLine = end
	return u
}

// WithPage returns a copy with page number set (enables raster mode).
func (u FileURI) WithPage(page int) FileURI {
	u.page = page
	return u
}

// String builds the file:// URI string.
func (u FileURI) String() string {
	base := fmt.Sprintf("file://%d/%s/%s", u.repoID, u.blobName, u.path)
	if u.page > 0 {
		return fmt.Sprintf("%s?page=%d&mode=raster", base, u.page)
	}
	if u.startLine > 0 {
		return fmt.Sprintf("%s?lines=L%d-L%d&line_numbers=true", base, u.startLine, u.endLine)
	}
	return base
}
