package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAll(t *testing.T) {
	set := func(ops []Operation) map[Operation]struct{} {
		m := make(map[Operation]struct{}, len(ops))
		for _, op := range ops {
			m[op] = struct{}{}
		}
		return m
	}

	t.Run("always present", func(t *testing.T) {
		ops := set(RAGOnlyPrescribedOperations().All())
		always := []Operation{
			OperationCreateRepository,
			OperationCloneRepository,
			OperationSyncRepository,
			OperationDeleteRepository,
			OperationRescanCommit,
			OperationScanCommit,
			OperationExtractSnippetsForCommit,
			OperationCreateBM25IndexForCommit,
			OperationCreateCodeEmbeddingsForCommit,
			OperationExtractPageImagesForCommit,
			OperationCreatePageImageEmbeddingsForCommit,
		}
		for _, op := range always {
			assert.Contains(t, ops, op, "expected %s to be present", op)
		}
	})

	t.Run("enrichments included when enabled", func(t *testing.T) {
		ops := set(FullPrescribedOperations().All())
		enrichmentOps := []Operation{
			OperationCreateSummaryEmbeddingsForCommit,
			OperationCreatePublicAPIDocsForCommit,
			OperationCreateArchitectureEnrichmentForCommit,
			OperationCreateCommitDescriptionForCommit,
			OperationCreateDatabaseSchemaForCommit,
			OperationCreateCookbookForCommit,
			OperationGenerateWikiForCommit,
		}
		for _, op := range enrichmentOps {
			assert.Contains(t, ops, op, "expected %s to be present", op)
		}
	})

	t.Run("enrichments excluded when disabled", func(t *testing.T) {
		ops := set(RAGOnlyPrescribedOperations().All())
		enrichmentOps := []Operation{
			OperationCreateSummaryEmbeddingsForCommit,
			OperationCreatePublicAPIDocsForCommit,
			OperationCreateArchitectureEnrichmentForCommit,
			OperationCreateCommitDescriptionForCommit,
			OperationCreateDatabaseSchemaForCommit,
			OperationCreateCookbookForCommit,
			OperationGenerateWikiForCommit,
		}
		for _, op := range enrichmentOps {
			assert.NotContains(t, ops, op, "expected %s to be absent", op)
		}
	})
}
