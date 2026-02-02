package queue

import (
	"testing"
)

func TestTaskOperation_String(t *testing.T) {
	op := OperationScanCommit
	if op.String() != "kodit.commit.scan" {
		t.Errorf("String() = %v, want 'kodit.commit.scan'", op.String())
	}
}

func TestTaskOperation_IsRepositoryOperation(t *testing.T) {
	tests := []struct {
		op       TaskOperation
		expected bool
	}{
		{OperationCloneRepository, true},
		{OperationSyncRepository, true},
		{OperationDeleteRepository, true},
		{OperationCreateRepository, true},
		{OperationScanCommit, false},
		{OperationExtractSnippetsForCommit, false},
		{OperationRoot, false},
		{OperationRepository, false}, // kodit.repository doesn't have the trailing dot
	}

	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			if got := tt.op.IsRepositoryOperation(); got != tt.expected {
				t.Errorf("IsRepositoryOperation() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTaskOperation_IsCommitOperation(t *testing.T) {
	tests := []struct {
		op       TaskOperation
		expected bool
	}{
		{OperationScanCommit, true},
		{OperationExtractSnippetsForCommit, true},
		{OperationCreateBM25IndexForCommit, true},
		{OperationCreateCodeEmbeddingsForCommit, true},
		{OperationCreateSummaryEnrichmentForCommit, true},
		{OperationCreateArchitectureEnrichmentForCommit, true},
		{OperationCloneRepository, false},
		{OperationSyncRepository, false},
		{OperationRoot, false},
		{OperationCommit, false}, // kodit.commit doesn't have the trailing dot
	}

	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			if got := tt.op.IsCommitOperation(); got != tt.expected {
				t.Errorf("IsCommitOperation() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPrescribedOperations_CreateNewRepository(t *testing.T) {
	prescribed := PrescribedOperations{}
	ops := prescribed.CreateNewRepository()

	if len(ops) != 1 {
		t.Errorf("CreateNewRepository() length = %v, want 1", len(ops))
	}
	if ops[0] != OperationCloneRepository {
		t.Errorf("CreateNewRepository()[0] = %v, want OperationCloneRepository", ops[0])
	}
}

func TestPrescribedOperations_SyncRepository(t *testing.T) {
	prescribed := PrescribedOperations{}
	ops := prescribed.SyncRepository()

	if len(ops) != 1 {
		t.Errorf("SyncRepository() length = %v, want 1", len(ops))
	}
	if ops[0] != OperationSyncRepository {
		t.Errorf("SyncRepository()[0] = %v, want OperationSyncRepository", ops[0])
	}
}

func TestPrescribedOperations_ScanAndIndexCommit(t *testing.T) {
	prescribed := PrescribedOperations{}
	ops := prescribed.ScanAndIndexCommit()

	// Should have 15 operations
	if len(ops) != 15 {
		t.Errorf("ScanAndIndexCommit() length = %v, want 15", len(ops))
	}

	// First operation should be scan
	if ops[0] != OperationScanCommit {
		t.Errorf("ScanAndIndexCommit()[0] = %v, want OperationScanCommit", ops[0])
	}

	// Verify extract snippets is early in the sequence
	if ops[1] != OperationExtractSnippetsForCommit {
		t.Errorf("ScanAndIndexCommit()[1] = %v, want OperationExtractSnippetsForCommit", ops[1])
	}

	// Verify all are commit operations (except scan itself)
	for i, op := range ops {
		if !op.IsCommitOperation() {
			t.Errorf("ScanAndIndexCommit()[%d] = %v is not a commit operation", i, op)
		}
	}
}

func TestPrescribedOperations_IndexCommit(t *testing.T) {
	prescribed := PrescribedOperations{}
	ops := prescribed.IndexCommit()

	// Should have 10 operations (no scan, no examples)
	if len(ops) != 10 {
		t.Errorf("IndexCommit() length = %v, want 10", len(ops))
	}

	// First operation should be extract snippets
	if ops[0] != OperationExtractSnippetsForCommit {
		t.Errorf("IndexCommit()[0] = %v, want OperationExtractSnippetsForCommit", ops[0])
	}

	// Verify all are commit operations
	for i, op := range ops {
		if !op.IsCommitOperation() {
			t.Errorf("IndexCommit()[%d] = %v is not a commit operation", i, op)
		}
	}
}

func TestPrescribedOperations_ScanAndIndexCommit_ContainsAllExpectedOperations(t *testing.T) {
	prescribed := PrescribedOperations{}
	ops := prescribed.ScanAndIndexCommit()

	expected := []TaskOperation{
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

	if len(ops) != len(expected) {
		t.Fatalf("ScanAndIndexCommit() length = %v, want %v", len(ops), len(expected))
	}

	for i, exp := range expected {
		if ops[i] != exp {
			t.Errorf("ScanAndIndexCommit()[%d] = %v, want %v", i, ops[i], exp)
		}
	}
}
