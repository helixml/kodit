package indexing

import (
	"context"
	"fmt"
	"strconv"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
)

// shareCommitEnrichments records repoID as an owner of enrichments another
// repository already produced for the same commit, so searches scoped to
// repoID find them. Search scoping reads only these repository associations.
//
// Only git commits are shared: two repositories with the same git commit SHA
// hold the same content. A directory snapshot's SHA is a hash kodit computes
// (salted with the repository ID), so its enrichments are never shared.
func shareCommitEnrichments(
	ctx context.Context,
	commits repository.CommitStore,
	associations enrichment.AssociationStore,
	existing []enrichment.Enrichment,
	commitSHA string,
	repoID int64,
) error {
	recorded, err := commits.Find(ctx, repository.WithSHA(commitSHA), repository.WithLimit(1))
	if err != nil {
		return fmt.Errorf("find commit: %w", err)
	}
	if len(recorded) == 0 || recorded[0].IsDirectorySnapshot() {
		return nil
	}

	ids := make([]int64, len(existing))
	for i, e := range existing {
		ids[i] = e.ID()
	}
	repoKey := strconv.FormatInt(repoID, 10)
	owned, err := associations.Find(ctx,
		enrichment.WithEnrichmentIDIn(ids),
		enrichment.WithEntityType(enrichment.EntityTypeRepository),
		enrichment.WithEntityID(repoKey),
	)
	if err != nil {
		return fmt.Errorf("find repository associations: %w", err)
	}
	have := make(map[int64]bool, len(owned))
	for _, a := range owned {
		have[a.EnrichmentID()] = true
	}
	for _, id := range ids {
		if have[id] {
			continue
		}
		if _, err := associations.Save(ctx, enrichment.RepositoryAssociation(id, repoKey)); err != nil {
			return fmt.Errorf("save repository association: %w", err)
		}
	}
	return nil
}
