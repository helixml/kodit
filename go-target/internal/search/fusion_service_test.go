package search

import (
	"testing"

	"github.com/helixml/kodit/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestNewFusionService(t *testing.T) {
	svc := NewFusionService()

	assert.Equal(t, 60.0, svc.K())
}

func TestNewFusionServiceWithK(t *testing.T) {
	t.Run("custom k value", func(t *testing.T) {
		svc := NewFusionServiceWithK(30.0)
		assert.Equal(t, 30.0, svc.K())
	})

	t.Run("invalid k defaults to 60", func(t *testing.T) {
		svc := NewFusionServiceWithK(0)
		assert.Equal(t, 60.0, svc.K())

		svc = NewFusionServiceWithK(-10)
		assert.Equal(t, 60.0, svc.K())
	})
}

func TestFusionService_Fuse_EmptyInput(t *testing.T) {
	svc := NewFusionService()

	results := svc.Fuse()
	assert.Empty(t, results)
}

func TestFusionService_Fuse_SingleList(t *testing.T) {
	svc := NewFusionService()

	list := []domain.FusionRequest{
		domain.NewFusionRequest("doc1", 1.0),
		domain.NewFusionRequest("doc2", 0.8),
		domain.NewFusionRequest("doc3", 0.5),
	}

	results := svc.Fuse(list)

	assert.Len(t, results, 3)

	// Results should be ordered by RRF score
	// rank 1: 1/(60+1) = 0.0164
	// rank 2: 1/(60+2) = 0.0161
	// rank 3: 1/(60+3) = 0.0159
	assert.Equal(t, "doc1", results[0].ID())
	assert.Equal(t, "doc2", results[1].ID())
	assert.Equal(t, "doc3", results[2].ID())

	// Check original scores are preserved
	assert.Len(t, results[0].OriginalScores(), 1)
	assert.Equal(t, 1.0, results[0].OriginalScores()[0])
}

func TestFusionService_Fuse_TwoLists(t *testing.T) {
	svc := NewFusionService()

	// List 1: doc1 is ranked 1st, doc2 is ranked 2nd
	list1 := []domain.FusionRequest{
		domain.NewFusionRequest("doc1", 1.0),
		domain.NewFusionRequest("doc2", 0.8),
	}

	// List 2: doc2 is ranked 1st, doc3 is ranked 2nd
	list2 := []domain.FusionRequest{
		domain.NewFusionRequest("doc2", 1.0),
		domain.NewFusionRequest("doc3", 0.8),
	}

	results := svc.Fuse(list1, list2)

	assert.Len(t, results, 3)

	// doc2 appears in both lists (rank 2 + rank 1)
	// RRF: 1/(60+2) + 1/(60+1) = 0.0161 + 0.0164 = 0.0325

	// doc1 appears once (rank 1)
	// RRF: 1/(60+1) = 0.0164

	// doc3 appears once (rank 2)
	// RRF: 1/(60+2) = 0.0161

	assert.Equal(t, "doc2", results[0].ID())
	assert.Equal(t, "doc1", results[1].ID())
	assert.Equal(t, "doc3", results[2].ID())

	// Check doc2 has original scores from both lists
	assert.Len(t, results[0].OriginalScores(), 2)
}

func TestFusionService_Fuse_ThreeLists(t *testing.T) {
	svc := NewFusionService()

	list1 := []domain.FusionRequest{
		domain.NewFusionRequest("doc1", 1.0),
		domain.NewFusionRequest("doc2", 0.9),
	}

	list2 := []domain.FusionRequest{
		domain.NewFusionRequest("doc2", 1.0),
		domain.NewFusionRequest("doc3", 0.9),
	}

	list3 := []domain.FusionRequest{
		domain.NewFusionRequest("doc3", 1.0),
		domain.NewFusionRequest("doc1", 0.9),
	}

	results := svc.Fuse(list1, list2, list3)

	assert.Len(t, results, 3)

	// doc1: rank 1 in list1 + rank 2 in list3 = 1/(60+1) + 1/(60+2)
	// doc2: rank 2 in list1 + rank 1 in list2 = 1/(60+2) + 1/(60+1)
	// doc3: rank 2 in list2 + rank 1 in list3 = 1/(60+2) + 1/(60+1)

	// All three should have the same score (appear once at rank 1, once at rank 2)
	// They should all be tied - order among ties is implementation-defined
}

func TestFusionService_FuseTopK(t *testing.T) {
	svc := NewFusionService()

	list := []domain.FusionRequest{
		domain.NewFusionRequest("doc1", 1.0),
		domain.NewFusionRequest("doc2", 0.9),
		domain.NewFusionRequest("doc3", 0.8),
		domain.NewFusionRequest("doc4", 0.7),
		domain.NewFusionRequest("doc5", 0.6),
	}

	results := svc.FuseTopK(3, list)

	assert.Len(t, results, 3)
	assert.Equal(t, "doc1", results[0].ID())
	assert.Equal(t, "doc2", results[1].ID())
	assert.Equal(t, "doc3", results[2].ID())
}

func TestFusionService_FuseTopK_KLargerThanResults(t *testing.T) {
	svc := NewFusionService()

	list := []domain.FusionRequest{
		domain.NewFusionRequest("doc1", 1.0),
		domain.NewFusionRequest("doc2", 0.9),
	}

	results := svc.FuseTopK(10, list)

	assert.Len(t, results, 2)
}

func TestFusionService_FuseTopK_ZeroK(t *testing.T) {
	svc := NewFusionService()

	list := []domain.FusionRequest{
		domain.NewFusionRequest("doc1", 1.0),
	}

	results := svc.FuseTopK(0, list)

	assert.Len(t, results, 1)
}

func TestFusionService_Fuse_ReciprocalRankFormula(t *testing.T) {
	// Use k=0 for easier verification
	svc := NewFusionServiceWithK(1.0) // k=1 so ranks are 1/(1+rank)

	list := []domain.FusionRequest{
		domain.NewFusionRequest("doc1", 1.0), // rank 1: 1/(1+1) = 0.5
		domain.NewFusionRequest("doc2", 0.9), // rank 2: 1/(1+2) = 0.333
	}

	results := svc.Fuse(list)

	assert.Len(t, results, 2)
	assert.InDelta(t, 0.5, results[0].Score(), 0.001)
	assert.InDelta(t, 0.333, results[1].Score(), 0.001)
}
