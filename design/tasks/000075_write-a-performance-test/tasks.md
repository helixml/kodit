# Implementation Tasks

- [x] Create `kodit/test/performance/transport_test.go` with package `performance_test`
- [x] Add imports: `net/http`, `net/http/httptest`, `sync`, `sync/atomic`, `sort`, `fmt`, `testing`, `time`, `strings`, `github.com/helixml/kodit/infrastructure/provider`
- [x] Implement helper `latencyStats(durations []time.Duration) (p50, p99 time.Duration)` that sorts and indexes the slice
- [x] Implement `TestCachingTransportPerformance` as the top-level test function
- [x] Add `cache_miss` sub-test: each goroutine sends unique request bodies (goroutineID + iteration index in body) so the cache is always cold; measure RoundTrip latency per call
- [x] Add `cache_hit` sub-test: warm the cache with one serial request, then all goroutines send the same body; measure RoundTrip latency per call
- [x] Add `mixed` sub-test: half the goroutines use a shared warm key, half use unique cold keys
- [x] Loop over concurrency levels `[]int{1, 4, 8, 16, 32}` inside each sub-test using `t.Run(fmt.Sprintf("goroutines_%d", n), ...)`
- [x] Use `sync.WaitGroup` to coordinate goroutines; pre-allocate `[][]time.Duration` (one slice per goroutine, length = iterations) to collect latencies without a mutex on the hot path
- [x] Use `atomic.Int32` to count upstream calls and assert correctness (cache_hit must produce exactly 1 upstream call regardless of goroutine count)
- [x] Use `t.TempDir()` for the SQLite file directory and `httptest.NewServer` for the upstream — no external dependencies
- [x] Set a fixed iteration count per goroutine (50) so total work scales with concurrency
- [x] Log a human-readable results table per scenario with `t.Logf`: columns `goroutines`, `total_reqs`, `wall_time`, `req/sec`, `p50`, `p99`
- [x] Verify the test runs with `CGO_ENABLED=1 go test -tags "fts5" -run TestCachingTransportPerformance -v ./test/performance/...` and produces readable output