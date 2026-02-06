package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/helixml/kodit/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FileStore implements repository.FileStore using GORM.
type FileStore struct {
	db     Database
	mapper FileMapper
}

// NewFileStore creates a new FileStore.
func NewFileStore(db Database) FileStore {
	return FileStore{
		db:     db,
		mapper: FileMapper{},
	}
}

// Get retrieves a file by ID.
func (s FileStore) Get(ctx context.Context, id int64) (repository.File, error) {
	var model FileModel
	result := s.db.Session(ctx).Where("id = ?", id).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return repository.File{}, fmt.Errorf("%w: file with id %d", ErrNotFound, id)
		}
		return repository.File{}, fmt.Errorf("get file: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// Save creates or updates a file.
func (s FileStore) Save(ctx context.Context, file repository.File) (repository.File, error) {
	model := s.mapper.ToModel(file)

	result := s.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "commit_sha"}, {Name: "path"}},
		DoUpdates: clause.AssignmentColumns([]string{"blob_sha", "mime_type", "extension", "size"}),
	}).Create(&model)

	if result.Error != nil {
		return repository.File{}, fmt.Errorf("save file: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// SaveAll creates or updates multiple files.
func (s FileStore) SaveAll(ctx context.Context, files []repository.File) ([]repository.File, error) {
	if len(files) == 0 {
		return []repository.File{}, nil
	}

	models := make([]FileModel, len(files))
	for i, f := range files {
		models[i] = s.mapper.ToModel(f)
	}

	result := s.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "commit_sha"}, {Name: "path"}},
		DoUpdates: clause.AssignmentColumns([]string{"blob_sha", "mime_type", "extension", "size"}),
	}).Create(&models)

	if result.Error != nil {
		return nil, fmt.Errorf("save files: %w", result.Error)
	}

	saved := make([]repository.File, len(models))
	for i, m := range models {
		saved[i] = s.mapper.ToDomain(m)
	}
	return saved, nil
}

// Delete removes a file.
func (s FileStore) Delete(ctx context.Context, file repository.File) error {
	result := s.db.Session(ctx).Where("commit_sha = ? AND path = ?", file.CommitSHA(), file.Path()).Delete(&FileModel{})
	if result.Error != nil {
		return fmt.Errorf("delete file: %w", result.Error)
	}
	return nil
}

// FindByCommitSHA retrieves all files for a commit.
func (s FileStore) FindByCommitSHA(ctx context.Context, sha string) ([]repository.File, error) {
	var models []FileModel
	result := s.db.Session(ctx).Where("commit_sha = ?", sha).Order("path ASC").Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find files by commit: %w", result.Error)
	}

	files := make([]repository.File, len(models))
	for i, m := range models {
		files[i] = s.mapper.ToDomain(m)
	}
	return files, nil
}

// DeleteByCommitSHA deletes all files for a commit.
func (s FileStore) DeleteByCommitSHA(ctx context.Context, sha string) error {
	result := s.db.Session(ctx).Where("commit_sha = ?", sha).Delete(&FileModel{})
	if result.Error != nil {
		return fmt.Errorf("delete files by commit: %w", result.Error)
	}
	return nil
}

// GetByCommitAndBlobSHA retrieves a file by commit SHA and blob SHA.
func (s FileStore) GetByCommitAndBlobSHA(ctx context.Context, commitSHA, blobSHA string) (repository.File, error) {
	var model FileModel
	result := s.db.Session(ctx).Where("commit_sha = ? AND blob_sha = ?", commitSHA, blobSHA).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return repository.File{}, fmt.Errorf("%w: file with blob %s at commit %s", ErrNotFound, blobSHA, commitSHA)
		}
		return repository.File{}, fmt.Errorf("get file: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// GetByCommitAndPath retrieves a file by commit SHA and path.
func (s FileStore) GetByCommitAndPath(ctx context.Context, commitSHA, path string) (repository.File, error) {
	var model FileModel
	result := s.db.Session(ctx).Where("commit_sha = ? AND path = ?", commitSHA, path).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return repository.File{}, fmt.Errorf("%w: file %s at commit %s", ErrNotFound, path, commitSHA)
		}
		return repository.File{}, fmt.Errorf("get file: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// Find retrieves files matching a query.
func (s FileStore) Find(ctx context.Context, query Query) ([]repository.File, error) {
	var models []FileModel
	result := query.Apply(s.db.Session(ctx).Model(&FileModel{})).Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find files: %w", result.Error)
	}

	files := make([]repository.File, len(models))
	for i, m := range models {
		files[i] = s.mapper.ToDomain(m)
	}
	return files, nil
}
