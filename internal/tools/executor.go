package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/denggeng/symphony-go/internal/tracker"
)

type Executor struct {
	tracker tracker.Tracker
}

func New(tr tracker.Tracker) *Executor {
	return &Executor{tracker: tr}
}

func (executor *Executor) ToolSpecs() []map[string]any {
	if executor == nil || executor.tracker == nil {
		return nil
	}
	switch executor.tracker.Kind() {
	case "jira":
		return jiraToolSpecs()
	case "local":
		if _, ok := executor.tracker.(tracker.TaskUpdater); ok {
			return localToolSpecs()
		}
	}
	return nil
}

func (executor *Executor) Execute(ctx context.Context, name string, arguments any) map[string]any {
	if executor == nil || executor.tracker == nil {
		return failureResponse(map[string]any{"error": map[string]any{"message": "No tracker is configured for dynamic tools."}})
	}

	switch name {
	case "jira_api":
		request, err := normalizeJiraAPI(arguments)
		if err != nil {
			return failureResponse(map[string]any{"error": map[string]any{"message": err.Error()}})
		}
		response, err := executor.tracker.RawAPI(ctx, request)
		if err != nil {
			return failureResponse(map[string]any{"error": map[string]any{"message": err.Error()}})
		}
		return successResponse(map[string]any{
			"status_code": response.StatusCode,
			"headers":     response.Headers,
			"body":        response.Body,
			"raw_body":    response.RawBody,
		})
	case "jira_comment":
		issue, body, err := normalizeIssueText(arguments, "jira_comment", "body")
		if err != nil {
			return failureResponse(map[string]any{"error": map[string]any{"message": err.Error()}})
		}
		if err := executor.tracker.CreateComment(ctx, issue, body); err != nil {
			return failureResponse(map[string]any{"error": map[string]any{"message": err.Error()}})
		}
		return successResponse(map[string]any{"issue": issue, "commented": true})
	case "jira_transition":
		issue, state, err := normalizeIssueText(arguments, "jira_transition", "state")
		if err != nil {
			return failureResponse(map[string]any{"error": map[string]any{"message": err.Error()}})
		}
		if err := executor.tracker.UpdateIssueState(ctx, issue, state); err != nil {
			return failureResponse(map[string]any{"error": map[string]any{"message": err.Error()}})
		}
		return successResponse(map[string]any{"issue": issue, "state": state, "transitioned": true})
	case "task_update":
		updater, ok := executor.tracker.(tracker.TaskUpdater)
		if !ok {
			return failureResponse(map[string]any{"error": map[string]any{"message": "task_update is not supported for the current tracker."}})
		}
		issue, state, summary, err := normalizeTaskUpdate(arguments)
		if err != nil {
			return failureResponse(map[string]any{"error": map[string]any{"message": err.Error()}})
		}
		if err := updater.UpdateTask(ctx, issue, tracker.TaskUpdate{State: state, Summary: summary}); err != nil {
			return failureResponse(map[string]any{"error": map[string]any{"message": err.Error()}})
		}
		return successResponse(map[string]any{"issue": issue, "state": state, "updated": true})
	default:
		return failureResponse(map[string]any{"error": map[string]any{"message": fmt.Sprintf("Unsupported dynamic tool: %q.", name)}})
	}
}

func jiraToolSpecs() []map[string]any {
	return []map[string]any{
		{
			"name":        "jira_api",
			"description": "Execute a raw Jira Cloud REST API request using Symphony's configured Jira auth.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"method", "path"},
				"properties": map[string]any{
					"method": map[string]any{"type": "string", "description": "HTTP method such as GET, POST, PUT, or DELETE."},
					"path":   map[string]any{"type": "string", "description": "Jira API path under /rest/api/3/."},
					"query":  map[string]any{"type": "object", "additionalProperties": true, "description": "Optional query string parameters."},
					"body":   map[string]any{"description": "Optional JSON request body."},
				},
			},
		},
		{
			"name":        "jira_comment",
			"description": "Create a plain-text Jira comment on an issue using Symphony's configured Jira auth.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"issue", "body"},
				"properties": map[string]any{
					"issue": map[string]any{"type": "string", "description": "Jira issue key or id, such as ABC-123."},
					"body":  map[string]any{"type": "string", "description": "Plain-text comment body."},
				},
			},
		},
		{
			"name":        "jira_transition",
			"description": "Transition a Jira issue to a target state by name.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"issue", "state"},
				"properties": map[string]any{
					"issue": map[string]any{"type": "string", "description": "Jira issue key or id, such as ABC-123."},
					"state": map[string]any{"type": "string", "description": "Target Jira state name, such as In Review or Done."},
				},
			},
		},
	}
}

