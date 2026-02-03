// Package queue provides task queue management and orchestration.
package queue

import "strings"

// TaskOperation represents the type of task operation.
type TaskOperation string

// TaskOperation values for the task queue system.
const (
	OperationRoot                            TaskOperation = "kodit.root"
	OperationCreateIndex                     TaskOperation = "kodit.index.create"
	OperationRunIndex                        TaskOperation = "kodit.index.run"
	OperationRefreshWorkingCopy              TaskOperation = "kodit.index.run.refresh_working_copy"
	OperationDeleteOldSnippets               TaskOperation = "kodit.index.run.delete_old_snippets"
	OperationExtractSnippets                 TaskOperation = "kodit.index.run.extract_snippets"
	OperationCreateBM25Index                 TaskOperation = "kodit.index.run.create_bm25_index"
	OperationCreateCodeEmbeddings            TaskOperation = "kodit.index.run.create_code_embeddings"
	OperationEnrichSnippets                  TaskOperation = "kodit.index.run.enrich_snippets"
	OperationCreateTextEmbeddings            TaskOperation = "kodit.index.run.create_text_embeddings"
	OperationUpdateIndexTimestamp            TaskOperation = "kodit.index.run.update_index_timestamp"
	OperationClearFileProcessingStatuses     TaskOperation = "kodit.index.run.clear_file_processing_statuses"
	OperationRepository                      TaskOperation = "kodit.repository"
	OperationCreateRepository                TaskOperation = "kodit.repository.create"
	OperationDeleteRepository                TaskOperation = "kodit.repository.delete"
	OperationCloneRepository                 TaskOperation = "kodit.repository.clone"
	OperationSyncRepository                  TaskOperation = "kodit.repository.sync"
	OperationCommit                          TaskOperation = "kodit.commit"
	OperationExtractSnippetsForCommit        TaskOperation = "kodit.commit.extract_snippets"
	OperationCreateBM25IndexForCommit        TaskOperation = "kodit.commit.create_bm25_index"
	OperationCreateCodeEmbeddingsForCommit   TaskOperation = "kodit.commit.create_code_embeddings"
	OperationCreateSummaryEnrichmentForCommit TaskOperation = "kodit.commit.create_summary_enrichment"
	OperationCreateSummaryEmbeddingsForCommit TaskOperation = "kodit.commit.create_summary_embeddings"
	OperationCreateArchitectureEnrichmentForCommit TaskOperation = "kodit.commit.create_architecture_enrichment"
	OperationCreatePublicAPIDocsForCommit         TaskOperation = "kodit.commit.create_public_api_docs"
	OperationCreateCommitDescriptionForCommit     TaskOperation = "kodit.commit.create_commit_description"
	OperationCreateDatabaseSchemaForCommit        TaskOperation = "kodit.commit.create_database_schema"
	OperationCreateCookbookForCommit              TaskOperation = "kodit.commit.create_cookbook"
	OperationExtractExamplesForCommit             TaskOperation = "kodit.commit.extract_examples"
	OperationCreateExampleSummaryForCommit        TaskOperation = "kodit.commit.create_example_summary"
	OperationCreateExampleCodeEmbeddingsForCommit TaskOperation = "kodit.commit.create_example_code_embeddings"
	OperationCreateExampleSummaryEmbeddingsForCommit TaskOperation = "kodit.commit.create_example_summary_embeddings"
	OperationScanCommit                              TaskOperation = "kodit.commit.scan"
	OperationRescanCommit                            TaskOperation = "kodit.commit.rescan"
)

// String returns the string representation of the operation.
func (o TaskOperation) String() string {
	return string(o)
}

// IsRepositoryOperation returns true if this is a repository-level operation.
func (o TaskOperation) IsRepositoryOperation() bool {
	return strings.HasPrefix(string(o), "kodit.repository.")
}

// IsCommitOperation returns true if this is a commit-level operation.
func (o TaskOperation) IsCommitOperation() bool {
	return strings.HasPrefix(string(o), "kodit.commit.")
}

// PrescribedOperations provides predefined operation sequences for common workflows.
type PrescribedOperations struct{}

// CreateNewRepository returns the operations needed to create a new repository.
func (PrescribedOperations) CreateNewRepository() []TaskOperation {
	return []TaskOperation{
		OperationCloneRepository,
	}
}

// SyncRepository returns the operations needed to sync a repository.
func (PrescribedOperations) SyncRepository() []TaskOperation {
	return []TaskOperation{
		OperationSyncRepository,
	}
}

// ScanAndIndexCommit returns the full operation sequence for scanning and indexing a commit.
func (PrescribedOperations) ScanAndIndexCommit() []TaskOperation {
	return []TaskOperation{
		OperationScanCommit,
		OperationExtractSnippetsForCommit,
		OperationExtractExamplesForCommit,
		OperationCreateBM25IndexForCommit,
		OperationCreateCodeEmbeddingsForCommit,
		OperationCreateExampleCodeEmbeddingsForCommit,
		OperationCreateSummaryEnrichmentForCommit,
		OperationCreateExampleSummaryForCommit,
		OperationCreateSummaryEmbeddingsForCommit,
		OperationCreateExampleSummaryEmbeddingsForCommit,
		OperationCreateArchitectureEnrichmentForCommit,
		OperationCreatePublicAPIDocsForCommit,
		OperationCreateCommitDescriptionForCommit,
		OperationCreateDatabaseSchemaForCommit,
		OperationCreateCookbookForCommit,
	}
}

// IndexCommit returns the operation sequence for indexing an already-scanned commit.
func (PrescribedOperations) IndexCommit() []TaskOperation {
	return []TaskOperation{
		OperationExtractSnippetsForCommit,
		OperationCreateBM25IndexForCommit,
		OperationCreateCodeEmbeddingsForCommit,
		OperationCreateSummaryEnrichmentForCommit,
		OperationCreateSummaryEmbeddingsForCommit,
		OperationCreateArchitectureEnrichmentForCommit,
		OperationCreatePublicAPIDocsForCommit,
		OperationCreateCommitDescriptionForCommit,
		OperationCreateDatabaseSchemaForCommit,
		OperationCreateCookbookForCommit,
	}
}

// RescanCommit returns the operation sequence for rescanning a commit (full reindex).
func (PrescribedOperations) RescanCommit() []TaskOperation {
	return []TaskOperation{
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
		OperationCreateArchitectureEnrichmentForCommit,
		OperationCreatePublicAPIDocsForCommit,
		OperationCreateCommitDescriptionForCommit,
		OperationCreateDatabaseSchemaForCommit,
		OperationCreateCookbookForCommit,
	}
}
