// Package chunking provides fixed-size text chunking with overlap for RAG indexing.
package chunking

import (
	"fmt"
	"strings"
)

// ChunkParams configures the chunking algorithm.
type ChunkParams struct {
	Size    int
	Overlap int
	MinSize int
}

// DefaultChunkParams returns sensible defaults for code chunking.
func DefaultChunkParams() ChunkParams {
	return ChunkParams{
		Size:    1500,
		Overlap: 200,
		MinSize: 50,
	}
}

// Chunk represents a single text chunk with its byte offset and line range in the original content.
type Chunk struct {
	content   string
	offset    int
	startLine int
	endLine   int
}

// Content returns the chunk text.
func (c Chunk) Content() string { return c.content }

// Offset returns the byte offset of this chunk in the original content.
func (c Chunk) Offset() int { return c.offset }

// StartLine returns the 1-based line number where this chunk begins in the original content.
func (c Chunk) StartLine() int { return c.startLine }

// EndLine returns the 1-based line number where this chunk ends in the original content.
func (c Chunk) EndLine() int { return c.endLine }

// TextChunks holds the result of splitting content into fixed-size chunks.
type TextChunks struct {
	chunks []Chunk
}

// NewTextChunks splits content into fixed-size chunks with the given parameters.
// Size, Overlap, and MinSize are measured in runes (Unicode code points), while
// the returned Chunk.Offset is a byte offset into the original string.
//
// The algorithm uses three tiers:
//   - Tier 1: accumulate whole lines until the next line would exceed Size
//   - Tier 2: for lines exceeding Size, split on whitespace boundaries
//   - Tier 3: for tokens exceeding Size, split on rune boundaries
func NewTextChunks(content string, params ChunkParams) (TextChunks, error) {
	if params.Overlap >= params.Size {
		return TextChunks{}, fmt.Errorf("overlap (%d) must be less than size (%d)", params.Overlap, params.Size)
	}

	if content == "" {
		return TextChunks{}, nil
	}

	lines := splitLines(content)
	var chunks []Chunk
	var acc []string
	accRunes := 0
	byteOffset := 0

	for _, line := range lines {
		lineRunes := len([]rune(line))

		if lineRunes > params.Size {
			// Flush accumulator before handling the long line.
			if accRunes > 0 {
				text := strings.Join(acc, "")
				if len([]rune(text)) >= params.MinSize {
					chunks = append(chunks, Chunk{content: text, offset: byteOffset})
				}
				byteOffset += len(text)
				acc = nil
				accRunes = 0
			}
			// Tier 2/3: split long line into sub-chunks.
			subs := splitLongLine(line, params.Size, params.Overlap)
			for _, sub := range subs {
				if len([]rune(sub.content)) >= params.MinSize {
					chunks = append(chunks, Chunk{content: sub.content, offset: byteOffset + sub.offset})
				}
			}
			byteOffset += len(line)
			continue
		}

		if accRunes+lineRunes > params.Size && accRunes > 0 {
			text := strings.Join(acc, "")
			if len([]rune(text)) >= params.MinSize {
				chunks = append(chunks, Chunk{content: text, offset: byteOffset})
			}
			byteOffset += len(text)

			// Carry overlap lines from the end of the emitted chunk.
			acc, accRunes = overlapLines(acc, params.Overlap)
			byteOffset -= byteLen(acc)
		}

		acc = append(acc, line)
		accRunes += lineRunes
	}

	// Flush remaining accumulator.
	if accRunes > 0 {
		text := strings.Join(acc, "")
		if len([]rune(text)) >= params.MinSize {
			chunks = append(chunks, Chunk{content: text, offset: byteOffset})
		}
	}

	assignLineNumbers(content, chunks)
	return TextChunks{chunks: chunks}, nil
}

// assignLineNumbers computes 1-based start and end line numbers for each chunk
// using the chunk's byte offset into the original content. It builds a newline
// position index so that lookups work regardless of chunk ordering.
func assignLineNumbers(content string, chunks []Chunk) {
	// Build index: newlinePositions[i] is the byte offset of the i-th '\n'.
	var positions []int
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			positions = append(positions, i)
		}
	}

	for i := range chunks {
		chunks[i].startLine = lineAt(positions, chunks[i].offset)

		newlines := strings.Count(chunks[i].content, "\n")
		end := chunks[i].startLine + newlines
		if len(chunks[i].content) > 0 && chunks[i].content[len(chunks[i].content)-1] == '\n' {
			end--
		}
		chunks[i].endLine = end
	}
}

