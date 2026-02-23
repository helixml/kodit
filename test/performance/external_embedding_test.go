package performance_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/stretchr/testify/require"
)

const (
	// openRouterBaseURL is the OpenRouter API base URL.
	openRouterBaseURL = "https://openrouter.ai/api/v1"

	// openRouterEmbeddingModel is the embedding model to use via OpenRouter.
	openRouterEmbeddingModel = "openai/text-embedding-3-small"

	// openRouterTimeout is the HTTP timeout for embedding requests.
	openRouterTimeout = 60 * time.Second
)

// externalEmbedder creates an OpenAI-compatible provider pointed at OpenRouter.
// Skips the test if EMBEDDING_ENDPOINT_API_KEY is not set.
func externalEmbedder(t *testing.T) *provider.OpenAIProvider {
	t.Helper()

	apiKey := os.Getenv("EMBEDDING_ENDPOINT_API_KEY")
	if apiKey == "" {
		t.Skip("skipping: EMBEDDING_ENDPOINT_API_KEY not set")
	}

	return provider.NewOpenAIProviderFromConfig(provider.OpenAIConfig{
		APIKey:         apiKey,
		BaseURL:        openRouterBaseURL,
		EmbeddingModel: openRouterEmbeddingModel,
		Timeout:        openRouterTimeout,
		MaxRetries:     3,
		InitialDelay:   time.Second,
		BackoffFactor:  2.0,
	})
}

// TestExternalEmbeddingBatching measures single-request latency and
// latency distribution for external embedding providers.
//
// Run with:
//
//	EMBEDDING_ENDPOINT_API_KEY=sk-... go test -run TestExternalEmbeddingBatching -v ./test/performance/...
func TestExternalEmbeddingBatching(t *testing.T) {
	ctx := context.Background()
	embedder := externalEmbedder(t)
	defer func() { _ = embedder.Close() }()

	texts := sampleTexts(20)

	// Warm up: single request to establish connection and verify credentials.
	warmup := provider.NewEmbeddingRequest(texts[:1])
	resp, err := embedder.Embed(ctx, warmup)
	require.NoError(t, err)
	require.Len(t, resp.Embeddings(), 1)
	dimension := len(resp.Embeddings()[0])
	t.Logf("model=%s  dimension=%d", openRouterEmbeddingModel, dimension)

	// --- Phase 1: Sequential (one text per request) ---
	t.Run("sequential", func(t *testing.T) {
		counts := []int{1, 5, 10}
		for _, count := range counts {
			t.Run(fmt.Sprintf("n_%d", count), func(t *testing.T) {
				batch := texts[:count]

				start := time.Now()
				for _, text := range batch {
					req := provider.NewEmbeddingRequest([]string{text})
					resp, err := embedder.Embed(ctx, req)
					require.NoError(t, err)
					require.Len(t, resp.Embeddings(), 1)
				}
				elapsed := time.Since(start)

				perItem := elapsed / time.Duration(count)
				t.Logf("n=%d  total=%v  per_item=%v  items/sec=%.1f",
					count, elapsed, perItem, float64(count)/elapsed.Seconds())
			})
		}
	})

	// --- Phase 2: Latency distribution ---
	// Measures p50/p95/p99 latency for single-item requests.
	t.Run("latency_distribution", func(t *testing.T) {
		const iterations = 20
		latencies := make([]time.Duration, iterations)

		for i := range iterations {
			text := texts[i%len(texts)]
			start := time.Now()
			req := provider.NewEmbeddingRequest([]string{text})
			_, err := embedder.Embed(ctx, req)
			latencies[i] = time.Since(start)
			require.NoError(t, err)
		}

		sorted := make([]time.Duration, len(latencies))
		copy(sorted, latencies)
		sortDurations(sorted)

		var total time.Duration
		for _, d := range sorted {
			total += d
		}

		t.Logf("n=%d  avg=%v  p50=%v  p95=%v  p99=%v  min=%v  max=%v",
			iterations,
			total/time.Duration(iterations),
			sorted[iterations/2],
			sorted[iterations*95/100],
			sorted[iterations*99/100],
			sorted[0],
			sorted[iterations-1],
		)
	})
}

// sampleTexts returns n code snippet strings for embedding.
func sampleTexts(n int) []string {
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

	result := make([]string, n)
	for i := range result {
		result[i] = snippets[i%len(snippets)]
	}
	return result
}

// sortDurations sorts a slice of durations in ascending order.
func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && d[j] < d[j-1]; j-- {
			d[j], d[j-1] = d[j-1], d[j]
		}
	}
}
