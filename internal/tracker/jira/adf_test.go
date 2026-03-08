package jira

import "testing"

func TestADFToMarkdown(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"type": "doc",
		"content": []any{
			map[string]any{
				"type":    "paragraph",
				"content": []any{map[string]any{"type": "text", "text": "Hello"}},
			},
			map[string]any{
				"type": "bulletList",
				"content": []any{
					map[string]any{"type": "listItem", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "One"}}}}},
				},
			},
		},
	}
	output := adfToMarkdown(input)
	if output == "" {
		t.Fatalf("expected markdown output")
	}
}
