package indexing

import (
	"context"
	"strconv"

	"github.com/helixml/kodit/domain/enrichment"
)

// existingIDsLookup reports which of the given string IDs already have
// entries in the underlying index.
type existingIDsLookup func(ctx context.Context, ids []string) (map[string]struct{}, error)

// filterNewEnrichments returns the subset of enrichments whose IDs are not
// yet present in the index, according to the lookup.
func filterNewEnrichments(ctx context.Context, lookup existingIDsLookup, enrichments []enrichment.Enrichment) ([]enrichment.Enrichment, error) {
	ids := make([]string, len(enrichments))
	for i, e := range enrichments {
		ids[i] = strconv.FormatInt(e.ID(), 10)
	}

	existing, err := lookup(ctx, ids)
	if err != nil {
		return nil, err
	}

	result := make([]enrichment.Enrichment, 0, len(enrichments))
	for i, e := range enrichments {
		if _, ok := existing[ids[i]]; !ok {
			result = append(result, e)
		}
	}
	return result, nil
}
