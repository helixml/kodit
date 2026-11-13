package indexing

import (
	"context"
	"fmt"
	"strconv"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/git"
)

// CommitReader reads a commit from a working copy's git history.
type CommitReader interface {
	CommitDetails(ctx context.Context, localPath string, commitSHA string) (git.CommitInfo, error)
}

// shareCommitEnrichments records repo as an owner of enrichments another
// repository already produced for commitSHA, so searches scoped to repo find
// them. Search scoping reads only these repository associations.
//
// Content is shared only for a commit the repository holds: its own git
// history must contain commitSHA. A SHA named in a task payload, a rescan or
// a tracking config is not proof. A directory snapshot is never shared,
// because a plain directory has no git history (ErrNotGitRepository). Any
// error reading the commit means the repository does not hold it.
func shareCommitEnrichments(
	ctx context.Context,
	commits CommitReader,
	associations enrichment.AssociationStore,
	repo repository.Repository,
	existing []enrichment.Enrichment,
	commitSHA string,
) error {
	if !repo.HasWorkingCopy() {
		return nil
	}
	if _, err := commits.CommitDetails(ctx, repo.WorkingCopy().Path(), commitSHA); err != nil {
		return nil
	}

	ids := make([]int64, len(existing))
	for i, e := range existing {
		ids[i] = e.ID()
	}
	repoKey := strconv.FormatInt(repo.ID(), 10)
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
