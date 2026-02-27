package service

import (
	"context"
	"testing"

	"github.com/helixml/kodit/domain/chunk"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

func TestEnrichment_LineRanges(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()

	enrichmentStore := persistence.NewEnrichmentStore(db)
	lineRangeStore := persistence.NewChunkLineRangeStore(db)

	// Seed an enrichment.
	e := enrichment.NewEnrichment(
		enrichment.TypeDevelopment,
		enrichment.SubtypeSnippet,
		enrichment.EntityTypeSnippet,
		"func main() {}",
	)
	saved, err := enrichmentStore.Save(ctx, e)
	if err != nil {
		t.Fatalf("save enrichment: %v", err)
	}

	// Seed a line range for the enrichment.
	lr := chunk.NewLineRange(saved.ID(), 10, 25)
	_, err = lineRangeStore.Save(ctx, lr)
	if err != nil {
		t.Fatalf("save line range: %v", err)
	}

	svc := NewEnrichment(enrichmentStore, nil, nil, nil, nil, lineRangeStore)

	result, err := svc.LineRanges(ctx, []int64{saved.ID()})
	if err != nil {
		t.Fatalf("LineRanges: %v", err)
	}

	idStr := "1"
	got, ok := result[idStr]
	if !ok {
		t.Fatalf("missing line range for enrichment ID %s; keys: %v", idStr, result)
	}
	if got.StartLine() != 10 {
		t.Errorf("StartLine = %d, want 10", got.StartLine())
	}
	if got.EndLine() != 25 {
		t.Errorf("EndLine = %d, want 25", got.EndLine())
	}
}

func TestEnrichment_LineRanges_EmptyIDs(t *testing.T) {
	svc := NewEnrichment(nil, nil, nil, nil, nil, nil)

	result, err := svc.LineRanges(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}
