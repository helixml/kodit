package service

import "github.com/helixml/kodit/domain/repository"

// File provides read access to files, extensible with bespoke methods later.
type File struct {
	repository.Collection[repository.File]
}

// NewFile creates a new File service wrapping the given store.
func NewFile(store repository.FileStore) *File {
	return &File{Collection: repository.NewCollection[repository.File](store)}
}
