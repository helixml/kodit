package enrichment

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/helixml/kodit/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTextGenerator struct {
	responses map[string]string
	calls     []provider.ChatCompletionRequest
	err       error
}

func (f *fakeTextGenerator) ChatCompletion(_ context.Context, req provider.ChatCompletionRequest) (provider.ChatCompletionResponse, error) {
	f.calls = append(f.calls, req)
	if f.err != nil {
		return provider.ChatCompletionResponse{}, f.err
	}

	messages := req.Messages()
	usage := provider.NewUsage(10, 20, 30)
	if len(messages) >= 2 {
		userMsg := messages[1].Content()
		if resp, ok := f.responses[userMsg]; ok {
			return provider.NewChatCompletionResponse(resp, "stop", usage), nil
		}
	}

	return provider.NewChatCompletionResponse("enriched content", "stop", usage), nil
}

func TestRequest(t *testing.T) {
	t.Run("creates request with all fields", func(t *testing.T) {
		req := NewRequest("id-1", "some text", "system prompt")

		assert.Equal(t, "id-1", req.ID())
		assert.Equal(t, "some text", req.Text())
		assert.Equal(t, "system prompt", req.SystemPrompt())
	})
}

func TestResponse(t *testing.T) {
	t.Run("creates response with all fields", func(t *testing.T) {
		resp := NewResponse("id-1", "enriched text")

		assert.Equal(t, "id-1", resp.ID())
		assert.Equal(t, "enriched text", resp.Text())
	})
}

func TestProviderEnricher(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("processes single request", func(t *testing.T) {
		generator := &fakeTextGenerator{
			responses: map[string]string{
				"input text": "enriched output",
			},
		}

		enricher := NewProviderEnricher(generator, logger)

		requests := []Request{
			NewRequest("req-1", "input text", "summarize this"),
		}

		responses, err := enricher.Enrich(ctx, requests)

		require.NoError(t, err)
		assert.Len(t, responses, 1)
		assert.Equal(t, "req-1", responses[0].ID())
		assert.Equal(t, "enriched output", responses[0].Text())
	})

	t.Run("processes multiple requests", func(t *testing.T) {
		generator := &fakeTextGenerator{
			responses: map[string]string{
				"text 1": "result 1",
				"text 2": "result 2",
			},
		}

		enricher := NewProviderEnricher(generator, logger)

		requests := []Request{
			NewRequest("req-1", "text 1", "prompt"),
			NewRequest("req-2", "text 2", "prompt"),
		}

		responses, err := enricher.Enrich(ctx, requests)

		require.NoError(t, err)
		assert.Len(t, responses, 2)
		assert.Equal(t, "req-1", responses[0].ID())
		assert.Equal(t, "req-2", responses[1].ID())
	})

	t.Run("filters empty text requests", func(t *testing.T) {
		generator := &fakeTextGenerator{}

		enricher := NewProviderEnricher(generator, logger)

		requests := []Request{
			NewRequest("req-1", "", "prompt"),
			NewRequest("req-2", "", "prompt"),
		}

		responses, err := enricher.Enrich(ctx, requests)

		require.NoError(t, err)
		assert.Nil(t, responses)
		assert.Len(t, generator.calls, 0)
	})

	t.Run("returns error on generator failure", func(t *testing.T) {
		generator := &fakeTextGenerator{
			err: errors.New("api error"),
		}

		enricher := NewProviderEnricher(generator, logger)

		requests := []Request{
			NewRequest("req-1", "text", "prompt"),
		}

		responses, err := enricher.Enrich(ctx, requests)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "enrich request req-1")
		assert.Len(t, responses, 0)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		generator := &fakeTextGenerator{}

		enricher := NewProviderEnricher(generator, logger)

		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		requests := []Request{
			NewRequest("req-1", "text", "prompt"),
		}

		_, err := enricher.Enrich(canceledCtx, requests)

		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
	})

	t.Run("applies custom max tokens", func(t *testing.T) {
		generator := &fakeTextGenerator{}

		enricher := NewProviderEnricher(generator, logger).WithMaxTokens(4096)

		requests := []Request{
			NewRequest("req-1", "text", "prompt"),
		}

		_, err := enricher.Enrich(ctx, requests)

		require.NoError(t, err)
		assert.Len(t, generator.calls, 1)
		assert.Equal(t, 4096, generator.calls[0].MaxTokens())
	})

	t.Run("applies custom temperature", func(t *testing.T) {
		generator := &fakeTextGenerator{}

		enricher := NewProviderEnricher(generator, logger).WithTemperature(0.5)

		requests := []Request{
			NewRequest("req-1", "text", "prompt"),
		}

		_, err := enricher.Enrich(ctx, requests)

		require.NoError(t, err)
		assert.Len(t, generator.calls, 1)
		assert.Equal(t, 0.5, generator.calls[0].Temperature())
	})
}

func TestCleanThinkingTags(t *testing.T) {
	t.Run("removes thinking tags", func(t *testing.T) {
		input := "<think>internal reasoning</think>actual response"
		result := cleanThinkingTags(input)
		assert.Equal(t, "actual response", result)
	})

	t.Run("removes multiple thinking blocks", func(t *testing.T) {
		input := "<think>first</think>middle<think>second</think>end"
		result := cleanThinkingTags(input)
		assert.Equal(t, "middleend", result)
	})

	t.Run("handles unclosed thinking tag", func(t *testing.T) {
		input := "<think>unclosed reasoning"
		result := cleanThinkingTags(input)
		assert.Equal(t, "unclosed reasoning", result)
	})

	t.Run("returns unchanged text without tags", func(t *testing.T) {
		input := "plain text without tags"
		result := cleanThinkingTags(input)
		assert.Equal(t, "plain text without tags", result)
	})

	t.Run("handles empty string", func(t *testing.T) {
		result := cleanThinkingTags("")
		assert.Equal(t, "", result)
	})
}
