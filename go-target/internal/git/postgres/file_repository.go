package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/git"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FileRepository implements git.FileRepository using PostgreSQL.
type FileRepository struct {
	db     database.Database
	mapper FileMapper
}

// NewFileRepository creates a new FileRepository.
func NewFileRepository(db database.Database) *FileRepository {
	return &FileRepository{
		db:     db,
		mapper: FileMapper{},
	}
}

// Get retrieves a file by ID (not typically used since composite key).
func (r *FileRepository) Get(ctx context.Context, id int64) (git.File, error) {
	// Files use composite key (commit_sha, path), not integer ID
	return git.File{}, fmt.Errorf("%w: files use composite key", database.ErrNotFound)
}

// Find retrieves files matching a query.
func (r *FileRepository) Find(ctx context.Context, query database.Query) ([]git.File, error) {
	var entities []FileEntity
	result := query.Apply(r.db.Session(ctx).Model(&FileEntity{})).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find files: %w", result.Error)
	}

	files := make([]git.File, len(entities))
	for i, e := range entities {
		files[i] = r.mapper.ToDomain(e)
	}
	return files, nil
}

// Save creates or updates a file.
func (r *FileRepository) Save(ctx context.Context, file git.File) (git.File, error) {
	entity := r.mapper.ToDatabase(file)

	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "commit_sha"}, {Name: "path"}},
		DoUpdates: clause.AssignmentColumns([]string{"blob_sha", "mime_type", "extension", "size"}),
	}).Create(&entity)

	if result.Error != nil {
		return git.File{}, fmt.Errorf("save file: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// SaveAll creates or updates multiple files.
func (r *FileRepository) SaveAll(ctx context.Context, files []git.File) ([]git.File, error) {
	if len(files) == 0 {
		return nil, nil
	}

	entities := make([]FileEntity, len(files))
	for i, f := range files {
		entities[i] = r.mapper.ToDatabase(f)
	}

	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "commit_sha"}, {Name: "path"}},
		DoUpdates: clause.AssignmentColumns([]string{"blob_sha", "mime_type", "extension", "size"}),
	}).Create(&entities)

	if result.Error != nil {
		return nil, fmt.Errorf("save files: %w", result.Error)
	}

	saved := make([]git.File, len(entities))
	for i, e := range entities {
		saved[i] = r.mapper.ToDomain(e)
	}
	return saved, nil
}

// Delete removes a file.
func (r *FileRepository) Delete(ctx context.Context, file git.File) error {
	result := r.db.Session(ctx).Where("commit_sha = ? AND path = ?", file.CommitSHA(), file.Path()).Delete(&FileEntity{})
	if result.Error != nil {
		return fmt.Errorf("delete file: %w", result.Error)
	}
	return nil
}

// FindByCommitSHA retrieves all files for a commit.
func (r *FileRepository) FindByCommitSHA(ctx context.Context, sha string) ([]git.File, error) {
	var entities []FileEntity
	result := r.db.Session(ctx).Where("commit_sha = ?", sha).Order("path ASC").Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find files by commit: %w", result.Error)
	}

	files := make([]git.File, len(entities))
	for i, e := range entities {
		files[i] = r.mapper.ToDomain(e)
	}
	return files, nil
}

// DeleteByCommitSHA deletes all files for a commit.
func (r *FileRepository) DeleteByCommitSHA(ctx context.Context, sha string) error {
	result := r.db.Session(ctx).Where("commit_sha = ?", sha).Delete(&FileEntity{})
	if result.Error != nil {
		return fmt.Errorf("delete files by commit: %w", result.Error)
	}
	return nil
}

// GetByCommitAndPath retrieves a file by commit SHA and path.
func (r *FileRepository) GetByCommitAndPath(ctx context.Context, sha, path string) (git.File, error) {
	var entity FileEntity
	result := r.db.Session(ctx).Where("commit_sha = ? AND path = ?", sha, path).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return git.File{}, fmt.Errorf("%w: file %s at %s", database.ErrNotFound, path, sha)
		}
		return git.File{}, fmt.Errorf("get file: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}
