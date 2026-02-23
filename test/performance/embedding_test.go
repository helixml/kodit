package performance_test

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/database"
	"github.com/stretchr/testify/require"
)

const (
	// pgURL is the connection string for the local VectorChord PostgreSQL.
	pgURL = "postgresql://postgres:mysecretpassword@localhost:5432/kodit"

	// embeddingDimension is the output dimension of st-codesearch-distilroberta-base.
	embeddingDimension = 768
)

// embeddingAdapter adapts provider.Embedder to domain search.Embedder.
type embeddingAdapter struct {
	inner provider.Embedder
}

func (a *embeddingAdapter) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	resp, err := a.inner.Embed(ctx, provider.NewEmbeddingRequest(texts))
	if err != nil {
		return nil, err
	}
	return resp.Embeddings(), nil
}

// testDB connects to the VectorChord PostgreSQL instance and drops any
// leftover performance test tables. Returns the database and a cleanup function.
func testDB(t *testing.T) database.Database {
	t.Helper()

	ctx := context.Background()
	db, err := database.NewDatabase(ctx, pgURL)
	if err != nil {
		t.Skipf("cannot connect to VectorChord at %s: %v (start with: docker compose -f docker-compose.dev.yaml --profile vectorchord up -d)", pgURL, err)
	}

	// Drop performance test tables so each run starts clean.
	raw := db.Session(ctx)
	raw.Exec("DROP TABLE IF EXISTS vectorchord_perf_embeddings CASCADE")

	t.Cleanup(func() {
		raw := db.Session(context.Background())
		raw.Exec("DROP TABLE IF EXISTS vectorchord_perf_embeddings CASCADE")
		_ = db.Close()
	})

	return db
}

// testEmbedder creates a HugotEmbedding provider. Skips if the model
// is not compiled in (requires -tags embed_model).
func testEmbedder(t *testing.T) *provider.HugotEmbedding {
	t.Helper()
	modelDir := t.TempDir()
	emb := provider.NewHugotEmbedding(modelDir)
	if !emb.Available() {
		t.Skip("skipping: requires -tags embed_model for built-in ONNX model")
	}
	t.Cleanup(func() { _ = emb.Close() })
	return emb
}

// sampleCodeSnippets returns realistic code snippets for embedding.
func sampleCodeSnippets(n int) []search.Document {
	snippets := []string{
		"func HandleLogin(w http.ResponseWriter, r *http.Request) {\n\tvar creds Credentials\n\tif err := json.NewDecoder(r.Body).Decode(&creds); err != nil {\n\t\thttp.Error(w, \"bad request\", 400)\n\t\treturn\n\t}\n}",
		"type UserRepository struct {\n\tdb *gorm.DB\n}\n\nfunc (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {\n\tvar user User\n\terr := r.db.WithContext(ctx).Where(\"email = ?\", email).First(&user).Error\n\treturn &user, err\n}",
		"func TestCreateOrder(t *testing.T) {\n\tdb := testdb.New(t)\n\tstore := NewOrderStore(db)\n\torder := Order{UserID: 1, Total: 99.99}\n\terr := store.Create(context.Background(), &order)\n\trequire.NoError(t, err)\n\trequire.NotZero(t, order.ID)\n}",
		"class PaymentProcessor:\n    def __init__(self, gateway: PaymentGateway):\n        self.gateway = gateway\n\n    def charge(self, amount: Decimal, card_token: str) -> Receipt:\n        result = self.gateway.authorize(amount, card_token)\n        if result.approved:\n            return self.gateway.capture(result.transaction_id)\n        raise PaymentDeclined(result.reason)",
		"const fetchUsers = async (page: number): Promise<User[]> => {\n  const response = await fetch(`/api/users?page=${page}`);\n  if (!response.ok) throw new Error(`HTTP ${response.status}`);\n  const data = await response.json();\n  return data.users;\n};",
		"impl Iterator for TokenStream {\n    type Item = Token;\n    fn next(&mut self) -> Option<Self::Item> {\n        while self.pos < self.input.len() {\n            let ch = self.input[self.pos];\n            self.pos += 1;\n            if !ch.is_whitespace() {\n                return Some(Token::new(ch, self.pos - 1));\n            }\n        }\n        None\n    }\n}",
		"SELECT u.id, u.name, COUNT(o.id) as order_count\nFROM users u\nLEFT JOIN orders o ON o.user_id = u.id\nWHERE u.created_at > NOW() - INTERVAL '30 days'\nGROUP BY u.id, u.name\nHAVING COUNT(o.id) > 5\nORDER BY order_count DESC;",
		"func (s *Server) gracefulShutdown(ctx context.Context) error {\n\tctx, cancel := context.WithTimeout(ctx, 30*time.Second)\n\tdefer cancel()\n\ts.logger.Info(\"shutting down HTTP server\")\n\tif err := s.httpServer.Shutdown(ctx); err != nil {\n\t\treturn fmt.Errorf(\"http shutdown: %w\", err)\n\t}\n\ts.logger.Info(\"server stopped\")\n\treturn nil\n}",
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api-server\nspec:\n  replicas: 3\n  selector:\n    matchLabels:\n      app: api-server\n  template:\n    spec:\n      containers:\n      - name: api\n        image: myapp:latest\n        resources:\n          limits:\n            memory: 256Mi\n            cpu: 500m",
		"func BenchmarkSort(b *testing.B) {\n\tdata := make([]int, 10000)\n\tfor i := range data {\n\t\tdata[i] = rand.Intn(100000)\n\t}\n\tb.ResetTimer()\n\tfor i := 0; i < b.N; i++ {\n\t\tcp := make([]int, len(data))\n\t\tcopy(cp, data)\n\t\tsort.Ints(cp)\n\t}\n}",
	}

	documents := make([]search.Document, n)
	for i := range documents {
		text := snippets[i%len(snippets)]
		documents[i] = search.NewDocument(
			fmt.Sprintf("snippet-%06d", i),
			text,
		)
	}
	return documents
}

