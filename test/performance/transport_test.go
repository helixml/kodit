package performance_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/helixml/kodit/infrastructure/provider"
)

const (
	// iterations is the number of RoundTrip calls each goroutine makes.
	iterations = 50
)

// latencyStats computes p50 and p99 from a flat slice of durations.
// The slice is sorted in place.
func latencyStats(d []time.Duration) (p50, p99 time.Duration) {
	sortDurations(d) // defined in external_embedding_test.go
	n := len(d)
	if n == 0 {
		return 0, 0
	}
	p50 = d[n*50/100]
	p99idx := n * 99 / 100
	if p99idx >= n {
		p99idx = n - 1
	}
	p99 = d[p99idx]
	return
}

// runParallel launches goroutines concurrently, each executing fn(goroutineID, iteration).
// It returns the per-goroutine latency slices and the total wall-clock duration.
func runParallel(t *testing.T, goroutines int, fn func(gid, iter int) time.Duration) ([][]time.Duration, time.Duration) {
	t.Helper()
	perGoroutine := make([][]time.Duration, goroutines)
	for i := range perGoroutine {
		perGoroutine[i] = make([]time.Duration, iterations)
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)

	start := time.Now()
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range iterations {
				perGoroutine[g][i] = fn(g, i)
			}
		}(g)
	}
	wg.Wait()
	wall := time.Since(start)

	return perGoroutine, wall
}

// flattenDurations merges per-goroutine duration slices into one flat slice.
func flattenDurations(perGoroutine [][]time.Duration) []time.Duration {
	var all []time.Duration
	for _, s := range perGoroutine {
		all = append(all, s...)
	}
	return all
}

// printRow logs a single results row.
func printRow(t *testing.T, label string, goroutines int, wall time.Duration, durations []time.Duration) {
	t.Helper()
	total := goroutines * iterations
	reqPerSec := float64(total) / wall.Seconds()
	p50, p99 := latencyStats(durations)
	t.Logf("%-10s  goroutines=%-3d  total_reqs=%-5d  wall=%8v  req/sec=%8.1f  p50=%8v  p99=%8v",
		label, goroutines, total, wall.Round(time.Millisecond), reqPerSec, p50.Round(time.Microsecond), p99.Round(time.Microsecond))
}

// newUpstream creates an httptest.Server that returns a trivial JSON body and
// increments the provided counter on every request.
func newUpstream(counter *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
}

