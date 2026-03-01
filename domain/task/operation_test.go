package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func operationSet(ops []Operation) map[Operation]struct{} {
	s := make(map[Operation]struct{}, len(ops))
	for _, op := range ops {
		s[op] = struct{}{}
	}
	return s
}

func contains(ops []Operation, target Operation) bool {
	_, ok := operationSet(ops)[target]
	return ok
}

// coreOps are always present regardless of examples/enrichments flags.
var coreOps = []Operation{
	OperationScanCommit,
	OperationExtractSnippetsForCommit,
	OperationCreateBM25IndexForCommit,
	OperationCreateCodeEmbeddingsForCommit,
	OperationCreatePublicAPIDocsForCommit,
}

// exampleOnlyOps require examples=true but not enrichments.
var exampleOnlyOps = []Operation{
	OperationExtractExamplesForCommit,
	OperationCreateExampleCodeEmbeddingsForCommit,
}

// enrichmentOps require enrichments=true (no examples dependency).
var enrichmentOps = []Operation{
	OperationCreateSummaryEmbeddingsForCommit,
	OperationCreateArchitectureEnrichmentForCommit,
	OperationCreateCommitDescriptionForCommit,
	OperationCreateDatabaseSchemaForCommit,
	OperationCreateCookbookForCommit,
	OperationGenerateWikiForCommit,
}

// enrichmentAndExampleOps require both enrichments=true and examples=true.
var enrichmentAndExampleOps = []Operation{
	OperationCreateSummaryEnrichmentForCommit,
	OperationCreateExampleSummaryForCommit,
	OperationCreateExampleSummaryEmbeddingsForCommit,
}

func TestScanAndIndexCommit(t *testing.T) {
	tests := []struct {
		name        string
		examples    bool
		enrichments bool
		wantPresent []Operation
		wantAbsent  []Operation
	}{
		{
			name:        "all enabled",
			examples:    true,
			enrichments: true,
			wantPresent: flatten(coreOps, exampleOnlyOps, enrichmentOps, enrichmentAndExampleOps),
		},
		{
			name:        "enrichments disabled",
			examples:    true,
			enrichments: false,
			wantPresent: flatten(coreOps, exampleOnlyOps),
			wantAbsent:  flatten(enrichmentOps, enrichmentAndExampleOps),
		},
		{
			name:        "examples disabled",
			examples:    false,
			enrichments: true,
			wantPresent: flatten(coreOps, enrichmentOps),
			wantAbsent:  flatten(exampleOnlyOps, enrichmentAndExampleOps),
		},
		{
			name:        "both disabled",
			examples:    false,
			enrichments: false,
			wantPresent: coreOps,
			wantAbsent:  flatten(exampleOnlyOps, enrichmentOps, enrichmentAndExampleOps),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := NewPrescribedOperations(tt.examples, tt.enrichments).ScanAndIndexCommit()
			set := operationSet(ops)
			for _, op := range tt.wantPresent {
				assert.Contains(t, set, op, "expected %s to be present", op)
			}
			for _, op := range tt.wantAbsent {
				assert.NotContains(t, set, op, "expected %s to be absent", op)
			}
		})
	}
}

func TestIndexCommit(t *testing.T) {
	tests := []struct {
		name        string
		examples    bool
		enrichments bool
		wantPresent []Operation
		wantAbsent  []Operation
	}{
		{
			name:        "all enabled",
			examples:    true,
			enrichments: true,
			wantPresent: []Operation{
				OperationExtractSnippetsForCommit,
				OperationCreateBM25IndexForCommit,
				OperationCreateCodeEmbeddingsForCommit,
				OperationCreateSummaryEnrichmentForCommit,
				OperationCreateSummaryEmbeddingsForCommit,
				OperationCreatePublicAPIDocsForCommit,
				OperationCreateArchitectureEnrichmentForCommit,
				OperationCreateCommitDescriptionForCommit,
				OperationCreateDatabaseSchemaForCommit,
				OperationCreateCookbookForCommit,
				OperationGenerateWikiForCommit,
			},
		},
		{
			name:        "enrichments disabled",
			examples:    true,
			enrichments: false,
			wantPresent: []Operation{
				OperationExtractSnippetsForCommit,
				OperationCreateBM25IndexForCommit,
				OperationCreateCodeEmbeddingsForCommit,
				OperationCreatePublicAPIDocsForCommit,
			},
			wantAbsent: flatten(enrichmentOps, enrichmentAndExampleOps),
		},
		{
			name:        "examples disabled",
			examples:    false,
			enrichments: true,
			wantPresent: []Operation{
				OperationExtractSnippetsForCommit,
				OperationCreateBM25IndexForCommit,
				OperationCreateCodeEmbeddingsForCommit,
				OperationCreateSummaryEmbeddingsForCommit,
				OperationCreatePublicAPIDocsForCommit,
				OperationCreateArchitectureEnrichmentForCommit,
				OperationCreateCommitDescriptionForCommit,
				OperationCreateDatabaseSchemaForCommit,
				OperationCreateCookbookForCommit,
				OperationGenerateWikiForCommit,
			},
			wantAbsent: []Operation{OperationCreateSummaryEnrichmentForCommit},
		},
		{
			name:        "both disabled",
			examples:    false,
			enrichments: false,
			wantPresent: []Operation{
				OperationExtractSnippetsForCommit,
				OperationCreateBM25IndexForCommit,
				OperationCreateCodeEmbeddingsForCommit,
				OperationCreatePublicAPIDocsForCommit,
			},
			wantAbsent: flatten(enrichmentOps, enrichmentAndExampleOps),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := NewPrescribedOperations(tt.examples, tt.enrichments).IndexCommit()
			set := operationSet(ops)
			for _, op := range tt.wantPresent {
				assert.Contains(t, set, op, "expected %s to be present", op)
			}
			for _, op := range tt.wantAbsent {
				assert.NotContains(t, set, op, "expected %s to be absent", op)
			}
		})
	}
}

