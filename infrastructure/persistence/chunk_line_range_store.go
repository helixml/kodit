package persistence

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/domain/chunk"
	"github.com/helixml/kodit/internal/database"
)

// ChunkLineRangeStore implements chunk.LineRangeStore using GORM.
type ChunkLineRangeStore struct {
	database.Repository[chunk.LineRange, ChunkLineRangeModel]
}

// NewChunkLineRangeStore creates a new ChunkLineRangeStore.
func NewChunkLineRangeStore(db database.Database) ChunkLineRangeStore {
	return ChunkLineRangeStore{
		Repository: database.NewRepository[chunk.LineRange, ChunkLineRangeModel](db, ChunkLineRangeMapper{}, "chunk_line_range"),
	}
}

// Save creates or updates a chunk line range.
func (s ChunkLineRangeStore) Save(ctx context.Context, lr chunk.LineRange) (chunk.LineRange, error) {
	model := s.Mapper().ToModel(lr)

	if model.ID == 0 {
		result := s.DB(ctx).Create(&model)
		if result.Error != nil {
			return chunk.LineRange{}, fmt.Errorf("create chunk line range: %w", result.Error)
		}
	} else {
		result := s.DB(ctx).Save(&model)
		if result.Error != nil {
			return chunk.LineRange{}, fmt.Errorf("update chunk line range: %w", result.Error)
		}
	}

	return s.Mapper().ToDomain(model), nil
}

// Delete removes a chunk line range.
func (s ChunkLineRangeStore) Delete(ctx context.Context, lr chunk.LineRange) error {
	model := s.Mapper().ToModel(lr)
	result := s.DB(ctx).Delete(&model)
	if result.Error != nil {
		return fmt.Errorf("delete chunk line range: %w", result.Error)
	}
	return nil
}
