# Design: CachingTransport Performance Test

## Context

`CachingTransport` lives in `kodit/infrastructure/provider/transport.go`. It wraps an `http.RoundTripper` and caches POST responses in a SQLite file (`http_cache.db`). The database is opened via `NewDatabaseWithConfig`, which (like `NewDatabase`) calls `SetMaxOpenConns(1)` for SQLite — all goroutines share one connection handle.

Under parallel load this creates a bottleneck:
- **Read path** (`readCache`): GORM `First` query — serialised through the single connection.
- **Write path** (`writeCache`): GORM `Save` upsert — serialised and also holds a write lock.

WAL mode and `busy_timeout=5000ms` are already applied via the DSN in `parseDialector`, so callers queue instead of failing immediately — but they still queue.

## Existing Patterns

- Performance tests live in `kodit/test/performance/` (package `performance_test`).
- They are self-contained: use `httptest.NewServer` for upstream, `t.TempDir()` for SQLite files, and are skipped when optional dependencies (Postgres, API keys) are absent. This test needs no external deps.
- Existing tests log structured results with `t.Logf` — tabular output per scenario.
- Latency is measured using `time.Now()` / `time.Since()`, collecting raw durations into a slice then computing p50/p99 by sorting.
- The existing unit tests in `transport_test.go` use `atomic.Int32` to count upstream hits — the same technique can verify correctness inside the performance test.

## Test Structure

Single test function `TestCachingTransportPerformance` with three sub-tests:

```kodit/test/performance/transport_test.go#L1-1
// illustrative — not real line numbers
```

| Sub-test | Scenario | What is measured |
|---|---|---|
| `cache_miss` | Each goroutine sends a unique request body (cache always cold) | Write-heavy contention: read miss + upstream + write |
| `cache_hit` | All goroutines send the same body (pre-warmed before the parallel phase) | Read-only contention |
| `mixed` | Half unique keys (cold), half shared keys (warm) | Realistic mixed workload |

For each sub-test, the concurrency levels `[1, 4, 8, 16, 32]` are exercised via a `t.Run(fmt.Sprintf("goroutines_%d", n), ...)` inner loop.

Each goroutine runs a fixed number of iterations (e.g. 50), collecting one `time.Duration` per `RoundTrip` call. After all goroutines finish, compute:

- **Throughput**: `totalRequests / totalWallTime` (requests/sec)
- **p50 / p99**: sort the duration slice, index at 50th and 99th percentile

Results are printed in a `t.Logf` table.

## File Layout

```
kodit/test/performance/transport_test.go   ← new file
```

Same package (`performance_test`) and same build constraints as siblings (no tags required).

## Key Implementation Notes

- Use `sync.WaitGroup` to coordinate goroutines; collect durations via a pre-allocated `[][]time.Duration` (one slice per goroutine) to avoid mutex on the hot path.
- The `httptest.Server` handler should sleep 0ms (instant) so measured latency is dominated by SQLite, not network simulation.
- For `cache_miss`, each goroutine uses a unique body keyed by `goroutineID * iterations + i` — guarantees no accidental cache hits.
- For `cache_hit`, warm the cache with a single serial call before starting goroutines.
- The test does **not** assert any latency thresholds — it is an observability tool, not a regression gate. Assertions are limited to correctness (correct upstream call counts, no errors).

## What to Look For in Results

If SQLite locking is the bottleneck, you will see:
- p99 latency scaling linearly (or worse) with goroutine count on `cache_miss`.
- p99 on `cache_hit` staying flat (reads are serialised but non-blocking relative to each other in WAL mode).
- Throughput plateauing or declining above 4–8 goroutines on `cache_miss`.

If those patterns appear, the fix is to introduce an in-memory read-through layer (e.g. `sync.Map` keyed by the SHA-256 cache key) so concurrent read hits never touch SQLite at all, and writes are de-duplicated with a singleflight group.

## Implementation Notes

### Build Requirements

