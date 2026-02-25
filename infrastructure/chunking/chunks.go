// Package chunking provides fixed-size text chunking with overlap for RAG indexing.
package chunking

import "fmt"

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

// Chunk represents a single text chunk with its byte offset in the original content.
type Chunk struct {
	content string
	offset  int
}

// Content returns the chunk text.
func (c Chunk) Content() string { return c.content }

// Offset returns the byte offset of this chunk in the original content.
func (c Chunk) Offset() int { return c.offset }

// TextChunks holds the result of splitting content into fixed-size chunks.
type TextChunks struct {
	chunks []Chunk
}

// NewTextChunks splits content into fixed-size chunks with the given parameters.
// Size, Overlap, and MinSize are measured in runes (Unicode code points), while
// the returned Chunk.Offset is a byte offset into the original string.
func NewTextChunks(content string, params ChunkParams) (TextChunks, error) {
	if params.Overlap >= params.Size {
		return TextChunks{}, fmt.Errorf("overlap (%d) must be less than size (%d)", params.Overlap, params.Size)
	}

	if content == "" {
		return TextChunks{}, nil
	}

	runes := []rune(content)
	step := params.Size - params.Overlap
	var chunks []Chunk

	for i := 0; i < len(runes); i += step {
		end := min(i+params.Size, len(runes))

		slice := runes[i:end]
		if len(slice) < params.MinSize {
			break
		}

		// Skip chunks fully covered by the previous chunk's overlap.
		if i > 0 && len(slice) <= params.Overlap {
			break
		}

		chunks = append(chunks, Chunk{content: string(slice), offset: len(string(runes[:i]))})
	}

	return TextChunks{chunks: chunks}, nil
}

// All returns all chunks.
func (t TextChunks) All() []Chunk { return t.chunks }
