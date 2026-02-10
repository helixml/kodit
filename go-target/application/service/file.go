package service

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/domain/repository"
)

// FileListParams configures file listing.
type FileListParams struct {
	CommitSHA string
}

// FileGetParams configures retrieving a single file.
type FileGetParams struct {
	CommitSHA string
	BlobSHA   string
}

// File provides file query operations.
type File struct {
	fileStore repository.FileStore
}

// NewFile creates a new File service.
func NewFile(fileStore repository.FileStore) *File {
	return &File{
		fileStore: fileStore,
	}
}

// List returns files for a commit.
func (s *File) List(ctx context.Context, params *FileListParams) ([]repository.File, error) {
	if s.fileStore == nil {
		return []repository.File{}, nil
	}
	files, err := s.fileStore.FindByCommitSHA(ctx, params.CommitSHA)
	if err != nil {
		return nil, fmt.Errorf("find files: %w", err)
	}
	return files, nil
}

// Get retrieves a specific file by commit SHA and blob SHA.
func (s *File) Get(ctx context.Context, params *FileGetParams) (repository.File, error) {
	if s.fileStore == nil {
		return repository.File{}, fmt.Errorf("file store not configured")
	}
	file, err := s.fileStore.GetByCommitAndBlobSHA(ctx, params.CommitSHA, params.BlobSHA)
	if err != nil {
		return repository.File{}, fmt.Errorf("get file: %w", err)
	}
	return file, nil
}
