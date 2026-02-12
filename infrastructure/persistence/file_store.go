package persistence

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm/clause"
)

// FileStore implements repository.FileStore using GORM.
type FileStore struct {
	database.Repository[repository.File, FileModel]
}

// NewFileStore creates a new FileStore.
func NewFileStore(db database.Database) FileStore {
	return FileStore{
		Repository: database.NewRepository[repository.File, FileModel](db, FileMapper{}, "file"),
	}
}

// Save creates or updates a file.
func (s FileStore) Save(ctx context.Context, file repository.File) (repository.File, error) {
	model := s.Mapper().ToModel(file)

	result := s.DB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "commit_sha"}, {Name: "path"}},
		DoUpdates: clause.AssignmentColumns([]string{"blob_sha", "mime_type", "extension", "size"}),
	}).Create(&model)

	if result.Error != nil {
		return repository.File{}, fmt.Errorf("save file: %w", result.Error)
	}
	return s.Mapper().ToDomain(model), nil
}

// SaveAll creates or updates multiple files.
func (s FileStore) SaveAll(ctx context.Context, files []repository.File) ([]repository.File, error) {
	if len(files) == 0 {
		return []repository.File{}, nil
	}

	models := make([]FileModel, len(files))
	for i, f := range files {
		models[i] = s.Mapper().ToModel(f)
	}

	result := s.DB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "commit_sha"}, {Name: "path"}},
		DoUpdates: clause.AssignmentColumns([]string{"blob_sha", "mime_type", "extension", "size"}),
	}).Create(&models)

	if result.Error != nil {
		return nil, fmt.Errorf("save files: %w", result.Error)
	}

	saved := make([]repository.File, len(models))
	for i, m := range models {
		saved[i] = s.Mapper().ToDomain(m)
	}
	return saved, nil
}

// Delete removes a file.
func (s FileStore) Delete(ctx context.Context, file repository.File) error {
	result := s.DB(ctx).Where("commit_sha = ? AND path = ?", file.CommitSHA(), file.Path()).Delete(&FileModel{})
	if result.Error != nil {
		return fmt.Errorf("delete file: %w", result.Error)
	}
	return nil
}
