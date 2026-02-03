package git

import "time"

// File represents a file in a Git commit.
type File struct {
	id        int64
	commitSHA string
	path      string
	blobSHA   string
	mimeType  string
	extension string
	language  string
	size      int64
	createdAt time.Time
}

// NewFile creates a new File.
func NewFile(commitSHA, path, language string, size int64) File {
	return File{
		commitSHA: commitSHA,
		path:      path,
		language:  language,
		size:      size,
		createdAt: time.Now(),
	}
}

// NewFileWithDetails creates a new File with all details.
func NewFileWithDetails(commitSHA, path, blobSHA, mimeType, extension string, size int64) File {
	return File{
		commitSHA: commitSHA,
		path:      path,
		blobSHA:   blobSHA,
		mimeType:  mimeType,
		extension: extension,
		size:      size,
		createdAt: time.Now(),
	}
}

// ReconstructFile reconstructs a File from persistence.
func ReconstructFile(
	id int64,
	commitSHA, path, blobSHA, mimeType, extension, language string,
	size int64,
	createdAt time.Time,
) File {
	return File{
		id:        id,
		commitSHA: commitSHA,
		path:      path,
		blobSHA:   blobSHA,
		mimeType:  mimeType,
		extension: extension,
		language:  language,
		size:      size,
		createdAt: createdAt,
	}
}

// ID returns the file ID.
func (f File) ID() int64 { return f.id }

// CommitSHA returns the commit SHA this file belongs to.
func (f File) CommitSHA() string { return f.commitSHA }

// Path returns the file path relative to repository root.
func (f File) Path() string { return f.path }

// Language returns the detected programming language.
func (f File) Language() string { return f.language }

// Size returns the file size in bytes.
func (f File) Size() int64 { return f.size }

// BlobSHA returns the blob SHA.
func (f File) BlobSHA() string { return f.blobSHA }

// MimeType returns the MIME type.
func (f File) MimeType() string { return f.mimeType }

// Extension returns the file extension.
func (f File) Extension() string { return f.extension }

// CreatedAt returns the creation timestamp.
func (f File) CreatedAt() time.Time { return f.createdAt }

// WithID returns a new File with the specified ID.
func (f File) WithID(id int64) File {
	f.id = id
	return f
}
