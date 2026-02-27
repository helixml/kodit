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

func TestTextChunks_LineSplit(t *testing.T) {
	content := "hello\nworld\nfoo\n" // 16 runes
	params := ChunkParams{Size: 10, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 2)
	assert.Equal(t, "hello\n", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, "world\nfoo\n", result[1].Content())
	assert.Equal(t, 6, result[1].Offset())
}

func TestTextChunks_LineOverlap(t *testing.T) {
	content := "aaa\nbbb\nccc\nddd\n" // 16 runes, lines of 4 runes each
	params := ChunkParams{Size: 8, Overlap: 4, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 3)
	assert.Equal(t, "aaa\nbbb\n", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, "bbb\nccc\n", result[1].Content())
	assert.Equal(t, 4, result[1].Offset())
	assert.Equal(t, "ccc\nddd\n", result[2].Content())
	assert.Equal(t, 8, result[2].Offset())
}

func TestTextChunks_WhitespaceSplit(t *testing.T) {
	content := "aaaa bbbb cccc" // 14 runes, no newlines
	params := ChunkParams{Size: 7, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	// Whitespace-aware split should not break "bbbb".
	// Rune split would produce "aaaa bb" and "b cccc" — splitting "bbbb".
	require.Len(t, result, 3)
	assert.Equal(t, "aaaa ", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, "bbbb ", result[1].Content())
	assert.Equal(t, 5, result[1].Offset())
	assert.Equal(t, "cccc", result[2].Content())
	assert.Equal(t, 10, result[2].Offset())
}

func TestTextChunks_RealisticCode(t *testing.T) {
	content := "aaa\nbbb\nccc\nddd\neee\nfff\n" // 6 lines of 4 runes each = 24 runes
	params := ChunkParams{Size: 12, Overlap: 4, MinSize: 3}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.NotEmpty(t, result)

	for _, c := range result {
		assert.LessOrEqual(t, len([]rune(c.Content())), 12, "chunk exceeds Size")
		assert.GreaterOrEqual(t, len([]rune(c.Content())), 3, "chunk below MinSize")

		// Offset integrity: content at offset matches chunk.
		assert.Equal(t, c.Content(), content[c.Offset():c.Offset()+len(c.Content())],
			"offset mismatch for chunk %q at offset %d", c.Content(), c.Offset())
	}

	// Overlap: adjacent chunks share some content (all lines fit in overlap budget).
	if len(result) > 1 {
		for i := 1; i < len(result); i++ {
			prev := result[i-1]
			cur := result[i]
			assert.Less(t, cur.Offset(), prev.Offset()+len(prev.Content()),
				"chunks %d and %d should overlap", i-1, i)
		}
	}
}

func TestTextChunks_MinSizeLineChunks(t *testing.T) {
	content := "ab\ncd\nef\n" // 9 runes, 3 lines of 3 runes each
	params := ChunkParams{Size: 3, Overlap: 0, MinSize: 3}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)
	assert.Len(t, chunks.All(), 3, "each 3-rune chunk meets MinSize=3")

	params.MinSize = 4
	chunks, err = NewTextChunks(content, params)
	require.NoError(t, err)
	assert.Empty(t, chunks.All(), "all 3-rune chunks dropped with MinSize=4")
}

