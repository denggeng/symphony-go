package prompt

import (
	"testing"

	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/workflow"
)

func TestRendererRender(t *testing.T) {
	t.Parallel()
	renderer := New(workflow.Definition{PromptTemplate: `Identifier: {{ issue.identifier }}
{% if issue.description %}{{ issue.description }}{% else %}missing{% endif %}`})
	text := renderer.Render(domain.Issue{Identifier: "ABC-1", Description: "hello"}, 1)
	if text != "Identifier: ABC-1\nhello" {
		t.Fatalf("unexpected render: %q", text)
	}
}

func TestContinuationPrompt(t *testing.T) {
	t.Parallel()
	text := ContinuationPrompt(2, 5)
	if text == "" {
		t.Fatalf("expected continuation prompt")
	}
}
