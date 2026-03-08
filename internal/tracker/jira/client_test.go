package jira

import "testing"

func TestMapIssue(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		"id":  "10001",
		"key": "ABC-1",
		"fields": map[string]any{
			"summary": "Test issue",
			"description": map[string]any{
				"type": "doc",
				"content": []any{
					map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Hello Jira"}}},
				},
			},
			"status":   map[string]any{"name": "In Progress"},
			"priority": map[string]any{"id": "2"},
			"labels":   []any{"Bug", "Backend"},
		},
	}
	issue := mapIssue(payload)
	if issue.Identifier != "ABC-1" {
		t.Fatalf("unexpected identifier: %q", issue.Identifier)
	}
	if issue.Description == "" {
		t.Fatalf("expected description to be converted from ADF")
	}
	if issue.Priority == nil || *issue.Priority != 2 {
		t.Fatalf("unexpected priority: %#v", issue.Priority)
	}
	if len(issue.Labels) != 2 || issue.Labels[0] != "bug" {
		t.Fatalf("unexpected labels: %#v", issue.Labels)
	}
}