func TestTextChunks_NoTrailingNewline(t *testing.T) {
	content := "hello\nworld" // 11 runes, no trailing \n
	params := ChunkParams{Size: 20, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 1)
	assert.Equal(t, "hello\nworld", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
}

func TestTextChunks_MixedTier1And3(t *testing.T) {
	content := "ab\n" + strings.Repeat("x", 20) + "\nyz\n"
	params := ChunkParams{Size: 10, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	// "ab\n" (3) → Tier 1 chunk
	// 21-rune line ("xxxx...x\n") → Tier 3 rune split (no whitespace)
	// "yz\n" (3) → Tier 1 chunk
	assert.Equal(t, "ab\n", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
	last := result[len(result)-1]
	assert.Equal(t, "yz\n", last.Content())
	for _, c := range result {
		assert.LessOrEqual(t, len([]rune(c.Content())), 10, "chunk exceeds Size: %q", c.Content())
	}
}

func TestTextChunks_MixedTier1And2(t *testing.T) {
	content := "short\nthe quick brown fox jumps\nend\n"
	params := ChunkParams{Size: 12, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	// "short\n" (6) → Tier 1 chunk
	// "the quick brown fox jumps\n" (26) → Tier 2 whitespace split
	// "end\n" (4) → Tier 1 chunk
	for _, c := range result {
		assert.LessOrEqual(t, len([]rune(c.Content())), 12, "chunk exceeds Size: %q", c.Content())
	}
	// Verify short lines are kept whole.
	assert.Equal(t, "short\n", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
	// Last chunk should be "end\n" kept whole.
	last := result[len(result)-1]
	assert.Equal(t, "end\n", last.Content())
	// All content is covered.
	total := ""
	covered := make([]bool, len(content))
	for _, c := range result {
		total += c.Content()
		for i := range len(c.Content()) {
			covered[c.Offset()+i] = true
		}
	}
	for i, c := range covered {
		assert.True(t, c, "byte %d not covered", i)
	}
}

func TestTextChunks_RuneFallbackOverlap(t *testing.T) {
	content := "abcdefghijklmno" // 15 runes
	params := ChunkParams{Size: 6, Overlap: 2, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 4)
	assert.Equal(t, "abcdef", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, "efghij", result[1].Content())
	assert.Equal(t, 4, result[1].Offset())
	assert.Equal(t, "ijklmn", result[2].Content())
	assert.Equal(t, 8, result[2].Offset())
	assert.Equal(t, "mno", result[3].Content())
	assert.Equal(t, 12, result[3].Offset())
}

func TestTextChunks_RuneFallback(t *testing.T) {
	content := "abcdefghijklmno" // 15 runes, no whitespace, no newlines
	params := ChunkParams{Size: 6, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 3)
	assert.Equal(t, "abcdef", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, "ghijkl", result[1].Content())
	assert.Equal(t, 6, result[1].Offset())
	assert.Equal(t, "mno", result[2].Content())
	assert.Equal(t, 12, result[2].Offset())
}

func TestTextChunks_WhitespaceSplitMultiByte(t *testing.T) {
	// "── ── ── ──" = 11 runes, 25 bytes (each ─ is 3 bytes)
	content := "── ── ── ──"
	params := ChunkParams{Size: 6, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 2)
	assert.Equal(t, "── ── ", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, "── ──", result[1].Content())
	assert.Equal(t, 14, result[1].Offset()) // "── ── " = 3+3+1+3+3+1 = 14 bytes
}

func TestTextChunks_LineOverlapMultipleShortLines(t *testing.T) {
	content := "a\nb\nc\nd\ne\nf\n" // 12 runes, lines of 2 runes each
	params := ChunkParams{Size: 4, Overlap: 3, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	// Each chunk holds 2 lines (4 runes). Overlap=3 carries 1 line (2 runes).
	// Step is 1 line at a time: ab, bc, cd, de, ef → 5 chunks
	require.Len(t, result, 5)
	assert.Equal(t, "a\nb\n", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, "b\nc\n", result[1].Content())
	assert.Equal(t, 2, result[1].Offset())
	assert.Equal(t, "c\nd\n", result[2].Content())
	assert.Equal(t, 4, result[2].Offset())
	assert.Equal(t, "d\ne\n", result[3].Content())
	assert.Equal(t, 6, result[3].Offset())
	assert.Equal(t, "e\nf\n", result[4].Content())
	assert.Equal(t, 8, result[4].Offset())
}

func TestTextChunks_LineSplitMultiByte(t *testing.T) {
	// Each "──\n" is 3 runes / 7 bytes (each ─ is 3 bytes).
	content := "──\n──\n──\n" // 9 runes, 21 bytes
	params := ChunkParams{Size: 4, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 3)
	assert.Equal(t, "──\n", result[0].Content())
	assert.Equal(t, 0, result[0].Offset())
	assert.Equal(t, "──\n", result[1].Content())
	assert.Equal(t, 7, result[1].Offset())
	assert.Equal(t, "──\n", result[2].Content())
	assert.Equal(t, 14, result[2].Offset())
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

func TestChunk_LineNumbers_SingleLine(t *testing.T) {
	content := "hello world"
	params := ChunkParams{Size: 100, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 1)
	assert.Equal(t, 1, result[0].StartLine())
	assert.Equal(t, 1, result[0].EndLine())
}

func TestChunk_LineNumbers_MultiLine(t *testing.T) {
	content := "line1\nline2\nline3\n"
	params := ChunkParams{Size: 100, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 1)
	assert.Equal(t, 1, result[0].StartLine())
	assert.Equal(t, 3, result[0].EndLine())
}

func TestChunk_LineNumbers_SplitAcrossLines(t *testing.T) {
	// 5 lines of 6 runes each (including newline). Size=12 means 2 lines per chunk.
	content := "aaaaa\nbbbbb\nccccc\nddddd\neeeee\n"
	params := ChunkParams{Size: 12, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 3)

	assert.Equal(t, 1, result[0].StartLine())
	assert.Equal(t, 2, result[0].EndLine())

	assert.Equal(t, 3, result[1].StartLine())
	assert.Equal(t, 4, result[1].EndLine())

	assert.Equal(t, 5, result[2].StartLine())
	assert.Equal(t, 5, result[2].EndLine())
}

func TestChunk_LineNumbers_NoTrailingNewline(t *testing.T) {
	content := "line1\nline2\nline3"
	params := ChunkParams{Size: 100, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 1)
	assert.Equal(t, 1, result[0].StartLine())
	assert.Equal(t, 3, result[0].EndLine())
}

func TestChunk_LineNumbers_LongLineSplit(t *testing.T) {
	// A single long line exceeding Size — all sub-chunks stay on the same line.
	content := strings.Repeat("x", 30)
	params := ChunkParams{Size: 10, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 3)
	for _, c := range result {
		assert.Equal(t, 1, c.StartLine())
		assert.Equal(t, 1, c.EndLine())
	}
}

func TestChunk_LineNumbers_MultiByteRunes(t *testing.T) {
	// Multi-byte runes on separate lines. Byte offsets differ from rune offsets
	// but line numbers must still be correct.
	content := "──\n──\n──\n"
	params := ChunkParams{Size: 4, Overlap: 0, MinSize: 1}

	chunks, err := NewTextChunks(content, params)
	require.NoError(t, err)

	result := chunks.All()
	require.Len(t, result, 3)

	assert.Equal(t, 1, result[0].StartLine())
	assert.Equal(t, 1, result[0].EndLine())

	assert.Equal(t, 2, result[1].StartLine())
	assert.Equal(t, 2, result[1].EndLine())

	assert.Equal(t, 3, result[2].StartLine())
	assert.Equal(t, 3, result[2].EndLine())
}
