// Package repository provides Git repository domain types.
package repository

// WorkingCopy represents a local filesystem clone of a Git repository.
type WorkingCopy struct {
	path string
	uri  string
}

// NewWorkingCopy creates a new WorkingCopy.
func NewWorkingCopy(path, uri string) WorkingCopy {
	return WorkingCopy{
		path: path,
		uri:  uri,
	}
}

// Path returns the local filesystem path.
func (w WorkingCopy) Path() string { return w.path }

// URI returns the repository URI (remote URL or local path).
func (w WorkingCopy) URI() string { return w.uri }

// IsEmpty returns true if no path is set.
func (w WorkingCopy) IsEmpty() bool { return w.path == "" }

// Equal returns true if two WorkingCopy values are equal.
func (w WorkingCopy) Equal(other WorkingCopy) bool {
	return w.path == other.path && w.uri == other.uri
}
