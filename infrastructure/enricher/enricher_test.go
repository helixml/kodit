package enricher

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTextGenerator implements provider.TextGenerator for tests.
type fakeTextGenerator struct {
	// failAt is the set of request indices (0-based, in call order) that
	// should return an error. Use -1 or leave empty for no failures.
	failAt map[int]struct{}
	calls  int32
}

func (f *fakeTextGenerator) ChatCompletion(_ context.Context, req provider.ChatCompletionRequest) (provider.ChatCompletionResponse, error) {
	idx := int(atomic.AddInt32(&f.calls, 1)) - 1
	if _, ok := f.failAt[idx]; ok {
		return provider.ChatCompletionResponse{}, fmt.Errorf("upstream error at index %d", idx)
	}
	msgs := req.Messages()
	text := "response"
	if len(msgs) > 1 {
		text = "response for " + msgs[1].Content()
	}
	return provider.NewChatCompletionResponse(text, "stop", provider.NewUsage(0, 0, 0)), nil
}

func newRequests(n int) []domainservice.EnrichmentRequest {
	requests := make([]domainservice.EnrichmentRequest, n)
	for i := range requests {
		id := fmt.Sprintf("req-%d", i)
		requests[i] = domainservice.NewEnrichmentRequest(id, fmt.Sprintf("text %d", i), "system prompt")
	}
	return requests
}

func TestProviderEnricher_Enrich_EmptyRequests(t *testing.T) {
	gen := &fakeTextGenerator{}
	e := NewProviderEnricher(gen)

	responses, err := e.Enrich(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, responses)
}

func TestProviderEnricher_Enrich_AllSucceed(t *testing.T) {
	gen := &fakeTextGenerator{}
	e := NewProviderEnricher(gen)

	responses, err := e.Enrich(context.Background(), newRequests(3))
	require.NoError(t, err)
	assert.Len(t, responses, 3)

	ids := make(map[string]bool)
	for _, r := range responses {
		ids[r.ID()] = true
		assert.NotEmpty(t, r.Text())
	}
	assert.True(t, ids["req-0"])
	assert.True(t, ids["req-1"])
	assert.True(t, ids["req-2"])
}

func TestProviderEnricher_Enrich_ErrorMidRequest(t *testing.T) {
	gen := &fakeTextGenerator{failAt: map[int]struct{}{1: {}}}
	e := NewProviderEnricher(gen)

	var errorCallbackIDs []string
	responses, err := e.Enrich(context.Background(), newRequests(3),
		domainservice.WithMaxFailureRate(0),
		domainservice.WithRequestError(func(requestID string, _ error) {
			errorCallbackIDs = append(errorCallbackIDs, requestID)
		}),
	)
	require.Error(t, err)
	assert.Nil(t, responses)
	assert.Contains(t, err.Error(), "enrichment requests failed")
	assert.Len(t, errorCallbackIDs, 1)
}

func TestProviderEnricher_Enrich_ToleratesPartialFailure(t *testing.T) {
	gen := &fakeTextGenerator{failAt: map[int]struct{}{1: {}}}
	e := NewProviderEnricher(gen)

	responses, err := e.Enrich(context.Background(), newRequests(3),
		domainservice.WithMaxFailureRate(0.5),
	)
	require.NoError(t, err)
	assert.Len(t, responses, 2)
}

func TestProviderEnricher_Enrich_ExceedsFailureTolerance(t *testing.T) {
	gen := &fakeTextGenerator{failAt: map[int]struct{}{0: {}, 1: {}}}
	e := NewProviderEnricher(gen)

	_, err := e.Enrich(context.Background(), newRequests(3),
		domainservice.WithMaxFailureRate(0.1),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 of 3 enrichment requests failed")
}

func TestProviderEnricher_Enrich_ProgressCallback(t *testing.T) {
	gen := &fakeTextGenerator{}
	e := NewProviderEnricher(gen)

	var progressCalls int32
	_, err := e.Enrich(context.Background(), newRequests(3),
		domainservice.WithEnrichProgress(func(completed, total int) {
			atomic.AddInt32(&progressCalls, 1)
			assert.Equal(t, 3, total)
			assert.True(t, completed >= 1 && completed <= 3)
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(3), progressCalls)
}

func TestProviderEnricher_Enrich_Parallel(t *testing.T) {
	gen := &fakeTextGenerator{}
	e := NewProviderEnricher(gen).WithParallelism(3)

	responses, err := e.Enrich(context.Background(), newRequests(6))
	require.NoError(t, err)
	assert.Len(t, responses, 6)

	ids := make(map[string]bool)
	for _, r := range responses {
		ids[r.ID()] = true
	}
	for i := 0; i < 6; i++ {
		assert.True(t, ids[fmt.Sprintf("req-%d", i)])
	}
}

func TestProviderEnricher_Enrich_FiltersEmptyText(t *testing.T) {
	gen := &fakeTextGenerator{}
	e := NewProviderEnricher(gen)

	requests := []domainservice.EnrichmentRequest{
		domainservice.NewEnrichmentRequest("r1", "text", "sys"),
		domainservice.NewEnrichmentRequest("r2", "", "sys"),
		domainservice.NewEnrichmentRequest("r3", "text", "sys"),
	}

	responses, err := e.Enrich(context.Background(), requests)
	require.NoError(t, err)
	assert.Len(t, responses, 2)
}

func TestProviderEnricher_Enrich_ContextCancelled(t *testing.T) {
	gen := &fakeTextGenerator{}
	e := NewProviderEnricher(gen)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	responses, err := e.Enrich(ctx, newRequests(3))
	require.NoError(t, err)
	// With context cancelled before goroutines launch, we may get 0 responses.
	assert.True(t, len(responses) <= 3)
}
