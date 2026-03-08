package prompt

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/workflow"
)

var (
	conditionalPattern = regexp.MustCompile(`(?s)\{%\s*if\s+([^%]+?)\s*%\}(.*?)(?:\{%\s*else\s*%\}(.*?))?\{%\s*endif\s*%\}`)
	variablePattern    = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)
)

type Renderer struct {
	template string
}

func New(definition workflow.Definition) *Renderer {
	template := strings.TrimSpace(definition.PromptTemplate)
	if template == "" {
		template = config.DefaultPromptTemplate()
	}
	return &Renderer{template: template}
}

func (renderer *Renderer) Render(issue domain.Issue, attempt int) string {
	context := map[string]any{
		"attempt": attempt,
		"issue": map[string]any{
			"id":          issue.ID,
			"identifier":  issue.Identifier,
			"title":       issue.Title,
			"description": issue.Description,
			"state":       issue.State,
			"url":         issue.URL,
			"branch_name": issue.BranchName,
		},
	}

	resolved := renderer.template
	for {
		next := conditionalPattern.ReplaceAllStringFunc(resolved, func(match string) string {
			parts := conditionalPattern.FindStringSubmatch(match)
			if len(parts) < 3 {
				return match
			}
			condition := strings.TrimSpace(parts[1])
			whenTrue := parts[2]
			whenFalse := ""
			if len(parts) >= 4 {
				whenFalse = parts[3]
			}
			if truthy(lookup(context, condition)) {
				return whenTrue
			}
			return whenFalse
		})
		if next == resolved {
			break
		}
		resolved = next
	}

	resolved = variablePattern.ReplaceAllStringFunc(resolved, func(match string) string {
		parts := variablePattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		value := lookup(context, strings.TrimSpace(parts[1]))
		return stringify(value)
	})

	return strings.TrimSpace(resolved)
}

func ContinuationPrompt(turnNumber int, maxTurns int) string {
	return strings.TrimSpace(fmt.Sprintf(`Continuation guidance:

- The previous Codex turn completed normally, but the issue is still active.
- This is continuation turn #%d of %d for the current run.
- Resume from the current workspace and existing thread context.
- Focus only on the remaining ticket work.
- Do not restate the full task unless you need to clarify a blocker.`, turnNumber, maxTurns))
}

func lookup(context map[string]any, path string) any {
	segments := strings.Split(strings.TrimSpace(path), ".")
	var current any = context
	for _, segment := range segments {
		mapValue, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = mapValue[segment]
	}
	return current
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case bool:
		return typed
	default:
		return true
	}
}

func stringify(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprintf("%v", typed)
	}
}
