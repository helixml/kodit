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
func NewTextChunks(content string, params ChunkParams) (TextChunks, error) {
	if params.Overlap >= params.Size {
		return TextChunks{}, fmt.Errorf("overlap (%d) must be less than size (%d)", params.Overlap, params.Size)
	}

	if content == "" {
		return TextChunks{}, nil
	}

	step := params.Size - params.Overlap
	var chunks []Chunk

	for offset := 0; offset < len(content); offset += step {
		end := min(offset+params.Size, len(content))

		text := content[offset:end]
		if len(text) < params.MinSize {
			break
		}

		// Skip chunks fully covered by the previous chunk's overlap.
		if offset > 0 && len(text) <= params.Overlap {
			break
		}

		chunks = append(chunks, Chunk{content: text, offset: offset})
	}

	return TextChunks{chunks: chunks}, nil
}

// All returns all chunks.
func (t TextChunks) All() []Chunk { return t.chunks }
