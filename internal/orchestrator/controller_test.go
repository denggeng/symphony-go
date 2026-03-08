package orchestrator

import (
	"testing"
	"time"

	"github.com/denggeng/symphony-go/internal/config"
)

func TestRetryDelayCapsAtConfiguredMax(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Orchestrator: config.OrchestratorConfig{MaxRetryBackoffMs: 15_000}}
	delay := retryDelay(cfg, 10, false)
	if delay != 15*time.Second {
		t.Fatalf("unexpected capped delay: %v", delay)
	}
}

func TestWorkspacePathForIssueSanitizesIdentifier(t *testing.T) {
	t.Parallel()
	path := workspacePathForIssue("/tmp/workspaces", "ABC/123")
	if path != "/tmp/workspaces/ABC_123" {
		t.Fatalf("unexpected workspace path: %s", path)
	}
}
