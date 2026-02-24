# Requirements: CachingTransport Performance Test

## Background

`CachingTransport` is an `http.RoundTripper` that stores POST request/response pairs in a file-based SQLite database (`http_cache.db`). It is used to cache embedding API calls. The database is opened via `NewDatabaseWithConfig` with `SetMaxOpenConns(1)`, meaning all concurrent callers share a single connection — any parallel `RoundTrip` call must queue behind that one connection.

The user suspects that under parallel load (e.g. multiple goroutines embedding different code snippets simultaneously), this single-connection SQLite bottleneck causes measurable latency degradation.

## User Stories

**As a developer**, I want a performance test that measures `CachingTransport` throughput under parallel load, so I can confirm whether SQLite locking is the bottleneck and have a baseline to validate future fixes against.

**As a developer**, I want to see both cache-miss (read + write) and cache-hit (read only) scenarios measured in parallel, so I understand which operation is most affected by contention.

## Acceptance Criteria

1. A test in `kodit/test/performance/` exercises `CachingTransport` with N goroutines making concurrent `RoundTrip` calls.
2. The test measures and logs throughput (requests/sec) and p50/p99 latency for:
   - **Cache miss**: all requests go to an upstream (httptest.Server), results are written to SQLite.
   - **Cache hit**: all requests are already in SQLite, no upstream calls.
   - **Mixed**: half the keys are warm, half are cold.
3. The test is self-contained — it uses a `t.TempDir()` for the SQLite file and a local `httptest.Server`, requiring no external services.
4. Concurrency levels tested: 1, 4, 8, 16, 32 goroutines.
5. The test runs with a plain `go test -run TestCachingTransportPerformance -v ./test/performance/...` invocation (no special build tags required).
6. Results are printed to `t.Log` in a structured, human-readable table per scenario.