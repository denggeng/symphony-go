package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/tracker"
)

type fakeTracker struct {
	commentIssue string
	commentBody  string
	stateIssue   string
	stateName    string
}

type fakeLocalTracker struct {
	updatedIssue   string
	updatedState   string
	updatedSummary string
}

func (fakeTracker) Kind() string                                                 { return "jira" }
func (fakeTracker) FetchCandidateIssues(context.Context) ([]domain.Issue, error) { return nil, nil }
func (fakeTracker) FetchIssuesByIDs(context.Context, []string) ([]domain.Issue, error) {
	return nil, nil
}
func (fakeTracker) FetchIssuesByStates(context.Context, []string) ([]domain.Issue, error) {
	return nil, nil
}
func (tracker *fakeTracker) CreateComment(_ context.Context, issue string, body string) error {
	tracker.commentIssue = issue
	tracker.commentBody = body
	return nil
}
func (tracker *fakeTracker) UpdateIssueState(_ context.Context, issue string, state string) error {
	tracker.stateIssue = issue
	tracker.stateName = state
	return nil
}
func (fakeTracker) RawAPI(_ context.Context, request tracker.RawRequest) (tracker.RawResponse, error) {
	return tracker.RawResponse{StatusCode: 200, Body: map[string]any{"method": request.Method, "path": request.Path}}, nil
}

func (fakeLocalTracker) Kind() string { return "local" }
func (fakeLocalTracker) FetchCandidateIssues(context.Context) ([]domain.Issue, error) {
	return nil, nil
}
func (fakeLocalTracker) FetchIssuesByIDs(context.Context, []string) ([]domain.Issue, error) {
	return nil, nil
}
func (fakeLocalTracker) FetchIssuesByStates(context.Context, []string) ([]domain.Issue, error) {
	return nil, nil
}
func (fakeLocalTracker) CreateComment(context.Context, string, string) error    { return nil }
func (fakeLocalTracker) UpdateIssueState(context.Context, string, string) error { return nil }
func (tracker *fakeLocalTracker) UpdateTask(_ context.Context, issue string, update tracker.TaskUpdate) error {
	tracker.updatedIssue = issue
	tracker.updatedState = update.State
	tracker.updatedSummary = update.Summary
	return nil
}
func (fakeLocalTracker) RawAPI(context.Context, tracker.RawRequest) (tracker.RawResponse, error) {
	return tracker.RawResponse{}, nil
}

func TestExecutorExecuteJiraAPI(t *testing.T) {
	t.Parallel()
	executor := New(&fakeTracker{})
	result := executor.Execute(context.Background(), "jira_api", map[string]any{"method": "GET", "path": "/rest/api/3/issue/ABC-1"})
	if success, _ := result["success"].(bool); !success {
		t.Fatalf("expected success result: %#v", result)
	}
}

func TestExecutorExecuteJiraComment(t *testing.T) {
	t.Parallel()
	tracker := &fakeTracker{}
	executor := New(tracker)
	result := executor.Execute(context.Background(), "jira_comment", map[string]any{"issue": "ABC-1", "body": "done"})
	if success, _ := result["success"].(bool); !success {
		t.Fatalf("expected success result: %#v", result)
	}
	if tracker.commentIssue != "ABC-1" || tracker.commentBody != "done" {
		t.Fatalf("unexpected tracker comment call: %#v", tracker)
	}
}

func TestExecutorExecuteJiraTransition(t *testing.T) {
	t.Parallel()
	tracker := &fakeTracker{}
	executor := New(tracker)
	result := executor.Execute(context.Background(), "jira_transition", map[string]any{"issue": "ABC-1", "state": "Done"})
	if success, _ := result["success"].(bool); !success {
		t.Fatalf("expected success result: %#v", result)
	}
	if tracker.stateIssue != "ABC-1" || tracker.stateName != "Done" {
		t.Fatalf("unexpected tracker transition call: %#v", tracker)
	}
}

func TestExecutorRejectsIncompleteJiraComment(t *testing.T) {
	t.Parallel()
	executor := New(&fakeTracker{})
	result := executor.Execute(context.Background(), "jira_comment", map[string]any{"issue": "ABC-1"})
	if success, _ := result["success"].(bool); success {
		t.Fatalf("expected failure result: %#v", result)
	}
	items, _ := result["contentItems"].([]map[string]any)
	if len(items) == 0 || !strings.Contains(items[0]["text"].(string), "jira_comment") {
		t.Fatalf("expected jira_comment validation message: %#v", result)
	}
}

func TestExecutorExecuteTaskUpdate(t *testing.T) {
	t.Parallel()
	tracker := &fakeLocalTracker{}
	executor := New(tracker)
	result := executor.Execute(context.Background(), "task_update", map[string]any{"issue": "hello-endpoint", "state": "Done", "summary": "implemented and tested"})
	if success, _ := result["success"].(bool); !success {
		t.Fatalf("expected success result: %#v", result)
	}
	if tracker.updatedIssue != "hello-endpoint" || tracker.updatedState != "Done" || tracker.updatedSummary != "implemented and tested" {
		t.Fatalf("unexpected task update call: %#v", tracker)
	}
}

func TestExecutorRejectsIncompleteTaskUpdate(t *testing.T) {
	t.Parallel()
	executor := New(&fakeLocalTracker{})
	result := executor.Execute(context.Background(), "task_update", map[string]any{"issue": "hello-endpoint", "state": "Done"})
	if success, _ := result["success"].(bool); success {
		t.Fatalf("expected failure result: %#v", result)
	}
	items, _ := result["contentItems"].([]map[string]any)
	if len(items) == 0 || !strings.Contains(items[0]["text"].(string), "task_update") {
		t.Fatalf("expected task_update validation message: %#v", result)
	}
}
