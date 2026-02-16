package v1

import (
	"context"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/domain/repository"
)

// sourceFileMap returns source files grouped by enrichment ID string.
func sourceFileMap(ctx context.Context, client *kodit.Client, enrichmentIDs []int64) (map[string][]repository.File, error) {
	fileIDsByEnrichment, err := client.Enrichments.SourceFiles(ctx, enrichmentIDs)
	if err != nil {
		return nil, err
	}

	var allFileIDs []int64
	for _, ids := range fileIDsByEnrichment {
		allFileIDs = append(allFileIDs, ids...)
	}

	if len(allFileIDs) == 0 {
		return map[string][]repository.File{}, nil
	}

	files, err := client.Files.Find(ctx, repository.WithIDIn(allFileIDs))
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]repository.File, len(files))
	for _, f := range files {
		byID[f.ID()] = f
	}

	result := make(map[string][]repository.File, len(fileIDsByEnrichment))
	for enrichmentID, fileIDs := range fileIDsByEnrichment {
		for _, fid := range fileIDs {
			if f, ok := byID[fid]; ok {
				result[enrichmentID] = append(result[enrichmentID], f)
			}
		}
	}

	return result, nil
}
