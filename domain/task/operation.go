package task

import "strings"

// Operation represents the type of task operation.
type Operation string

// Operation values for the task queue system.
const (
	OperationRoot                                    Operation = "kodit.root"
	OperationCreateIndex                             Operation = "kodit.index.create"
	OperationRunIndex                                Operation = "kodit.index.run"
	OperationRefreshWorkingCopy                      Operation = "kodit.index.run.refresh_working_copy"
	OperationDeleteOldSnippets                       Operation = "kodit.index.run.delete_old_snippets"
	OperationExtractSnippets                         Operation = "kodit.index.run.extract_snippets"
	OperationCreateBM25Index                         Operation = "kodit.index.run.create_bm25_index"
	OperationCreateCodeEmbeddings                    Operation = "kodit.index.run.create_code_embeddings"
	OperationEnrichSnippets                          Operation = "kodit.index.run.enrich_snippets"
	OperationCreateTextEmbeddings                    Operation = "kodit.index.run.create_text_embeddings"
	OperationUpdateIndexTimestamp                    Operation = "kodit.index.run.update_index_timestamp"
	OperationClearFileProcessingStatuses             Operation = "kodit.index.run.clear_file_processing_statuses"
	OperationRepository                              Operation = "kodit.repository"
	OperationCreateRepository                        Operation = "kodit.repository.create"
	OperationDeleteRepository                        Operation = "kodit.repository.delete"
	OperationCloneRepository                         Operation = "kodit.repository.clone"
	OperationSyncRepository                          Operation = "kodit.repository.sync"
	OperationCommit                                  Operation = "kodit.commit"
	OperationExtractSnippetsForCommit                Operation = "kodit.commit.extract_snippets"
	OperationCreateBM25IndexForCommit                Operation = "kodit.commit.create_bm25_index"
	OperationCreateCodeEmbeddingsForCommit           Operation = "kodit.commit.create_code_embeddings"
	OperationCreateSummaryEnrichmentForCommit        Operation = "kodit.commit.create_summary_enrichment"
	OperationCreateSummaryEmbeddingsForCommit        Operation = "kodit.commit.create_summary_embeddings"
	OperationCreateArchitectureEnrichmentForCommit   Operation = "kodit.commit.create_architecture_enrichment"
	OperationCreatePublicAPIDocsForCommit            Operation = "kodit.commit.create_public_api_docs"
	OperationCreateCommitDescriptionForCommit        Operation = "kodit.commit.create_commit_description"
	OperationCreateDatabaseSchemaForCommit           Operation = "kodit.commit.create_database_schema"
	OperationCreateCookbookForCommit                 Operation = "kodit.commit.create_cookbook"
	OperationExtractExamplesForCommit                Operation = "kodit.commit.extract_examples"
	OperationCreateExampleSummaryForCommit           Operation = "kodit.commit.create_example_summary"
	OperationCreateExampleCodeEmbeddingsForCommit    Operation = "kodit.commit.create_example_code_embeddings"
	OperationCreateExampleSummaryEmbeddingsForCommit Operation = "kodit.commit.create_example_summary_embeddings"
	OperationGenerateWikiForCommit                   Operation = "kodit.commit.generate_wiki"
	OperationScanCommit                              Operation = "kodit.commit.scan"
	OperationRescanCommit                            Operation = "kodit.commit.rescan"
)

// String returns the string representation of the operation.
func (o Operation) String() string {
	return string(o)
}

// IsRepositoryOperation returns true if this is a repository-level operation.
func (o Operation) IsRepositoryOperation() bool {
	return strings.HasPrefix(string(o), "kodit.repository.")
}

// IsCommitOperation returns true if this is a commit-level operation.
func (o Operation) IsCommitOperation() bool {
	return strings.HasPrefix(string(o), "kodit.commit.")
}

// PrescribedOperations provides predefined operation sequences for common workflows.
type PrescribedOperations struct {
	examples    bool
	enrichments bool
}

// DefaultPrescribedOperations returns the standard operation set.
// LLM enrichments are included only when a text provider is available,
// preserving backward-compatible behaviour.
func DefaultPrescribedOperations(hasTextProvider bool) PrescribedOperations {
	return PrescribedOperations{enrichments: hasTextProvider}
}

// RAGOnlyPrescribedOperations returns the operation set for RAG use cases:
// snippet extraction, BM25 indexing, and code embeddings.
// All enrichments (including public API docs) are excluded regardless of
// provider configuration.
func RAGOnlyPrescribedOperations() PrescribedOperations {
	return PrescribedOperations{enrichments: false}
}

// FullPrescribedOperations returns the complete operation set including all
// LLM enrichments. The caller must ensure a text provider is configured.
func FullPrescribedOperations() PrescribedOperations {
	return PrescribedOperations{enrichments: true}
}

// RequiresTextProvider reports whether this operation set needs a text
// generation provider. Callers should fail fast when this returns true and no
// provider is configured.
func (p PrescribedOperations) RequiresTextProvider() bool {
	return p.enrichments
}