// lineAt returns the 1-based line number for the given byte offset using
// binary search over newline positions.
func lineAt(positions []int, offset int) int {
	lo, hi := 0, len(positions)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if positions[mid] < offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo + 1
}

// splitLines splits content into lines, preserving the trailing \n on each line.
// The last segment is included even if it doesn't end with \n.
func splitLines(content string) []string {
	var lines []string
	for len(content) > 0 {
		idx := strings.IndexByte(content, '\n')
		if idx < 0 {
			lines = append(lines, content)
			break
		}
		lines = append(lines, content[:idx+1])
		content = content[idx+1:]
	}
	return lines
}

// overlapLines walks backward through lines and returns the trailing lines
// whose total rune count fits within the overlap budget.
func overlapLines(lines []string, overlap int) ([]string, int) {
	if overlap == 0 {
		return nil, 0
	}
	total := 0
	start := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		r := len([]rune(lines[i]))
		if total+r > overlap {
			break
		}
		total += r
		start = i
	}
	if start == len(lines) {
		return nil, 0
	}
	carried := make([]string, len(lines)-start)
	copy(carried, lines[start:])
	return carried, total
}

// byteLen returns the total byte length of the given strings.
func byteLen(lines []string) int {
	n := 0
	for _, l := range lines {
		n += len(l)
	}
	return n
}

// subChunk is a chunk-within-a-line with a byte offset relative to the line start.
type subChunk struct {
	content string
	offset  int
}

// splitLongLine splits a line that exceeds Size using whitespace boundaries (Tier 2),
// falling back to rune boundaries (Tier 3) for tokens exceeding Size.
func splitLongLine(line string, size int, overlap int) []subChunk {
	tokens := splitWhitespace(line)
	if len(tokens) <= 1 {
		return splitRunes(line, size, overlap)
	}
	return splitTokens(tokens, size, overlap)
}

// splitWhitespace splits a string into tokens at whitespace boundaries,
// keeping the whitespace attached to the preceding token.
func splitWhitespace(s string) []string {
	runes := []rune(s)
	var tokens []string
	start := 0
	for i := 0; i < len(runes); i++ {
		if runes[i] == ' ' || runes[i] == '\t' {
			continue
		}
		if i > 0 && (runes[i-1] == ' ' || runes[i-1] == '\t') && i > start {
			tokens = append(tokens, string(runes[start:i]))
			start = i
		}
	}
	if start < len(runes) {
		tokens = append(tokens, string(runes[start:]))
	}
	return tokens
}

// splitTokens accumulates whitespace tokens into chunks of at most size runes (Tier 2).
// Tokens exceeding size are split via splitRunes (Tier 3).
func splitTokens(tokens []string, size int, overlap int) []subChunk {
	var result []subChunk
	var acc []string
	accRunes := 0
	byteOff := 0

	for _, tok := range tokens {
		tokRunes := len([]rune(tok))

		if tokRunes > size {
			// Flush accumulator, then Tier 3 split the oversized token.
			if accRunes > 0 {
				text := strings.Join(acc, "")
				result = append(result, subChunk{content: text, offset: byteOff})
				byteOff += len(text)
				acc = nil
				accRunes = 0
			}
			subs := splitRunes(tok, size, overlap)
			for _, sub := range subs {
				result = append(result, subChunk{content: sub.content, offset: byteOff + sub.offset})
			}
			byteOff += len(tok)
			continue
		}

		if accRunes+tokRunes > size && accRunes > 0 {
			text := strings.Join(acc, "")
			result = append(result, subChunk{content: text, offset: byteOff})
			byteOff += len(text)
			acc = nil
			accRunes = 0
		}

		acc = append(acc, tok)
		accRunes += tokRunes
	}

	if accRunes > 0 {
		text := strings.Join(acc, "")
		result = append(result, subChunk{content: text, offset: byteOff})
	}

	return result
}

// splitRunes splits content into chunks of at most size runes with overlap (Tier 3).
func splitRunes(content string, size int, overlap int) []subChunk {
	runes := []rune(content)
	step := size - overlap
	var result []subChunk
	for i := 0; i < len(runes); i += step {
		end := min(i+size, len(runes))
		slice := runes[i:end]
		if i > 0 && len(slice) <= overlap {
			break
		}
		byteOff := len(string(runes[:i]))
		result = append(result, subChunk{content: string(slice), offset: byteOff})
	}
	return result
}

// All returns all chunks.
func (t TextChunks) All() []Chunk { return t.chunks }
