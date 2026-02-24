package search

import (
	"strings"
	"testing"
	"unicode/utf8"

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

func TestTokenBudget_Batches_ByChars(t *testing.T) {
	// 25 chars budget. Each doc is 10 chars, so 2 fit per batch.
	b, _ := NewTokenBudget(25)
	b = b.WithMaxBatchSize(100)

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

func TestTokenBudget_Truncate_MultibyteSafe(t *testing.T) {
	// "hello 🌍🌎🌏" is 10 runes but 22 bytes (each emoji is 4 bytes).
	// Truncating to 7 chars must produce "hello 🌍", not corrupt UTF-8.
	b, _ := NewTokenBudget(7)
	result := b.Truncate("hello 🌍🌎🌏")
	require.Equal(t, "hello 🌍", result)
	require.True(t, utf8.ValidString(result))
}

func TestTokenBudget_Truncate_CJK(t *testing.T) {
	// Each CJK character is 3 bytes. A budget of 3 chars must keep 3 runes.
	b, _ := NewTokenBudget(3)
	result := b.Truncate("日本語テスト")
	require.Equal(t, "日本語", result)
	require.True(t, utf8.ValidString(result))
}

func TestTokenBudget_Truncate_MultibyteFitsExact(t *testing.T) {
	// All characters fit — nothing should be truncated.
	b, _ := NewTokenBudget(10)
	result := b.Truncate("hello 🌍🌎🌏")
	require.Equal(t, "hello 🌍🌎🌏", result)
}

func TestTokenBudget_Batches_MultibyteCharCounting(t *testing.T) {
	// Budget 10 chars. Each doc is "🌍🌎🌏🌏🌏" = 5 runes (20 bytes).
	// Two docs = 10 runes = exactly the budget, so both fit in one batch.
	b, _ := NewTokenBudget(10)
	b = b.WithMaxBatchSize(100)

	docs := []Document{
		NewDocument("a", "🌍🌎🌏🌏🌏"),
		NewDocument("b", "🌍🌎🌏🌏🌏"),
	}

	batches := b.Batches(docs)
	require.Len(t, batches, 1, "two 5-rune docs should fit in a 10-char budget")
}

func TestTokenBudget_Batches_SingleDoc(t *testing.T) {
	b := DefaultTokenBudget()
	docs := []Document{NewDocument("id", "hello")}
	batches := b.Batches(docs)
	require.Len(t, batches, 1)
	require.Len(t, batches[0], 1)
}

func TestTokenBudget_WithMaxBatchSize_LimitsBatchCount(t *testing.T) {
	// 100 char budget, 5-char docs. Without max batch size limit, all 6 docs
	// would fit in one batch (30 chars < 100). With max batch size of 2,
	// we should get 3 batches of 2 docs each.
	b, err := NewTokenBudget(100)
	require.NoError(t, err)
	b = b.WithMaxBatchSize(2)

	docs := make([]Document, 6)
	for i := range docs {
		docs[i] = NewDocument("id", "hello")
	}

	batches := b.Batches(docs)
	require.Len(t, batches, 3)
	for _, batch := range batches {
		require.Len(t, batch, 2)
	}
}

func TestTokenBudget_WithMaxBatchSize_DefaultIsOne(t *testing.T) {
	// Default max batch size is 1, so each doc gets its own batch.
	b, err := NewTokenBudget(100)
	require.NoError(t, err)

	docs := make([]Document, 3)
	for i := range docs {
		docs[i] = NewDocument("id", "hello")
	}

	batches := b.Batches(docs)
	require.Len(t, batches, 3)
	for _, batch := range batches {
		require.Len(t, batch, 1)
	}
}

func TestTokenBudget_WithMaxBatchSize_TokenBudgetStillApplied(t *testing.T) {
	// Even with a generous max batch size, the token budget still constrains.
	// 15 char budget, 10-char docs. Only 1 fits per batch due to chars,
	// even though max batch size allows 10.
	b, err := NewTokenBudget(15)
	require.NoError(t, err)
	b = b.WithMaxBatchSize(10)

	docs := make([]Document, 3)
	for i := range docs {
		docs[i] = NewDocument("id", strings.Repeat("a", 10))
	}

	batches := b.Batches(docs)
	require.Len(t, batches, 3)
	for _, batch := range batches {
		require.Len(t, batch, 1)
	}
}

func TestTokenBudget_WithMaxBatchSize_UnevenSplit(t *testing.T) {
	// 7 docs with max batch size 3 → 3 + 3 + 1
	b, err := NewTokenBudget(1000)
	require.NoError(t, err)
	b = b.WithMaxBatchSize(3)

	docs := make([]Document, 7)
	for i := range docs {
		docs[i] = NewDocument("id", "hi")
	}

	batches := b.Batches(docs)
	require.Len(t, batches, 3)
	require.Len(t, batches[0], 3)
	require.Len(t, batches[1], 3)
	require.Len(t, batches[2], 1)
}