func TestRescanCommit(t *testing.T) {
	tests := []struct {
		name        string
		examples    bool
		enrichments bool
		wantPresent []Operation
		wantAbsent  []Operation
	}{
		{
			name:        "all enabled",
			examples:    true,
			enrichments: true,
			wantPresent: []Operation{
				OperationRescanCommit,
				OperationExtractSnippetsForCommit,
				OperationExtractExamplesForCommit,
				OperationCreateBM25IndexForCommit,
				OperationCreateCodeEmbeddingsForCommit,
				OperationCreateExampleCodeEmbeddingsForCommit,
				OperationCreateSummaryEnrichmentForCommit,
				OperationCreateExampleSummaryForCommit,
				OperationCreateSummaryEmbeddingsForCommit,
				OperationCreateExampleSummaryEmbeddingsForCommit,
				OperationCreatePublicAPIDocsForCommit,
				OperationCreateArchitectureEnrichmentForCommit,
				OperationCreateCommitDescriptionForCommit,
				OperationCreateDatabaseSchemaForCommit,
				OperationCreateCookbookForCommit,
				OperationGenerateWikiForCommit,
			},
		},
		{
			name:        "enrichments disabled",
			examples:    true,
			enrichments: false,
			wantPresent: []Operation{
				OperationRescanCommit,
				OperationExtractSnippetsForCommit,
				OperationExtractExamplesForCommit,
				OperationCreateBM25IndexForCommit,
				OperationCreateCodeEmbeddingsForCommit,
				OperationCreateExampleCodeEmbeddingsForCommit,
				OperationCreatePublicAPIDocsForCommit,
			},
			wantAbsent: flatten(enrichmentOps, enrichmentAndExampleOps),
		},
		{
			name:        "both disabled",
			examples:    false,
			enrichments: false,
			wantPresent: []Operation{
				OperationRescanCommit,
				OperationExtractSnippetsForCommit,
				OperationCreateBM25IndexForCommit,
				OperationCreateCodeEmbeddingsForCommit,
				OperationCreatePublicAPIDocsForCommit,
			},
			wantAbsent: flatten(exampleOnlyOps, enrichmentOps, enrichmentAndExampleOps),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := NewPrescribedOperations(tt.examples, tt.enrichments).RescanCommit()
			set := operationSet(ops)
			for _, op := range tt.wantPresent {
				assert.Contains(t, set, op, "expected %s to be present", op)
			}
			for _, op := range tt.wantAbsent {
				assert.NotContains(t, set, op, "expected %s to be absent", op)
			}
		})
	}
}

func TestPublicAPIDocsAlwaysPresent(t *testing.T) {
	combinations := []struct {
		examples    bool
		enrichments bool
	}{
		{true, true},
		{true, false},
		{false, true},
		{false, false},
	}

	for _, c := range combinations {
		p := NewPrescribedOperations(c.examples, c.enrichments)

		assert.True(t, contains(p.ScanAndIndexCommit(), OperationCreatePublicAPIDocsForCommit),
			"ScanAndIndexCommit(examples=%v, enrichments=%v)", c.examples, c.enrichments)
		assert.True(t, contains(p.IndexCommit(), OperationCreatePublicAPIDocsForCommit),
			"IndexCommit(examples=%v, enrichments=%v)", c.examples, c.enrichments)
		assert.True(t, contains(p.RescanCommit(), OperationCreatePublicAPIDocsForCommit),
			"RescanCommit(examples=%v, enrichments=%v)", c.examples, c.enrichments)
	}
}

func TestAllAggregatesWorkflows(t *testing.T) {
	p := NewPrescribedOperations(true, true)
	all := p.All()
	set := operationSet(all)

	// All should include operations from every workflow
	assert.Contains(t, set, OperationCloneRepository)
	assert.Contains(t, set, OperationSyncRepository)
	assert.Contains(t, set, OperationScanCommit)
	assert.Contains(t, set, OperationRescanCommit)
	assert.Contains(t, set, OperationGenerateWikiForCommit)
}

func flatten(slices ...[]Operation) []Operation {
	var result []Operation
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}