// All returns every operation that appears in any prescribed workflow.
// Used at startup to validate that all required handlers are registered.
func (p PrescribedOperations) All() []Operation {
	seen := make(map[Operation]struct{})
	var all []Operation

	for _, ops := range [][]Operation{
		p.CreateNewRepository(),
		p.SyncRepository(),
		p.ScanAndIndexCommit(),
		p.IndexCommit(),
		p.RescanCommit(),
	} {
		for _, op := range ops {
			if _, ok := seen[op]; !ok {
				seen[op] = struct{}{}
				all = append(all, op)
			}
		}
	}
	return all
}

// CreateNewRepository returns the operations needed to create a new repository.
func (p PrescribedOperations) CreateNewRepository() []Operation {
	return []Operation{
		OperationCloneRepository,
	}
}

// SyncRepository returns the operations needed to sync a repository.
func (p PrescribedOperations) SyncRepository() []Operation {
	return []Operation{
		OperationCloneRepository,
		OperationSyncRepository,
	}
}

// ScanAndIndexCommit returns the full operation sequence for scanning and indexing a commit.
func (p PrescribedOperations) ScanAndIndexCommit() []Operation {
	ops := []Operation{
		OperationScanCommit,
		OperationExtractSnippetsForCommit,
	}
	if p.examples {
		ops = append(ops, OperationExtractExamplesForCommit)
	}
	ops = append(ops,
		OperationCreateBM25IndexForCommit,
		OperationCreateCodeEmbeddingsForCommit,
	)
	if p.examples {
		ops = append(ops, OperationCreateExampleCodeEmbeddingsForCommit)
	}
	if p.enrichments && p.examples {
		ops = append(ops, OperationCreateSummaryEnrichmentForCommit)
	}
	if p.enrichments && p.examples {
		ops = append(ops, OperationCreateExampleSummaryForCommit)
	}
	if p.enrichments {
		ops = append(ops, OperationCreateSummaryEmbeddingsForCommit)
	}
	if p.enrichments && p.examples {
		ops = append(ops, OperationCreateExampleSummaryEmbeddingsForCommit)
	}
	if p.enrichments {
		ops = append(ops,
			OperationCreatePublicAPIDocsForCommit,
			OperationCreateArchitectureEnrichmentForCommit,
			OperationCreateCommitDescriptionForCommit,
			OperationCreateDatabaseSchemaForCommit,
			OperationCreateCookbookForCommit,
			OperationGenerateWikiForCommit,
		)
	}
	return ops
}

// IndexCommit returns the operation sequence for indexing an already-scanned commit.
func (p PrescribedOperations) IndexCommit() []Operation {
	ops := []Operation{
		OperationExtractSnippetsForCommit,
		OperationCreateBM25IndexForCommit,
		OperationCreateCodeEmbeddingsForCommit,
	}
	if p.enrichments && p.examples {
		ops = append(ops, OperationCreateSummaryEnrichmentForCommit)
	}
	if p.enrichments {
		ops = append(ops, OperationCreateSummaryEmbeddingsForCommit)
	}
	if p.enrichments {
		ops = append(ops,
			OperationCreatePublicAPIDocsForCommit,
			OperationCreateArchitectureEnrichmentForCommit,
			OperationCreateCommitDescriptionForCommit,
			OperationCreateDatabaseSchemaForCommit,
			OperationCreateCookbookForCommit,
			OperationGenerateWikiForCommit,
		)
	}
	return ops
}

// RescanCommit returns the operation sequence for rescanning a commit (full reindex).
func (p PrescribedOperations) RescanCommit() []Operation {
	ops := []Operation{
		OperationRescanCommit,
		OperationExtractSnippetsForCommit,
	}
	if p.examples {
		ops = append(ops, OperationExtractExamplesForCommit)
	}
	ops = append(ops,
		OperationCreateBM25IndexForCommit,
		OperationCreateCodeEmbeddingsForCommit,
	)
	if p.examples {
		ops = append(ops, OperationCreateExampleCodeEmbeddingsForCommit)
	}
	if p.enrichments && p.examples {
		ops = append(ops, OperationCreateSummaryEnrichmentForCommit)
	}
	if p.enrichments && p.examples {
		ops = append(ops, OperationCreateExampleSummaryForCommit)
	}
	if p.enrichments {
		ops = append(ops, OperationCreateSummaryEmbeddingsForCommit)
	}
	if p.enrichments && p.examples {
		ops = append(ops, OperationCreateExampleSummaryEmbeddingsForCommit)
	}
	if p.enrichments {
		ops = append(ops,
			OperationCreatePublicAPIDocsForCommit,
			OperationCreateArchitectureEnrichmentForCommit,
			OperationCreateCommitDescriptionForCommit,
			OperationCreateDatabaseSchemaForCommit,
			OperationCreateCookbookForCommit,
			OperationGenerateWikiForCommit,
		)
	}
	return ops
}