// TestCachingTransportPerformance measures CachingTransport throughput and latency
// under parallel load. It covers three scenarios:
//
//   - cache_miss: every request body is unique → always cold → exercises the
//     read-miss + upstream + write path under contention.
//   - cache_hit: all goroutines use the same pre-warmed body → read-only
//     contention, no upstream calls during parallel phase.
//   - mixed: half the goroutines use a warm shared key, half use unique
//     cold keys.
//
// Each scenario sweeps goroutine counts [1, 4, 8, 16, 32].
// No external services are required — the upstream is an httptest.Server and
// the SQLite database is placed in t.TempDir().
func TestCachingTransportPerformance(t *testing.T) {
	concurrencyLevels := []int{1, 4, 8, 16, 32}

	// --- cache_miss ---------------------------------------------------------
	t.Run("cache_miss", func(t *testing.T) {
		t.Log("Scenario: every request body is unique — always a cache miss (read + upstream + write)")

		for _, goroutines := range concurrencyLevels {
			goroutines := goroutines
			t.Run(fmt.Sprintf("goroutines_%d", goroutines), func(t *testing.T) {
				var upstreamCalls atomic.Int32
				srv := newUpstream(&upstreamCalls)
				defer srv.Close()

				transport, err := provider.NewCachingTransport(t.TempDir(), srv.Client().Transport)
				if err != nil {
					t.Fatalf("create transport: %v", err)
				}
				defer func() { _ = transport.Close() }()

				perGoroutine, wall := runParallel(t, goroutines, func(gid, iter int) time.Duration {
					// Unique body per call guarantees a cache miss every time.
					body := fmt.Sprintf(`{"gid":%d,"iter":%d}`, gid, iter)
					req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/embeddings", strings.NewReader(body))
					start := time.Now()
					resp, err := transport.RoundTrip(req)
					elapsed := time.Since(start)
					if err != nil {
						t.Errorf("RoundTrip error: %v", err)
						return elapsed
					}
					_ = resp.Body.Close()
					return elapsed
				})

				expectedCalls := int32(goroutines * iterations)
				if got := upstreamCalls.Load(); got != expectedCalls {
					t.Errorf("upstream calls: got %d, want %d", got, expectedCalls)
				}

				printRow(t, "cache_miss", goroutines, wall, flattenDurations(perGoroutine))
			})
		}
	})

	// --- cache_hit ----------------------------------------------------------
	t.Run("cache_hit", func(t *testing.T) {
		t.Log("Scenario: all goroutines use the same pre-warmed key — read-only contention, no upstream calls during parallel phase")

		for _, goroutines := range concurrencyLevels {
			goroutines := goroutines
			t.Run(fmt.Sprintf("goroutines_%d", goroutines), func(t *testing.T) {
				var upstreamCalls atomic.Int32
				srv := newUpstream(&upstreamCalls)
				defer srv.Close()

				transport, err := provider.NewCachingTransport(t.TempDir(), srv.Client().Transport)
				if err != nil {
					t.Fatalf("create transport: %v", err)
				}
				defer func() { _ = transport.Close() }()

				// Warm the cache with a single serial call.
				warmBody := `{"input":"warm-key"}`
				warmReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/embeddings", strings.NewReader(warmBody))
				warmResp, err := transport.RoundTrip(warmReq)
				if err != nil {
					t.Fatalf("warm request: %v", err)
				}
				_ = warmResp.Body.Close()
				upstreamCalls.Store(0) // reset counter before the parallel phase

				perGoroutine, wall := runParallel(t, goroutines, func(_, _ int) time.Duration {
					req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/embeddings", strings.NewReader(warmBody))
					start := time.Now()
					resp, err := transport.RoundTrip(req)
					elapsed := time.Since(start)
					if err != nil {
						t.Errorf("RoundTrip error: %v", err)
						return elapsed
					}
					_ = resp.Body.Close()
					return elapsed
				})

				// No upstream calls expected during the parallel phase.
				if got := upstreamCalls.Load(); got != 0 {
					t.Errorf("upstream calls during parallel phase: got %d, want 0", got)
				}

				printRow(t, "cache_hit", goroutines, wall, flattenDurations(perGoroutine))
			})
		}
	})

	// --- mixed --------------------------------------------------------------
	t.Run("mixed", func(t *testing.T) {
		t.Log("Scenario: half goroutines use a warm shared key (hits), half use unique cold keys (misses)")

		for _, goroutines := range concurrencyLevels {
			goroutines := goroutines
			t.Run(fmt.Sprintf("goroutines_%d", goroutines), func(t *testing.T) {
				var upstreamCalls atomic.Int32
				srv := newUpstream(&upstreamCalls)
				defer srv.Close()

				transport, err := provider.NewCachingTransport(t.TempDir(), srv.Client().Transport)
				if err != nil {
					t.Fatalf("create transport: %v", err)
				}
				defer func() { _ = transport.Close() }()

				// Warm the shared key.
				sharedBody := `{"input":"shared-warm-key"}`
				warmReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/embeddings", strings.NewReader(sharedBody))
				warmResp, err := transport.RoundTrip(warmReq)
				if err != nil {
					t.Fatalf("warm request: %v", err)
				}
				_ = warmResp.Body.Close()
				upstreamCalls.Store(0) // reset before parallel phase

				// Even-numbered goroutines hit the warm shared key.
				// Odd-numbered goroutines use a unique body per call (cold).
				perGoroutine, wall := runParallel(t, goroutines, func(gid, iter int) time.Duration {
					var body string
					if gid%2 == 0 {
						body = sharedBody
					} else {
						body = fmt.Sprintf(`{"gid":%d,"iter":%d,"cold":true}`, gid, iter)
					}
					req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/embeddings", strings.NewReader(body))
					start := time.Now()
					resp, err := transport.RoundTrip(req)
					elapsed := time.Since(start)
					if err != nil {
						t.Errorf("RoundTrip error: %v", err)
						return elapsed
					}
					_ = resp.Body.Close()
					return elapsed
				})

				printRow(t, "mixed", goroutines, wall, flattenDurations(perGoroutine))
			})
		}
	})
}