func localToolSpecs() []map[string]any {
	return []map[string]any{
		{
			"name":        "task_update",
			"description": "Update a local Markdown task state and write a concise result summary.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"issue", "state", "summary"},
				"properties": map[string]any{
					"issue":   map[string]any{"type": "string", "description": "Local task id, usually the Markdown filename without .md."},
					"state":   map[string]any{"type": "string", "description": "Target task state such as Done, Blocked, or In Progress."},
					"summary": map[string]any{"type": "string", "description": "Concise handoff describing changes, validation, and blockers if any."},
				},
			},
		},
	}
}

func normalizeJiraAPI(arguments any) (tracker.RawRequest, error) {
	if arguments == nil {
		return tracker.RawRequest{}, fmt.Errorf("jira_api expects an object with method and path")
	}
	args, ok := arguments.(map[string]any)
	if !ok {
		return tracker.RawRequest{}, fmt.Errorf("jira_api expects a JSON object argument")
	}
	method, _ := args["method"].(string)
	path, _ := args["path"].(string)
	if strings.TrimSpace(method) == "" || strings.TrimSpace(path) == "" {
		return tracker.RawRequest{}, fmt.Errorf("jira_api requires non-empty method and path")
	}
	query := map[string]string{}
	if rawQuery, ok := args["query"].(map[string]any); ok {
		for key, value := range rawQuery {
			query[key] = fmt.Sprintf("%v", value)
		}
	}
	return tracker.RawRequest{Method: method, Path: path, Query: query, Body: args["body"]}, nil
}

func normalizeIssueText(arguments any, toolName string, field string) (string, string, error) {
	if arguments == nil {
		return "", "", fmt.Errorf("%s expects an object argument", toolName)
	}
	args, ok := arguments.(map[string]any)
	if !ok {
		return "", "", fmt.Errorf("%s expects a JSON object argument", toolName)
	}
	issue, _ := args["issue"].(string)
	value, _ := args[field].(string)
	if strings.TrimSpace(issue) == "" || strings.TrimSpace(value) == "" {
		return "", "", fmt.Errorf("%s requires non-empty issue and %s", toolName, field)
	}
	return strings.TrimSpace(issue), strings.TrimSpace(value), nil
}

func normalizeTaskUpdate(arguments any) (string, string, string, error) {
	if arguments == nil {
		return "", "", "", fmt.Errorf("task_update expects a JSON object argument")
	}
	args, ok := arguments.(map[string]any)
	if !ok {
		return "", "", "", fmt.Errorf("task_update expects a JSON object argument")
	}
	issue, _ := args["issue"].(string)
	state, _ := args["state"].(string)
	summary, _ := args["summary"].(string)
	if strings.TrimSpace(issue) == "" || strings.TrimSpace(state) == "" || strings.TrimSpace(summary) == "" {
		return "", "", "", fmt.Errorf("task_update requires non-empty issue, state, and summary")
	}
	return strings.TrimSpace(issue), strings.TrimSpace(state), strings.TrimSpace(summary), nil
}

func successResponse(payload any) map[string]any {
	return map[string]any{
		"success": true,
		"contentItems": []map[string]any{
			{
				"type": "inputText",
				"text": encodePayload(payload),
			},
		},
	}
}

func failureResponse(payload any) map[string]any {
	return map[string]any{
		"success": false,
		"contentItems": []map[string]any{
			{
				"type": "inputText",
				"text": encodePayload(payload),
			},
		},
	}
}

func encodePayload(payload any) string {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", payload)
	}
	return string(encoded)
}
