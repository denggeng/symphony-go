package tools

import (
	"context"
	"testing"

	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/tracker"
)

type fakeTracker struct{}

func (fakeTracker) Kind() string                                                 { return "jira" }
func (fakeTracker) FetchCandidateIssues(context.Context) ([]domain.Issue, error) { return nil, nil }
func (fakeTracker) FetchIssuesByIDs(context.Context, []string) ([]domain.Issue, error) {
	return nil, nil
}
func (fakeTracker) FetchIssuesByStates(context.Context, []string) ([]domain.Issue, error) {
	return nil, nil
}
func (fakeTracker) CreateComment(context.Context, string, string) error    { return nil }
func (fakeTracker) UpdateIssueState(context.Context, string, string) error { return nil }
func (fakeTracker) RawAPI(_ context.Context, request tracker.RawRequest) (tracker.RawResponse, error) {
	return tracker.RawResponse{StatusCode: 200, Body: map[string]any{"method": request.Method, "path": request.Path}}, nil
}

func TestExecutorExecuteJiraAPI(t *testing.T) {
	t.Parallel()
	executor := New(fakeTracker{})
	result := executor.Execute(context.Background(), "jira_api", map[string]any{"method": "GET", "path": "/rest/api/3/issue/ABC-1"})
	if success, _ := result["success"].(bool); !success {
		t.Fatalf("expected success result: %#v", result)
	}
}
