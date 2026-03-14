package prompt

import (
	"strings"
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

func TestRendererRenderIncludesReviewFields(t *testing.T) {
	t.Parallel()
	renderer := New(workflow.Definition{PromptTemplate: `Lane: {{ issue.lane }}
Review: {{ issue.review_of }}`})
	text := renderer.Render(domain.Issue{Lane: "review", ReviewOf: "impl-1"}, 1)
	if text != "Lane: review\nReview: impl-1" {
		t.Fatalf("unexpected render with review fields: %q", text)
	}
}

func TestContinuationPrompt(t *testing.T) {
	t.Parallel()
	text := ContinuationPrompt(2, 5)
	if text == "" {
		t.Fatalf("expected continuation prompt")
	}
}

func TestRendererUsesDefaultPromptTemplate(t *testing.T) {
	t.Parallel()
	renderer := New(workflow.Definition{})
	text := renderer.Render(domain.Issue{Identifier: "task-1", Title: "Local task", State: "To Do", Description: "Implement it."}, 1)
	if !strings.Contains(text, "tracked task") || !strings.Contains(text, "State: To Do") {
		t.Fatalf("unexpected default prompt: %q", text)
	}
}
