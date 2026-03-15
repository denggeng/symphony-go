package orchestrator

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/runner"
	"github.com/denggeng/symphony-go/internal/workspace"
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

func TestCanDispatchRespectsLaneConcurrencyLimits(t *testing.T) {
	t.Parallel()
	controller := New(Options{Config: config.Config{Orchestrator: config.OrchestratorConfig{MaxConcurrentAgents: 2, MaxRetryBackoffMs: 300_000, ConcurrencyLimits: map[string]int{"default": 1, "review": 1}}}})
	controller.running["impl-1"] = &runningEntry{Issue: domain.Issue{ID: "impl-1", Identifier: "impl-1", Lane: "default"}}
	if controller.canDispatch(domain.Issue{ID: "impl-2", Identifier: "impl-2", Lane: "default"}) {
		t.Fatalf("expected second default-lane task to be held")
	}
	if !controller.canDispatch(domain.Issue{ID: "review-1", Identifier: "review-1", Lane: "review"}) {
		t.Fatalf("expected review lane to dispatch alongside default lane")
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

func TestBuildRunHistorySnapshotSuppressesTerminalStateCancellation(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 3, 14, 8, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Minute)
	entry := &runningEntry{
		RunID:         "run-1",
		Issue:         domain.Issue{ID: "ISSUE-1", Identifier: "ISSUE-1", Title: "slice", State: "Done"},
		Identifier:    "ISSUE-1",
		WorkspacePath: "/tmp/workspaces/ISSUE-1",
		StartedAt:     startedAt,
		StopRequested: true,
		StopReason:    "terminal_state",
	}

	history := buildRunHistorySnapshot(entry, runner.Result{}, context.Canceled, finishedAt)
	if history.Status != "stopped_terminal" {
		t.Fatalf("Status = %q, want %q", history.Status, "stopped_terminal")
	}
	if history.Error != "" {
		t.Fatalf("Error = %q, want empty", history.Error)
	}
	if history.LastMessage != "" {
		t.Fatalf("LastMessage = %q, want empty", history.LastMessage)
	}
}

func TestBuildRunHistorySnapshotKeepsActualFailure(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 3, 14, 8, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(30 * time.Second)
	entry := &runningEntry{
		RunID:         "run-2",
		Issue:         domain.Issue{ID: "ISSUE-2", Identifier: "ISSUE-2", Title: "slice", State: "In Progress"},
		Identifier:    "ISSUE-2",
		WorkspacePath: "/tmp/workspaces/ISSUE-2",
		StartedAt:     startedAt,
	}

	err := fmt.Errorf("provider failed")
	history := buildRunHistorySnapshot(entry, runner.Result{}, err, finishedAt)
	if history.Status != "failed" {
		t.Fatalf("Status = %q, want %q", history.Status, "failed")
	}
	if history.Error != err.Error() {
		t.Fatalf("Error = %q, want %q", history.Error, err.Error())
	}
	if history.LastMessage != err.Error() {
		t.Fatalf("LastMessage = %q, want %q", history.LastMessage, err.Error())
	}
}

func TestBuildRunHistorySnapshotPreservesTotalEventCountBeyondRetainedBuffer(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(45 * time.Second)
	events := make([]RunEventSnapshot, maxRunEventsPerRun)
	for i := range events {
		events[i] = RunEventSnapshot{Timestamp: startedAt.Add(time.Duration(i) * time.Millisecond), Type: "notification", Message: fmt.Sprintf("event-%03d", i)}
	}
	entry := &runningEntry{
		RunID:         "run-3",
		Issue:         domain.Issue{ID: "ISSUE-3", Identifier: "ISSUE-3", Title: "slice", State: "Done"},
		Identifier:    "ISSUE-3",
		WorkspacePath: "/tmp/workspaces/ISSUE-3",
		StartedAt:     startedAt,
		EventCount:    maxRunEventsPerRun + 37,
		Events:        events,
	}

	history := buildRunHistorySnapshot(entry, runner.Result{}, nil, finishedAt)
	if history.EventCount != maxRunEventsPerRun+37 {
		t.Fatalf("EventCount = %d, want %d", history.EventCount, maxRunEventsPerRun+37)
	}
}

func TestSnapshotRunningIssueReportsTotalEventCount(t *testing.T) {
	t.Parallel()

	controller := New(Options{Config: config.Config{Orchestrator: config.OrchestratorConfig{PollIntervalMs: 30_000}}})
	startedAt := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	events := make([]RunEventSnapshot, maxRunEventsPerRun)
	controller.mu.Lock()
	controller.running["RUN-1"] = &runningEntry{
		RunID:      "run-1",
		Issue:      domain.Issue{ID: "RUN-1", Identifier: "running", State: "In Progress"},
		Identifier: "running",
		StartedAt:  startedAt,
		EventCount: maxRunEventsPerRun + 12,
		Events:     events,
	}
	controller.mu.Unlock()

	snapshot := controller.Snapshot()
	if len(snapshot.Running) != 1 {
		t.Fatalf("unexpected running length: %d", len(snapshot.Running))
	}
	if snapshot.Running[0].EventCount != maxRunEventsPerRun+12 {
		t.Fatalf("EventCount = %d, want %d", snapshot.Running[0].EventCount, maxRunEventsPerRun+12)
	}
}

func TestSnapshotIncludesBacklogReadyAndBlockedItems(t *testing.T) {
	t.Parallel()

	controller := New(Options{Config: config.Config{Orchestrator: config.OrchestratorConfig{PollIntervalMs: 30_000}}})
	startedAt := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	readyUpdatedAt := startedAt.Add(5 * time.Minute)
	blockedUpdatedAt := startedAt.Add(6 * time.Minute)

	controller.mu.Lock()
	controller.running["RUN-1"] = &runningEntry{
		RunID:      "run-1",
		Issue:      domain.Issue{ID: "RUN-1", Identifier: "running", State: "In Progress"},
		Identifier: "running",
		StartedAt:  startedAt,
	}
	controller.retries["RETRY-1"] = &retryEntry{IssueID: "RETRY-1", Identifier: "retrying", DueAt: startedAt.Add(time.Minute)}
	controller.mu.Unlock()

	controller.updateBacklogSnapshot([]domain.Issue{
		{ID: "RUN-1", Identifier: "running", Title: "Running issue", State: "In Progress"},
		{ID: "RETRY-1", Identifier: "retrying", Title: "Retry issue", State: "To Do"},
		{ID: "READY-1", Identifier: "ready", Title: "Ready issue", State: "To Do", Priority: intPtr(1), Order: intPtr(10), UpdatedAt: &readyUpdatedAt},
		{ID: "BLOCKED-1", Identifier: "blocked", Title: "Blocked issue", State: "To Do", Priority: intPtr(1), Order: intPtr(20), Dependencies: []string{"schema"}, BlockedBy: []string{"schema"}, UpdatedAt: &blockedUpdatedAt},
	})

	snapshot := controller.Snapshot()
	if len(snapshot.Backlog) != 2 {
		t.Fatalf("unexpected backlog length: %d", len(snapshot.Backlog))
	}
	if snapshot.Backlog[0].Identifier != "ready" || snapshot.Backlog[0].QueueStatus != "ready" {
		t.Fatalf("unexpected first backlog item: %#v", snapshot.Backlog[0])
	}
	if snapshot.Backlog[1].Identifier != "blocked" || snapshot.Backlog[1].QueueStatus != "blocked" {
		t.Fatalf("unexpected second backlog item: %#v", snapshot.Backlog[1])
	}
	if got, want := snapshot.Backlog[1].BlockedBy, []string{"schema"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected blocked_by: got %v want %v", got, want)
	}
	if got, want := snapshot.Backlog[1].Dependencies, []string{"schema"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected dependencies: got %v want %v", got, want)
	}
}

func intPtr(value int) *int {
	return &value
}

func TestIssueSnapshotFindsRetryAndBacklogItems(t *testing.T) {
	t.Parallel()

	controller := New(Options{Config: config.Config{Orchestrator: config.OrchestratorConfig{PollIntervalMs: 30_000}}})
	now := time.Date(2026, 3, 14, 9, 30, 0, 0, time.UTC)
	updatedAt := now.Add(-2 * time.Minute)

	controller.mu.Lock()
	controller.retries["RETRY-1"] = &retryEntry{
		Issue:        domain.Issue{ID: "RETRY-1", Identifier: "retrying", Title: "Retrying issue", State: "In Progress", Priority: intPtr(2), Order: intPtr(5), UpdatedAt: &updatedAt},
		IssueID:      "RETRY-1",
		Identifier:   "retrying",
		Attempt:      3,
		DueAt:        now.Add(30 * time.Second),
		Error:        "transient failure",
		Continuation: false,
	}
	controller.backlog = []BacklogSnapshot{{
		IssueID:      "BLOCKED-1",
		Identifier:   "blocked",
		Title:        "Blocked issue",
		State:        "To Do",
		QueueStatus:  "blocked",
		Priority:     intPtr(1),
		Order:        intPtr(20),
		Dependencies: []string{"schema"},
		BlockedBy:    []string{"schema"},
		UpdatedAt:    &updatedAt,
	}}
	controller.mu.Unlock()

	retrySnapshot, ok := controller.IssueSnapshot("retrying")
	if !ok {
		t.Fatalf("expected retry issue snapshot")
	}
	if retrySnapshot.QueueStatus != "retrying" {
		t.Fatalf("unexpected retry queue status: %q", retrySnapshot.QueueStatus)
	}
	if retrySnapshot.RetryAttempt != 3 {
		t.Fatalf("unexpected retry attempt: %d", retrySnapshot.RetryAttempt)
	}
	if retrySnapshot.Error != "transient failure" {
		t.Fatalf("unexpected retry error: %q", retrySnapshot.Error)
	}

	blockedSnapshot, ok := controller.IssueSnapshot("blocked")
	if !ok {
		t.Fatalf("expected backlog issue snapshot")
	}
	if blockedSnapshot.QueueStatus != "blocked" {
		t.Fatalf("unexpected backlog queue status: %q", blockedSnapshot.QueueStatus)
	}
	if got, want := blockedSnapshot.BlockedBy, []string{"schema"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected blocked_by: got %v want %v", got, want)
	}
}

func TestOnRunFinishedDoesNotRetrySyncBackFailure(t *testing.T) {
	t.Parallel()

	controller := New(Options{Config: config.Config{Orchestrator: config.OrchestratorConfig{PollIntervalMs: 30_000}}})
	controller.mu.Lock()
	controller.running["ISSUE-1"] = &runningEntry{
		RunID:         "run-1",
		Issue:         domain.Issue{ID: "ISSUE-1", Identifier: "ISSUE-1", State: "Done"},
		Identifier:    "ISSUE-1",
		WorkspacePath: "/tmp/workspaces/ISSUE-1",
		RetryAttempt:  2,
	}
	controller.claimed["ISSUE-1"] = struct{}{}
	controller.mu.Unlock()

	controller.onRunFinished(domain.Issue{ID: "ISSUE-1", Identifier: "ISSUE-1"}, runner.Result{WorkspacePath: "/tmp/workspaces/ISSUE-1", Turns: 1}, fmt.Errorf("%w: copy failed", workspace.ErrWorkspaceSyncBackFailed))

	controller.mu.RLock()
	defer controller.mu.RUnlock()
	if _, ok := controller.retries["ISSUE-1"]; ok {
		t.Fatalf("expected sync-back failure not to schedule retry")
	}
	if _, ok := controller.claimed["ISSUE-1"]; ok {
		t.Fatalf("expected claim to be released after sync-back failure")
	}
}
