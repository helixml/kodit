# Implementation Tasks

- [ ] Update `github.com/helixml/kodit` in `go.mod` to commit `417f16b7dfce928b0e9d1a888454cfc6cbe98892` (`go get github.com/helixml/kodit@417f16b && go mod tidy`)
- [ ] Add `RAGProviderKodit RAGProvider = "kodit"` constant to `api/pkg/config/config.go`
- [ ] Create `api/pkg/rag/rag_kodit.go` with `KoditRAG` struct implementing `rag.RAG`:
  - `Index()` is a no-op
  - `Query()` resolves repo ID from `.kodit_meta.json` and calls `koditClient.Search.Query()` filtered by repo ID
  - `Delete()` resolves repo ID, calls `koditClient.Repositories.Delete()`, removes `.kodit_meta.json`
- [ ] Add `indexKoditFilestore()` method to `Reconciler` in `api/pkg/controller/knowledge/`:
  - Requires `k.Source.Filestore != nil`, errors otherwise
  - Builds local path from `config.FileStore.LocalFSPath + k.Source.Filestore.Path`
  - Calls `koditClient.Repositories.Add()` with `file://` URI
  - Persists repo ID to `.kodit_meta.json` in the filestore dir
  - Polls `koditClient.WorkerIdle()` until indexing completes
- [ ] Add early-return branch in `indexKnowledge()` to call `indexKoditFilestore()` when provider is `kodit`
- [ ] Instantiate `KoditRAG` in server startup when `RAG_DEFAULT_PROVIDER=kodit` (pass `config.FileStore.LocalFSPath` and the existing kodit client)
- [ ] Write unit tests for `KoditRAG.Index`, `Query`, `Delete` (mock kodit client and file I/O)
- [ ] Write unit test for `indexKoditFilestore` covering: filestore path construction, metadata write, non-filestore error
- [ ] Add `rag_kodit.go` to mock generation (`go generate ./api/pkg/rag/`)
- [ ] Manually test: set `KODIT_ENABLED=true`, `RAG_DEFAULT_PROVIDER=kodit`, upload files to a knowledge source, verify they are searchable via kodit semantic search
