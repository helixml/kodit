package search

import "context"

// ExistingSnippetIDs returns the subset of ids whose snippet IDs already
// have embeddings in the store. The lookup is split into chunks of
// MaxSnippetIDsPerFind so the IN-clause bind parameters stay within the
// PostgreSQL 65535 limit, and the matches across chunks are unioned.
func ExistingSnippetIDs(ctx context.Context, store EmbeddingStore, ids []string) (map[string]struct{}, error) {
	existing := make(map[string]struct{}, len(ids))
	for start := 0; start < len(ids); start += MaxSnippetIDsPerFind {
		end := min(start+MaxSnippetIDsPerFind, len(ids))
		found, err := store.Find(ctx, WithSnippetIDs(ids[start:end]))
		if err != nil {
			return nil, err
		}
		for _, e := range found {
			existing[e.SnippetID()] = struct{}{}
		}
	}
	return existing, nil
}
