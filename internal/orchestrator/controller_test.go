package orchestrator

import (
	"fmt"
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

func TestHistoryRetentionAndDetailLookup(t *testing.T) {
	t.Parallel()
	controller := New(Options{Config: config.Config{Orchestrator: config.OrchestratorConfig{PollIntervalMs: 30_000}}})
	base := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)

	controller.mu.Lock()
	for index := 0; index < maxRunHistoryEntries+5; index++ {
		runID := fmt.Sprintf("run-%03d", index)
		controller.appendHistoryLocked(RunHistorySnapshot{
			RunID:          runID,
			Identifier:     fmt.Sprintf("ABC-%d", index),
			Status:         "succeeded",
			StartedAt:      base.Add(time.Duration(index) * time.Minute),
			FinishedAt:     base.Add(time.Duration(index+1) * time.Minute),
			RuntimeSeconds: 60,
			EventCount:     1,
		}, []RunEventSnapshot{{Timestamp: base.Add(time.Duration(index) * time.Minute), Type: "session_started", Message: runID}})
	}
	controller.mu.Unlock()

	history := controller.History()
	if len(history) != maxRunHistoryEntries {
		t.Fatalf("unexpected history length: %d", len(history))
	}
	if history[0].RunID != "run-054" {
		t.Fatalf("unexpected newest run: %s", history[0].RunID)
	}
	if history[len(history)-1].RunID != "run-005" {
		t.Fatalf("unexpected oldest retained run: %s", history[len(history)-1].RunID)
	}
	if _, ok := controller.RunHistory("run-001"); ok {
		t.Fatalf("expected trimmed run to be missing")
	}

	detail, ok := controller.RunHistory("run-054")
	if !ok {
		t.Fatalf("expected retained run detail")
	}
	if len(detail.Events) != 1 {
		t.Fatalf("unexpected event count: %d", len(detail.Events))
	}
	if detail.Events[0].Message != "run-054" {
		t.Fatalf("unexpected event message: %s", detail.Events[0].Message)
	}
}
