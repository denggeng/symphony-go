package tracker

import (
	"context"

	"github.com/denggeng/symphony-go/internal/domain"
)

type RawRequest struct {
	Method string
	Path   string
	Query  map[string]string
	Body   any
}

type RawResponse struct {
	StatusCode int    `json:"status_code"`
	Headers    any    `json:"headers,omitempty"`
	Body       any    `json:"body,omitempty"`
	RawBody    string `json:"raw_body,omitempty"`
}

// TaskUpdate describes the final state and handoff summary for a tracked task.
type TaskUpdate struct {
	State   string `json:"state"`
	Summary string `json:"summary,omitempty"`
}

type Tracker interface {
	Kind() string
	FetchCandidateIssues(ctx context.Context) ([]domain.Issue, error)
	FetchIssuesByIDs(ctx context.Context, ids []string) ([]domain.Issue, error)
	FetchIssuesByStates(ctx context.Context, states []string) ([]domain.Issue, error)
	CreateComment(ctx context.Context, issueIdentifier string, body string) error
	UpdateIssueState(ctx context.Context, issueIdentifier string, stateName string) error
	RawAPI(ctx context.Context, request RawRequest) (RawResponse, error)
}

// TaskUpdater is implemented by trackers that can close the loop on a task.
type TaskUpdater interface {
	UpdateTask(ctx context.Context, issueIdentifier string, update TaskUpdate) error
}
