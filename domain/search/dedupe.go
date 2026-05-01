package search

import "context"

// ExistingSnippetIDs returns the subset of ids whose snippet IDs already
// have entries in the store. The lookup is split into chunks of
// MaxSnippetIDsPerFind so the IN-clause bind parameters stay within the
// PostgreSQL 65535 limit, and the matches across chunks are unioned.
//
// Works for any search.Store (BM25 or embedding) — both expose Find
// returning Result, which carries the snippet ID.
func ExistingSnippetIDs(ctx context.Context, store Store, ids []string) (map[string]struct{}, error) {
	existing := make(map[string]struct{}, len(ids))
	for start := 0; start < len(ids); start += MaxSnippetIDsPerFind {
		end := min(start+MaxSnippetIDsPerFind, len(ids))
		found, err := store.Find(ctx, WithSnippetIDs(ids[start:end]))
		if err != nil {
			return nil, err
		}
		for _, r := range found {
			existing[r.SnippetID()] = struct{}{}
		}
	}
	return existing, nil
}
