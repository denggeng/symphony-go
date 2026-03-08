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
	if executor.tracker.Kind() != "jira" {
		return nil
	}
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
	}
}

func (executor *Executor) Execute(ctx context.Context, name string, arguments any) map[string]any {
	if executor == nil || executor.tracker == nil {
		return failureResponse(map[string]any{"error": map[string]any{"message": "No tracker is configured for dynamic tools."}})
	}
	if name != "jira_api" {
		return failureResponse(map[string]any{"error": map[string]any{"message": fmt.Sprintf("Unsupported dynamic tool: %q.", name)}})
	}
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
