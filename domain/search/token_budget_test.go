package search

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTokenBudget_Valid(t *testing.T) {
	b, err := NewTokenBudget(100)
	require.NoError(t, err)
	require.Equal(t, "hello", b.Truncate("hello"))
}

func TestNewTokenBudget_Invalid(t *testing.T) {
	_, err := NewTokenBudget(0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "maxChars")

	_, err = NewTokenBudget(-1)
	require.Error(t, err)
}

func TestDefaultTokenBudget(t *testing.T) {
	b := DefaultTokenBudget()
	require.Equal(t, "hello", b.Truncate("hello"))
}

func TestTokenBudget_Truncate_Short(t *testing.T) {
	b, _ := NewTokenBudget(10)
	require.Equal(t, "hello", b.Truncate("hello"))
}

func TestTokenBudget_Truncate_Exact(t *testing.T) {
	b, _ := NewTokenBudget(5)
	require.Equal(t, "hello", b.Truncate("hello"))
}

func TestTokenBudget_Truncate_Long(t *testing.T) {
	b, _ := NewTokenBudget(5)
	require.Equal(t, "hello", b.Truncate("hello world"))
}

func TestTokenBudget_Batches_Empty(t *testing.T) {
	b := DefaultTokenBudget()
	require.Nil(t, b.Batches(nil))
	require.Nil(t, b.Batches([]Document{}))
}

func TestTokenBudget_Batches_ByCount(t *testing.T) {
	// Budget large enough for all texts, so the 10-text cap is the limit.
	b, _ := NewTokenBudget(100000)

	docs := make([]Document, 23)
	for i := range docs {
		docs[i] = NewDocument("id", "x")
	}

	batches := b.Batches(docs)
	require.Len(t, batches, 3)
	require.Len(t, batches[0], 10)
	require.Len(t, batches[1], 10)
	require.Len(t, batches[2], 3)
}

func TestTokenBudget_Batches_ByChars(t *testing.T) {
	// 25 chars budget. Each doc is 10 chars, so 2 fit per batch.
	b, _ := NewTokenBudget(25)

	docs := make([]Document, 5)
	for i := range docs {
		docs[i] = NewDocument("id", strings.Repeat("a", 10))
	}

	batches := b.Batches(docs)
	require.Len(t, batches, 3)
	require.Len(t, batches[0], 2)
	require.Len(t, batches[1], 2)
	require.Len(t, batches[2], 1)
}

func TestTokenBudget_Batches_LargeDocOwnBatch(t *testing.T) {
	// 20 char budget. A 50-char doc exceeds budget but gets its own batch.
	b, _ := NewTokenBudget(20)

	docs := []Document{
		NewDocument("a", strings.Repeat("x", 5)),
		NewDocument("b", strings.Repeat("y", 50)),
		NewDocument("c", strings.Repeat("z", 5)),
	}

	batches := b.Batches(docs)
	require.Len(t, batches, 3)
	require.Len(t, batches[0], 1, "small doc alone because next would overflow")
	require.Len(t, batches[1], 1, "large doc alone")
	require.Len(t, batches[2], 1, "trailing small doc")
}

func TestTokenBudget_Batches_TruncatedSizeMeasured(t *testing.T) {
	// Budget 25 chars. Docs are 50 chars but truncated to 25 for measurement.
	// One doc fills the budget, so each is alone.
	b, _ := NewTokenBudget(25)

	docs := make([]Document, 3)
	for i := range docs {
		docs[i] = NewDocument("id", strings.Repeat("a", 50))
	}

	batches := b.Batches(docs)
	require.Len(t, batches, 3)
	require.Len(t, batches[0], 1)
	require.Len(t, batches[1], 1)
	require.Len(t, batches[2], 1)
}

func TestTokenBudget_Batches_SingleDoc(t *testing.T) {
	b := DefaultTokenBudget()
	docs := []Document{NewDocument("id", "hello")}
	batches := b.Batches(docs)
	require.Len(t, batches, 1)
	require.Len(t, batches[0], 1)
}
