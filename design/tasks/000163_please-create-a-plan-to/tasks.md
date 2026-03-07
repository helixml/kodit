# Implementation Tasks

## Setup / shared helpers

- [ ] Verify `make test PKG=./application/handler/...` passes before starting (establish baseline)

## `application/handler/repository/` — new `handler_test.go`

- [ ] `TestClone_PersistsWorkingCopy`: Execute Clone handler with a fake Cloner; assert repo's working copy path is saved in DB
- [ ] `TestClone_SkipsIfAlreadyCloned`: Repo already has a working copy; assert Execute returns nil and cloner is not called
- [ ] `TestDelete_RemovesRepositoryAndEnrichments`: Seed repo + enrichments; Execute Delete; assert repo and enrichments are gone from DB
- [ ] `TestDelete_RemovesWorkingCopyFromDisk`: Create a real temp dir as working copy; Execute Delete; assert dir no longer exists
- [ ] `TestSync_EnqueuesCommitScan`: Execute Sync with a fake Cloner+Scanner that returns a branch with a head SHA; assert a task is enqueued in the task store

## `application/handler/enrichment/handler_test.go` — extend existing file

- [ ] `TestArchitectureDiscovery_CreatesEnrichment`: Fake discoverer returns a string; fake enricher returns summary; assert one `TypeArchitecture/SubtypePhysical` enrichment created with commit association
- [ ] `TestArchitectureDiscovery_SkipsWhenExists`: Run handler twice; assert enrichment count unchanged on second run
- [ ] `TestCookbook_CreatesEnrichment`: Seed files for commit; fake context gatherer; fake enricher; assert one `TypeUsage/SubtypeCookbook` enrichment created
- [ ] `TestCookbook_SkipsWhenNoFiles`: No files for commit; assert skips cleanly
- [ ] `TestDatabaseSchema_CreatesEnrichment`: Fake discoverer returns schema report; assert one `TypeArchitecture/SubtypeDatabaseSchema` enrichment created
- [ ] `TestDatabaseSchema_SkipsWhenNoSchema`: Discoverer returns "No database schemas detected"; assert no enrichment created
- [ ] `TestAPIDocs_CreatesEnrichment`: Seed files with a language; fake extractor returns enrichments; assert enrichments saved with commit association
- [ ] `TestAPIDocs_SkipsWhenNoFiles`: No files for commit; assert skips cleanly
- [ ] `TestExtractExamples_SavesExamplesFromGoFile`: Write a `.go` file to `t.TempDir()`; run handler; assert `SubtypeExample` enrichments created with commit associations
- [ ] `TestExampleSummary_CreatesSummariesForExamples`: Seed `SubtypeExample` enrichments; run handler; assert one `SubtypeExampleSummary` per example created and linked

## `application/handler/indexing/` — new `handler_test.go`

- [ ] `TestCreateBM25Index_IndexesEnrichments`: Seed chunk enrichments with commit SHA; run handler; assert `bm25Store.Find` returns results for a keyword from the content
- [ ] `TestCreateBM25Index_SkipsWhenNoEnrichments`: No enrichments for commit; assert Execute returns nil
- [ ] `TestCreateCodeEmbeddings_SendsDocumentsToEmbedding`: Seed snippet enrichments; use `recordingEmbedding`; run handler; assert correct number of documents sent
- [ ] `TestCreateCodeEmbeddings_SkipsAlreadyEmbedded`: Pre-populate SQLite embedding store with one ID; run handler; assert only new enrichments sent to embedding service
- [ ] `TestCreateExampleCodeEmbeddings_IndexesExamples`: Seed `SubtypeExample` enrichments; run handler; assert documents sent to embedding service
- [ ] `TestCreateExampleSummaryEmbeddings_IndexesSummaries`: Seed `SubtypeExampleSummary` enrichments; run handler; assert documents sent

## Verification

- [ ] `make test PKG=./application/handler/...` passes green with all new tests
