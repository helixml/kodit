package search

import (
	"math"
	"testing"
)

func TestFusion_Fuse_SingleList(t *testing.T) {
	fusion := NewFusion() // k = 60

	list := []FusionRequest{
		NewFusionRequest("a", 0.9),
		NewFusionRequest("b", 0.7),
		NewFusionRequest("c", 0.5),
	}

	results := fusion.Fuse(list)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// With 0-indexed ranks and k=60:
	// rank 0: 1/(60+0) = 1/60
	// rank 1: 1/(60+1) = 1/61
	// rank 2: 1/(60+2) = 1/62
	expectedScores := []float64{1.0 / 60.0, 1.0 / 61.0, 1.0 / 62.0}
	expectedIDs := []string{"a", "b", "c"}

	for i, r := range results {
		if r.ID() != expectedIDs[i] {
			t.Errorf("result[%d]: expected ID %q, got %q", i, expectedIDs[i], r.ID())
		}
		if math.Abs(r.Score()-expectedScores[i]) > 1e-10 {
			t.Errorf("result[%d]: expected score %f, got %f", i, expectedScores[i], r.Score())
		}
	}
}

func TestFusion_Fuse_TwoLists(t *testing.T) {
	fusion := NewFusion()

	list1 := []FusionRequest{
		NewFusionRequest("a", 0.9),
		NewFusionRequest("b", 0.7),
	}
	list2 := []FusionRequest{
		NewFusionRequest("b", 0.8),
		NewFusionRequest("c", 0.6),
	}

	results := fusion.Fuse(list1, list2)

	// "b" appears in both lists: rank 1 in list1, rank 0 in list2
	// b score = 1/(60+1) + 1/(60+0) = 1/61 + 1/60
	// "a" appears once at rank 0: 1/(60+0) = 1/60
	// "c" appears once at rank 1: 1/(60+1) = 1/61

	scores := make(map[string]float64)
	for _, r := range results {
		scores[r.ID()] = r.Score()
	}

	expectedB := 1.0/61.0 + 1.0/60.0
	if math.Abs(scores["b"]-expectedB) > 1e-10 {
		t.Errorf("b: expected score %f, got %f", expectedB, scores["b"])
	}

	expectedA := 1.0 / 60.0
	if math.Abs(scores["a"]-expectedA) > 1e-10 {
		t.Errorf("a: expected score %f, got %f", expectedA, scores["a"])
	}

	expectedC := 1.0 / 61.0
	if math.Abs(scores["c"]-expectedC) > 1e-10 {
		t.Errorf("c: expected score %f, got %f", expectedC, scores["c"])
	}

	// b should be first (highest score)
	if results[0].ID() != "b" {
		t.Errorf("expected first result to be 'b', got %q", results[0].ID())
	}
}

func TestFusion_FuseTopK(t *testing.T) {
	fusion := NewFusion()

	list := []FusionRequest{
		NewFusionRequest("a", 0.9),
		NewFusionRequest("b", 0.7),
		NewFusionRequest("c", 0.5),
	}

	results := fusion.FuseTopK(2, list)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID() != "a" {
		t.Errorf("expected first result 'a', got %q", results[0].ID())
	}
	if results[1].ID() != "b" {
		t.Errorf("expected second result 'b', got %q", results[1].ID())
	}
}

func TestFusion_Fuse_EmptyInput(t *testing.T) {
	fusion := NewFusion()
	results := fusion.Fuse()
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestFusion_CustomK(t *testing.T) {
	fusion := NewFusionWithK(10)
	if fusion.K() != 10.0 {
		t.Errorf("expected K=10, got %f", fusion.K())
	}

	list := []FusionRequest{
		NewFusionRequest("a", 0.9),
	}
	results := fusion.Fuse(list)

	// rank 0 with k=10: 1/(10+0) = 1/10 = 0.1
	expected := 0.1
	if math.Abs(results[0].Score()-expected) > 1e-10 {
		t.Errorf("expected score %f, got %f", expected, results[0].Score())
	}
}

func TestFusion_InvalidK(t *testing.T) {
	fusion := NewFusionWithK(-5)
	if fusion.K() != 60.0 {
		t.Errorf("expected default K=60 for invalid input, got %f", fusion.K())
	}
}

func TestFusion_Fuse_TieBreaking(t *testing.T) {
	fusion := NewFusion()

	// All items at same rank in their respective single-item lists
	// produce identical RRF scores. Tie-breaking should sort by ID.
	list1 := []FusionRequest{NewFusionRequest("c", 0.5)}
	list2 := []FusionRequest{NewFusionRequest("a", 0.5)}
	list3 := []FusionRequest{NewFusionRequest("b", 0.5)}

	results := fusion.Fuse(list1, list2, list3)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// All have same RRF score (1/(60+0)), so should be sorted by ID ascending
	expectedOrder := []string{"a", "b", "c"}
	for i, r := range results {
		if r.ID() != expectedOrder[i] {
			t.Errorf("result[%d]: expected ID %q, got %q", i, expectedOrder[i], r.ID())
		}
	}

	// Verify determinism: run again and expect same order
	results2 := fusion.Fuse(list1, list2, list3)
	for i, r := range results2 {
		if r.ID() != results[i].ID() {
			t.Errorf("non-deterministic: run2[%d]=%q != run1[%d]=%q", i, r.ID(), i, results[i].ID())
		}
	}
}

func TestFusion_Fuse_AsymmetricLists(t *testing.T) {
	fusion := NewFusion()

	// One list has many results, another has none overlapping
	list1 := []FusionRequest{
		NewFusionRequest("a", 0.9),
		NewFusionRequest("b", 0.8),
		NewFusionRequest("c", 0.7),
	}
	list2 := []FusionRequest{
		NewFusionRequest("d", 0.6),
	}

	results := fusion.Fuse(list1, list2)

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// "a" is rank 0 in list1: 1/60
	// "d" is rank 0 in list2: 1/60
	// Both should have same score, tie-broken by ID
	scores := make(map[string]float64)
	for _, r := range results {
		scores[r.ID()] = r.Score()
	}

	if math.Abs(scores["a"]-scores["d"]) > 1e-10 {
		t.Errorf("a and d should have equal scores: a=%f, d=%f", scores["a"], scores["d"])
	}

	// "a" < "d" alphabetically, so "a" should come first in tie
	if results[0].ID() != "a" {
		t.Errorf("expected first result 'a' (tie-break by ID), got %q", results[0].ID())
	}
}
