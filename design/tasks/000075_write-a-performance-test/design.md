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