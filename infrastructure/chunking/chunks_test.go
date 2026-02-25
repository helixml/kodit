package chunking

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextChunks_BasicFixedSize(t *testing.T) {
	content := strings.Repeat("A", 300)
	params := ChunkParams{Size: 100, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 3)
	for _, c := range result {
		assert.Len(t, c.Content(), 100)
	}
}

func TestTextChunks_Overlap(t *testing.T) {
	content := "AAAAABBBBBCCCCC"
	params := ChunkParams{Size: 10, Overlap: 5, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 2)
	assert.Equal(t, "AAAAABBBBB", result[0].Content())
	assert.Equal(t, "BBBBBCCCCC", result[1].Content())
}

func TestTextChunks_MinSizeFiltering(t *testing.T) {
	content := "hello"
	params := ChunkParams{Size: 100, Overlap: 0, MinSize: 10}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	assert.Empty(t, chunks.All())
}

func TestTextChunks_EmptyContent(t *testing.T) {
	params := ChunkParams{Size: 100, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks("", params)
	require.NoError(t, err)

	assert.Empty(t, chunks.All())
}

func TestTextChunks_OverlapMustBeLessThanSize(t *testing.T) {
	params := ChunkParams{Size: 10, Overlap: 10, MinSize: 1}

	_, err := NewTextChunks("some content", params)
	require.Error(t, err)
}

func TestDefaultChunkParams(t *testing.T) {
	params := DefaultChunkParams()

	assert.Equal(t, 1500, params.Size)
	assert.Equal(t, 200, params.Overlap)
	assert.Equal(t, 50, params.MinSize)
}

func TestTextChunks_ByteOffsets(t *testing.T) {
	content := strings.Repeat("X", 200)
	params := ChunkParams{Size: 100, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 2)
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, 100, result[1].Offset())
}

func TestTextChunks_OverlapByteOffsets(t *testing.T) {
	content := strings.Repeat("Z", 25)
	params := ChunkParams{Size: 10, Overlap: 5, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 4)
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, 5, result[1].Offset())
	assert.Equal(t, 10, result[2].Offset())
	assert.Equal(t, 15, result[3].Offset())
}

func TestTextChunks_Deterministic(t *testing.T) {
	content := "The quick brown fox 🦊 jumps over the lazy dog 🐕\n" +
		strings.Repeat("── code block ──\n", 20)
	params := ChunkParams{Size: 50, Overlap: 10, MinSize: 5}

	first, err := NewTextChunks(content, params)
	require.NoError(t, err)

	for range 100 {
		got, err := NewTextChunks(content, params)
		require.NoError(t, err)

		require.Len(t, got.All(), len(first.All()), "chunk count changed between runs")
		for i, chunk := range got.All() {
			assert.Equal(t, first.All()[i].Content(), chunk.Content(), "content differs at chunk %d", i)
			assert.Equal(t, first.All()[i].Offset(), chunk.Offset(), "offset differs at chunk %d", i)
		}
	}
}

func TestTextChunks_MultiByteRunes(t *testing.T) {
	// Each "─" is 3 bytes (0xe2 0x94 0x80). Chunking by runes must never
	// split a multi-byte character, which would produce invalid UTF-8.
	content := strings.Repeat("─", 10) // 10 runes, 30 bytes
	params := ChunkParams{Size: 4, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 3)
	assert.Equal(t, "────", result[0].Content())
	assert.Equal(t, "────", result[1].Content())
	assert.Equal(t, "──", result[2].Content())

	// Byte offsets must land on character boundaries.
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, 12, result[1].Offset()) // 4 runes * 3 bytes
	assert.Equal(t, 24, result[2].Offset()) // 8 runes * 3 bytes
}
