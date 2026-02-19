package task

import "testing"

func TestOperation_String(t *testing.T) {
	op := OperationScanCommit
	if op.String() != "kodit.commit.scan" {
		t.Errorf("String() = %q, want %q", op.String(), "kodit.commit.scan")
	}
}

func TestOperation_IsRepositoryOperation(t *testing.T) {
	tests := []struct {
		op   Operation
		want bool
	}{
		{OperationCloneRepository, true},
		{OperationSyncRepository, true},
		{OperationDeleteRepository, true},
		{OperationScanCommit, false},
		{OperationRoot, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			if got := tt.op.IsRepositoryOperation(); got != tt.want {
				t.Errorf("IsRepositoryOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOperation_IsCommitOperation(t *testing.T) {
	tests := []struct {
		op   Operation
		want bool
	}{
		{OperationScanCommit, true},
		{OperationRescanCommit, true},
		{OperationExtractSnippetsForCommit, true},
		{OperationCreateBM25IndexForCommit, true},
		{OperationCloneRepository, false},
		{OperationRoot, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			if got := tt.op.IsCommitOperation(); got != tt.want {
				t.Errorf("IsCommitOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrescribedOperations_All_ContainsAllWorkflows(t *testing.T) {
	po := PrescribedOperations{}
	all := po.All()

	if len(all) == 0 {
		t.Fatal("All() should return operations")
	}

	// Should contain operations from all workflows
	allSet := make(map[Operation]struct{})
	for _, op := range all {
		allSet[op] = struct{}{}
	}

	for _, workflow := range [][]Operation{
		po.CreateNewRepository(),
		po.SyncRepository(),
		po.ScanAndIndexCommit(),
		po.IndexCommit(),
		po.RescanCommit(),
	} {
		for _, op := range workflow {
			if _, ok := allSet[op]; !ok {
				t.Errorf("All() missing operation %v", op)
			}
		}
	}
}

func TestPrescribedOperations_All_NoDuplicates(t *testing.T) {
	po := PrescribedOperations{}
	all := po.All()

	seen := make(map[Operation]struct{})
	for _, op := range all {
		if _, ok := seen[op]; ok {
			t.Errorf("All() contains duplicate: %v", op)
		}
		seen[op] = struct{}{}
	}
}

func TestPrescribedOperations_CreateNewRepository(t *testing.T) {
	ops := PrescribedOperations{}.CreateNewRepository()
	if len(ops) == 0 {
		t.Fatal("CreateNewRepository() should return operations")
	}
	if ops[0] != OperationCloneRepository {
		t.Errorf("first operation = %v, want %v", ops[0], OperationCloneRepository)
	}
}

func TestPrescribedOperations_SyncRepository(t *testing.T) {
	ops := PrescribedOperations{}.SyncRepository()
	if len(ops) == 0 {
		t.Fatal("SyncRepository() should return operations")
	}
	if ops[0] != OperationSyncRepository {
		t.Errorf("first operation = %v, want %v", ops[0], OperationSyncRepository)
	}
}

func TestPrescribedOperations_ScanAndIndexCommit(t *testing.T) {
	ops := PrescribedOperations{}.ScanAndIndexCommit()

	if ops[0] != OperationScanCommit {
		t.Errorf("first operation = %v, want %v", ops[0], OperationScanCommit)
	}

	// Should contain extraction, indexing, and enrichment operations
	hasExtract := false
	hasBM25 := false
	hasEmbeddings := false
	hasSummary := false
	for _, op := range ops {
		switch op {
		case OperationExtractSnippetsForCommit:
			hasExtract = true
		case OperationCreateBM25IndexForCommit:
			hasBM25 = true
		case OperationCreateCodeEmbeddingsForCommit:
			hasEmbeddings = true
		case OperationCreateSummaryEnrichmentForCommit:
			hasSummary = true
		}
	}
	if !hasExtract {
		t.Error("ScanAndIndexCommit should include ExtractSnippets")
	}
	if !hasBM25 {
		t.Error("ScanAndIndexCommit should include CreateBM25Index")
	}
	if !hasEmbeddings {
		t.Error("ScanAndIndexCommit should include CreateCodeEmbeddings")
	}
	if !hasSummary {
		t.Error("ScanAndIndexCommit should include CreateSummaryEnrichment")
	}
}

func TestPrescribedOperations_RescanCommit_StartsWithRescan(t *testing.T) {
	ops := PrescribedOperations{}.RescanCommit()

	if ops[0] != OperationRescanCommit {
		t.Errorf("first operation = %v, want %v", ops[0], OperationRescanCommit)
	}
}

func TestPrescribedOperations_IndexCommit_NoScan(t *testing.T) {
	ops := PrescribedOperations{}.IndexCommit()

	for _, op := range ops {
		if op == OperationScanCommit || op == OperationRescanCommit {
			t.Errorf("IndexCommit should not include scan operation: %v", op)
		}
	}
}

func TestPrescribedOperations_AllOperationsAreValidConstants(t *testing.T) {
	po := PrescribedOperations{}

	validOps := map[Operation]struct{}{
		OperationRoot:                                    {},
		OperationCreateIndex:                             {},
		OperationRunIndex:                                {},
		OperationRefreshWorkingCopy:                      {},
		OperationDeleteOldSnippets:                       {},
		OperationExtractSnippets:                         {},
		OperationCreateBM25Index:                         {},
		OperationCreateCodeEmbeddings:                    {},
		OperationEnrichSnippets:                          {},
		OperationCreateTextEmbeddings:                    {},
		OperationUpdateIndexTimestamp:                    {},
		OperationClearFileProcessingStatuses:             {},
		OperationRepository:                              {},
		OperationCreateRepository:                        {},
		OperationDeleteRepository:                        {},
		OperationCloneRepository:                         {},
		OperationSyncRepository:                          {},
		OperationCommit:                                  {},
		OperationExtractSnippetsForCommit:                {},
		OperationCreateBM25IndexForCommit:                {},
		OperationCreateCodeEmbeddingsForCommit:           {},
		OperationCreateSummaryEnrichmentForCommit:        {},
		OperationCreateSummaryEmbeddingsForCommit:        {},
		OperationCreateArchitectureEnrichmentForCommit:   {},
		OperationCreatePublicAPIDocsForCommit:            {},
		OperationCreateCommitDescriptionForCommit:        {},
		OperationCreateDatabaseSchemaForCommit:           {},
		OperationCreateCookbookForCommit:                 {},
		OperationExtractExamplesForCommit:                {},
		OperationCreateExampleSummaryForCommit:           {},
		OperationCreateExampleCodeEmbeddingsForCommit:    {},
		OperationCreateExampleSummaryEmbeddingsForCommit: {},
		OperationScanCommit:                              {},
		OperationRescanCommit:                            {},
	}

	for _, op := range po.All() {
		if _, ok := validOps[op]; !ok {
			t.Errorf("prescribed operation %q is not a defined constant", op)
		}
	}
}
