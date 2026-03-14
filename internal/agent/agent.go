package agent

import (
	"context"
	"time"

	"github.com/denggeng/symphony-go/internal/domain"
)

type Usage struct {
	InputTokens           int `json:"input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	TotalTokens           int `json:"total_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens,omitempty"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens,omitempty"`
}

type Event struct {
	Timestamp         time.Time      `json:"timestamp"`
	Type              string         `json:"type"`
	SessionID         string         `json:"session_id,omitempty"`
	ThreadID          string         `json:"thread_id,omitempty"`
	TurnID            string         `json:"turn_id,omitempty"`
	CodexAppServerPID string         `json:"codex_app_server_pid,omitempty"`
	Message           string         `json:"message,omitempty"`
	Usage             Usage          `json:"usage"`
	Raw               map[string]any `json:"raw,omitempty"`
}

type EventHandler func(Event)

type Session interface{}

type TurnResult struct {
	SessionID string
	ThreadID  string
	TurnID    string
}

type Backend interface {
	StartSession(ctx context.Context, workspace string) (Session, error)
	RunTurn(ctx context.Context, session Session, issue domain.Issue, prompt string, onEvent EventHandler) (TurnResult, error)
	StopSession(ctx context.Context, session Session) error
}