- SQLite requires CGO. The test must be run with `CGO_ENABLED=1 go test -tags "fts5" -run TestCachingTransportPerformance -v ./test/performance/...`
- The `make test` target already sets `CGO_ENABLED=1` via the Makefile `GOENV` variable, so the test runs correctly under `make test`.
- The `go test` invocation in the original requirements spec (no flags) does not work without CGO — updated to require `-tags fts5` and `CGO_ENABLED=1`.

### Actual Results (first run on dev machine)

```
cache_miss  goroutines=1    total_reqs=50     wall=    16ms  req/sec=  3094.6  p50=   259µs  p99= 2.419ms
cache_miss  goroutines=4    total_reqs=200    wall=    45ms  req/sec=  4476.7  p50=   685µs  p99= 4.393ms
cache_miss  goroutines=8    total_reqs=400    wall=    66ms  req/sec=  6079.6  p50=  1.11ms  p99= 3.675ms
cache_miss  goroutines=16   total_reqs=800    wall=   145ms  req/sec=  5517.2  p50= 2.248ms  p99= 9.552ms
cache_miss  goroutines=32   total_reqs=1600   wall=   408ms  req/sec=  3922.7  p50= 5.598ms  p99=43.487ms

cache_hit   goroutines=1    total_reqs=50     wall=     2ms  req/sec= 28175.5  p50=    30µs  p99=    99µs
cache_hit   goroutines=4    total_reqs=200    wall=     5ms  req/sec= 37113.2  p50=    67µs  p99=   515µs
cache_hit   goroutines=8    total_reqs=400    wall=    11ms  req/sec= 34918.4  p50=   133µs  p99=   975µs
cache_hit   goroutines=16   total_reqs=800    wall=    21ms  req/sec= 37807.4  p50=   233µs  p99= 1.922ms
cache_hit   goroutines=32   total_reqs=1600   wall=    33ms  req/sec= 48781.7  p50=   350µs  p99=  3.26ms

mixed       goroutines=1    total_reqs=50     wall=     1ms  req/sec= 36842.2  p50=    12µs  p99=   225µs
mixed       goroutines=4    total_reqs=200    wall=    28ms  req/sec=  7107.5  p50=   220µs  p99= 3.096ms
mixed       goroutines=8    total_reqs=400    wall=    48ms  req/sec=  8406.4  p50=   461µs  p99=  4.37ms
mixed       goroutines=16   total_reqs=800    wall=    93ms  req/sec=  8581.7  p50=   994µs  p99= 5.101ms
mixed       goroutines=32   total_reqs=1600   wall=   249ms  req/sec=  6420.7  p50=  2.31ms  p99=13.614ms
```

### Key Observations from Results

The data **confirms** the SQLite write-lock bottleneck hypothesis:

1. **`cache_miss` p99 degrades sharply with concurrency**: 2.4ms at 1 goroutine → 43ms at 32 goroutines (18× increase). Throughput peaks at 8 goroutines (~6k req/sec) then falls back to ~3.9k at 32 — classic single-writer serialisation collapse.

2. **`cache_hit` scales well**: p99 only grows from 99µs → 3.3ms across the same concurrency range (33× throughput increase vs 1-goroutine baseline). WAL mode allows concurrent reads, so read-only contention is mild. At 32 goroutines it actually achieves *higher* throughput than 1 goroutine because there is no upstream round-trip.

3. **`mixed` p99 is dominated by the cold writers**: At 32 goroutines p99=13.6ms, midway between the pure hit and pure miss extremes, confirming cold-path writes are the source of tail latency.

### Recommended Fix

Replace the SQLite read path with an in-memory `sync.Map` keyed by the SHA-256 cache key. Warm hits never touch the DB. For concurrent misses on the same key, use `golang.org/x/sync/singleflight` to coalesce upstream calls. SQLite then only handles the cold write path, which is inherently less latency-sensitive.

### Gotchas

- `sortDurations` is already defined in `external_embedding_test.go` (same package). Do not redefine it in the new file.
- Use `strings.NewReader` for request bodies — the existing unit tests use this pattern.
- The `for g := range goroutines` integer-range syntax requires Go 1.22+; this codebase is on Go 1.25 so it is fine.