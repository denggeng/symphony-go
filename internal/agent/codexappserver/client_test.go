package codexappserver

import (
	"testing"

	"github.com/denggeng/symphony-go/internal/agent"
)

func TestUsageFromPayloadParsesTokenCountTotals(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"type": "event_msg",
		"payload": map[string]any{
			"type": "token_count",
			"info": map[string]any{
				"total_token_usage": map[string]any{
					"input_tokens":            float64(123),
					"cached_input_tokens":     float64(45),
					"output_tokens":           float64(67),
					"reasoning_output_tokens": float64(8),
					"total_tokens":            float64(190),
				},
				"last_token_usage": map[string]any{
					"input_tokens":  float64(3),
					"output_tokens": float64(4),
					"total_tokens":  float64(7),
				},
			},
		},
	}

	got := usageFromPayload(payload)
	want := agent.Usage{
		InputTokens:           123,
		OutputTokens:          67,
		TotalTokens:           190,
		CachedInputTokens:     45,
		ReasoningOutputTokens: 8,
	}
	if got != want {
		t.Fatalf("usageFromPayload() = %#v, want %#v", got, want)
	}
}

func TestUsageFromPayloadParsesNestedUsageField(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"params": map[string]any{
			"usage": map[string]any{
				"input_tokens":  float64(10),
				"output_tokens": float64(5),
			},
		},
	}

	got := usageFromPayload(payload)
	want := agent.Usage{
		InputTokens:  10,
		OutputTokens: 5,
		TotalTokens:  15,
	}
	if got != want {
		t.Fatalf("usageFromPayload() = %#v, want %#v", got, want)
	}
}