// randomVector generates a random float64 vector of the given dimension.
func randomVector(dim int) []float64 {
	v := make([]float64, dim)
	for i := range v {
		v[i] = rand.Float64()*2 - 1
	}
	return v
}

// TestEmbeddingPipeline profiles the full embedding pipeline:
// model inference, database storage, and vector search.
func TestEmbeddingPipeline(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	embedder := testEmbedder(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	store, err := persistence.NewVectorChordEmbeddingStore(
		ctx, db, "perf", embeddingDimension, logger,
	)
	require.NoError(t, err)

	adapter := &embeddingAdapter{inner: embedder}
	svc, err := domainservice.NewEmbedding(store, adapter)
	require.NoError(t, err)

	// --- Phase 1: ONNX Model Inference ---
	t.Run("model_inference", func(t *testing.T) {
		batchSizes := []int{1, 10, 32, 64, 100}
		for _, size := range batchSizes {
			t.Run(fmt.Sprintf("batch_%d", size), func(t *testing.T) {
				texts := make([]string, size)
				for i := range texts {
					texts[i] = fmt.Sprintf("func Handle%d(ctx context.Context) error { return nil }", i)
				}

				start := time.Now()
				req := provider.NewEmbeddingRequest(texts)
				resp, err := embedder.Embed(ctx, req)
				elapsed := time.Since(start)
				require.NoError(t, err)

				embeddings := resp.Embeddings()
				require.Len(t, embeddings, size)

				perItem := elapsed / time.Duration(size)
				t.Logf("batch=%d  total=%v  per_item=%v  items/sec=%.1f",
					size, elapsed, perItem, float64(size)/elapsed.Seconds())
			})
		}
	})

	// --- Phase 2: Database Storage (SaveAll + index creation) ---
	t.Run("database_storage", func(t *testing.T) {
		counts := []int{10, 50, 100, 500}
		for _, count := range counts {
			t.Run(fmt.Sprintf("save_%d", count), func(t *testing.T) {
				// Generate pre-computed embeddings to isolate DB performance
				embeddings := make([]search.Embedding, count)
				for i := range embeddings {
					embeddings[i] = search.NewEmbedding(
						fmt.Sprintf("save-test-%d-%06d", count, i),
						randomVector(embeddingDimension),
					)
				}

				start := time.Now()
				err := store.SaveAll(ctx, embeddings)
				elapsed := time.Since(start)
				require.NoError(t, err)

				perItem := elapsed / time.Duration(count)
				t.Logf("count=%d  total=%v  per_item=%v  items/sec=%.1f",
					count, elapsed, perItem, float64(count)/elapsed.Seconds())
			})
		}
	})

	// --- Phase 3: Vector Search Performance ---
	t.Run("vector_search", func(t *testing.T) {
		// First, populate with a fixed dataset for search tests
		const datasetSize = 500
		embeddings := make([]search.Embedding, datasetSize)
		for i := range embeddings {
			embeddings[i] = search.NewEmbedding(
				fmt.Sprintf("search-dataset-%06d", i),
				randomVector(embeddingDimension),
			)
		}
		err := store.SaveAll(ctx, embeddings)
		require.NoError(t, err)

		queryVector := randomVector(embeddingDimension)

		limits := []int{5, 10, 50}
		for _, limit := range limits {
			t.Run(fmt.Sprintf("top_%d", limit), func(t *testing.T) {
				const iterations = 20
				var total time.Duration

				for range iterations {
					start := time.Now()
					results, err := store.Search(ctx,
						search.WithEmbedding(queryVector),
						repository.WithLimit(limit),
					)
					elapsed := time.Since(start)
					require.NoError(t, err)
					require.Len(t, results, limit)
					total += elapsed
				}

				avg := total / iterations
				t.Logf("limit=%d  iterations=%d  avg=%v  total=%v  queries/sec=%.1f",
					limit, iterations, avg, total, float64(iterations)/total.Seconds())
			})
		}
	})

	// --- Phase 4: End-to-End Index Pipeline ---
	t.Run("end_to_end_index", func(t *testing.T) {
		counts := []int{10, 50, 100}
		for _, count := range counts {
			t.Run(fmt.Sprintf("index_%d", count), func(t *testing.T) {
				documents := sampleCodeSnippets(count)
				// Give unique IDs for this sub-test
				unique := make([]search.Document, len(documents))
				for i, doc := range documents {
					unique[i] = search.NewDocument(
						fmt.Sprintf("e2e-%d-%s", count, doc.SnippetID()),
						doc.Text(),
					)
				}
				request := search.NewIndexRequest(unique)

				start := time.Now()
				err := svc.Index(ctx, request)
				elapsed := time.Since(start)
				require.NoError(t, err)

				perItem := elapsed / time.Duration(count)
				t.Logf("count=%d  total=%v  per_item=%v  items/sec=%.1f",
					count, elapsed, perItem, float64(count)/elapsed.Seconds())
			})
		}
	})

	// --- Phase 5: End-to-End Search ---
	t.Run("end_to_end_search", func(t *testing.T) {
		queries := []string{
			"user authentication login",
			"database query optimization",
			"error handling graceful shutdown",
			"unit test mock dependency injection",
			"REST API endpoint handler",
		}

		for _, query := range queries {
			t.Run(query, func(t *testing.T) {
				const iterations = 5
				var total time.Duration

				for range iterations {
					start := time.Now()
					results, err := svc.Find(ctx, query, repository.WithLimit(10))
					elapsed := time.Since(start)
					require.NoError(t, err)
					require.NotEmpty(t, results)
					total += elapsed
				}

				avg := total / time.Duration(iterations)
				t.Logf("query=%q  avg=%v  total=%v", query, avg, total)
			})
		}
	})
}

// TestEmbeddingPipelineCPUProfile generates a CPU profile of the full
// embedding pipeline. Run with:
//
//	go test -tags "fts5 ORT embed_model" -run TestEmbeddingPipelineCPUProfile -v ./test/performance/...
//
// Then analyze with:
//
//	go tool pprof test/performance/cpu.prof
func TestEmbeddingPipelineCPUProfile(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	embedder := testEmbedder(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	store, err := persistence.NewVectorChordEmbeddingStore(
		ctx, db, "perf", embeddingDimension, logger,
	)
	require.NoError(t, err)

	adapter := &embeddingAdapter{inner: embedder}
	svc, err := domainservice.NewEmbedding(store, adapter)
	require.NoError(t, err)

	// Create profile output
	profilePath := "cpu.prof"
	f, err := os.Create(profilePath)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	// Warm up the ONNX model before profiling
	warmReq := provider.NewEmbeddingRequest([]string{"warmup"})
	_, err = embedder.Embed(ctx, warmReq)
	require.NoError(t, err)

	// Start CPU profiling
	err = pprof.StartCPUProfile(f)
	require.NoError(t, err)
	defer pprof.StopCPUProfile()

	// Profile: index 200 documents (mix of inference + DB writes)
	documents := sampleCodeSnippets(200)
	request := search.NewIndexRequest(documents)
	err = svc.Index(ctx, request)
	require.NoError(t, err)

	// Profile: 50 search queries (mix of inference + DB reads)
	queries := []string{
		"authentication login handler",
		"database repository pattern",
		"kubernetes deployment config",
		"payment processing gateway",
		"test benchmark sort algorithm",
	}
	for i := 0; i < 50; i++ {
		query := queries[i%len(queries)]
		_, err := svc.Find(ctx, query, repository.WithLimit(10))
		require.NoError(t, err)
	}

	t.Logf("CPU profile written to %s", profilePath)
	t.Log("Analyze with: go tool pprof test/performance/cpu.prof")
}

// TestEmbeddingPipelineMemProfile generates a memory profile.
func TestEmbeddingPipelineMemProfile(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	embedder := testEmbedder(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	store, err := persistence.NewVectorChordEmbeddingStore(
		ctx, db, "perf", embeddingDimension, logger,
	)
	require.NoError(t, err)

	adapter := &embeddingAdapter{inner: embedder}
	svc, err := domainservice.NewEmbedding(store, adapter)
	require.NoError(t, err)

	// Warm up
	warmReq := provider.NewEmbeddingRequest([]string{"warmup"})
	_, err = embedder.Embed(ctx, warmReq)
	require.NoError(t, err)

	// Allocate/index 200 documents
	documents := sampleCodeSnippets(200)
	request := search.NewIndexRequest(documents)
	err = svc.Index(ctx, request)
	require.NoError(t, err)

	// Search 20 times
	for range 20 {
		_, err := svc.Find(ctx, "authentication handler", repository.WithLimit(10))
		require.NoError(t, err)
	}

	// Force GC and write heap profile
	runtime.GC()

	profilePath := "mem.prof"
	f, err := os.Create(profilePath)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	err = pprof.WriteHeapProfile(f)
	require.NoError(t, err)

	t.Logf("Memory profile written to %s", profilePath)
	t.Log("Analyze with: go tool pprof -alloc_space test/performance/mem.prof")
}

// TestVectorCopyOverhead measures the overhead of defensive vector copying
// in the domain layer (Embedding.Vector(), NewEmbedding, NewPgVector).
func TestVectorCopyOverhead(t *testing.T) {
	const iterations = 10000
	vec := randomVector(embeddingDimension)

	t.Run("NewEmbedding_creation", func(t *testing.T) {
		start := time.Now()
		for range iterations {
			_ = search.NewEmbedding("test", vec)
		}
		elapsed := time.Since(start)
		t.Logf("iterations=%d  total=%v  per_op=%v", iterations, elapsed, elapsed/iterations)
	})

	t.Run("Embedding_Vector_read", func(t *testing.T) {
		emb := search.NewEmbedding("test", vec)
		start := time.Now()
		for range iterations {
			_ = emb.Vector()
		}
		elapsed := time.Since(start)
		t.Logf("iterations=%d  total=%v  per_op=%v", iterations, elapsed, elapsed/iterations)
	})

	t.Run("PgVector_String_serialization", func(t *testing.T) {
		pgv := database.NewPgVector(vec)
		start := time.Now()
		for range iterations {
			_ = pgv.String()
		}
		elapsed := time.Since(start)
		t.Logf("iterations=%d  total=%v  per_op=%v", iterations, elapsed, elapsed/iterations)
	})
}

// TestSaveAllBatching measures whether SaveAll would benefit from
// batched inserts vs the current one-row-per-INSERT approach.
func TestSaveAllBatching(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	store, err := persistence.NewVectorChordEmbeddingStore(
		ctx, db, "perf", embeddingDimension, logger,
	)
	require.NoError(t, err)

	counts := []int{10, 50, 100, 200, 500}
	for _, count := range counts {
		t.Run(fmt.Sprintf("count_%d", count), func(t *testing.T) {
			embeddings := make([]search.Embedding, count)
			for i := range embeddings {
				embeddings[i] = search.NewEmbedding(
					fmt.Sprintf("batch-test-%d-%06d", count, i),
					randomVector(embeddingDimension),
				)
			}

			start := time.Now()
			err := store.SaveAll(ctx, embeddings)
			elapsed := time.Since(start)
			require.NoError(t, err)

			perItem := elapsed / time.Duration(count)
			t.Logf("count=%d  total=%v  per_item=%v  items/sec=%.1f",
				count, elapsed, perItem, float64(count)/elapsed.Seconds())
		})
	}
}
