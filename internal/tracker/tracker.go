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

type Tracker interface {
	Kind() string
	FetchCandidateIssues(ctx context.Context) ([]domain.Issue, error)
	FetchIssuesByIDs(ctx context.Context, ids []string) ([]domain.Issue, error)
	FetchIssuesByStates(ctx context.Context, states []string) ([]domain.Issue, error)
	CreateComment(ctx context.Context, issueIdentifier string, body string) error
	UpdateIssueState(ctx context.Context, issueIdentifier string, stateName string) error
	RawAPI(ctx context.Context, request RawRequest) (RawResponse, error)
}
