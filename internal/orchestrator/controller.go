package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/denggeng/symphony-go/internal/agent"
	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/runner"
	"github.com/denggeng/symphony-go/internal/tracker"
	"github.com/denggeng/symphony-go/internal/workflow"
	"github.com/denggeng/symphony-go/internal/workspace"
)

const continuationRetryDelay = 1 * time.Second
const failureRetryBaseDelay = 10 * time.Second

type Options struct {
	Logger    *slog.Logger
	Workflow  workflow.Definition
	Config    config.Config
	Tracker   tracker.Tracker
	Workspace *workspace.Manager
	Runner    *runner.Runner
}

type Controller struct {
	logger    *slog.Logger
	workflow  workflow.Definition
	cfg       config.Config
	tracker   tracker.Tracker
	workspace *workspace.Manager
	runner    *runner.Runner
	startedAt time.Time

	mu             sync.RWMutex
	pollInProgress bool
	refreshQueued  bool
	lastPollAt     *time.Time
	nextPollAt     *time.Time
	lastError      string
	running        map[string]*runningEntry
	claimed        map[string]struct{}
	retries        map[string]*retryEntry
	cancel         context.CancelFunc
	started        bool
}

type runningEntry struct {
	Issue         domain.Issue
	Identifier    string
	WorkspacePath string
	StartedAt     time.Time
	RetryAttempt  int
	Cancel        context.CancelFunc
	StopRequested bool
	StopReason    string
	SessionID     string
	CodexPID      string
	Turns         int
	LastEvent     string
	LastMessage   string
	LastTimestamp *time.Time
	Usage         agent.Usage
}

type retryEntry struct {
	IssueID      string
	Identifier   string
	Attempt      int
	DueAt        time.Time
	Error        string
	Timer        *time.Timer
	Continuation bool
}

type Snapshot struct {
	Service  ServiceSnapshot    `json:"service"`
	Workflow WorkflowSnapshot   `json:"workflow"`
	Config   config.Summary     `json:"config"`
	Polling  PollingSnapshot    `json:"polling"`
	Running  []RunningSnapshot  `json:"running"`
	Retrying []RetryingSnapshot `json:"retrying"`
}

type ServiceSnapshot struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"started_at"`
	Uptime    string    `json:"uptime"`
}

type WorkflowSnapshot struct {
	Path           string    `json:"path"`
	LoadedAt       time.Time `json:"loaded_at"`
	HasFrontMatter bool      `json:"has_front_matter"`
	PromptLength   int       `json:"prompt_length"`
}

type PollingSnapshot struct {
	Checking       bool       `json:"checking"`
	PollIntervalMs int        `json:"poll_interval_ms"`
	NextPollAt     *time.Time `json:"next_poll_at,omitempty"`
	LastPollAt     *time.Time `json:"last_poll_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
}

type RunningSnapshot struct {
	IssueID        string      `json:"issue_id"`
	Identifier     string      `json:"identifier"`
	State          string      `json:"state"`
	WorkspacePath  string      `json:"workspace_path"`
	SessionID      string      `json:"session_id,omitempty"`
	CodexPID       string      `json:"codex_app_server_pid,omitempty"`
	Turns          int         `json:"turns"`
	RetryAttempt   int         `json:"retry_attempt"`
	StartedAt      time.Time   `json:"started_at"`
	RuntimeSeconds int         `json:"runtime_seconds"`
	LastEvent      string      `json:"last_event,omitempty"`
	LastMessage    string      `json:"last_message,omitempty"`
	LastTimestamp  *time.Time  `json:"last_timestamp,omitempty"`
	Usage          agent.Usage `json:"usage"`
	StopRequested  bool        `json:"stop_requested"`
	StopReason     string      `json:"stop_reason,omitempty"`
}

type RetryingSnapshot struct {
	IssueID      string    `json:"issue_id"`
	Identifier   string    `json:"identifier"`
	Attempt      int       `json:"attempt"`
	DueAt        time.Time `json:"due_at"`
	DueInMs      int64     `json:"due_in_ms"`
	Error        string    `json:"error,omitempty"`
	Continuation bool      `json:"continuation"`
}

type RefreshResponse struct {
	Queued      bool      `json:"queued"`
	Coalesced   bool      `json:"coalesced"`
	RequestedAt time.Time `json:"requested_at"`
	Operations  []string  `json:"operations"`
}

func New(opts Options) *Controller {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := time.Now().UTC()
	return &Controller{
		logger:    logger,
		workflow:  opts.Workflow,
		cfg:       opts.Config,
		tracker:   opts.Tracker,
		workspace: opts.Workspace,
		runner:    opts.Runner,
		startedAt: now,
		running:   map[string]*runningEntry{},
		claimed:   map[string]struct{}{},
		retries:   map[string]*retryEntry{},
	}
}

func (controller *Controller) Start(ctx context.Context) {
	controller.mu.Lock()
	if controller.started {
		controller.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	controller.cancel = cancel
	controller.started = true
	controller.mu.Unlock()

	go controller.cleanupStartup(runCtx)
	controller.triggerPoll("startup")

	interval := time.Duration(controller.cfg.Orchestrator.PollIntervalMs) * time.Millisecond
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				controller.triggerPoll("interval")
			}
		}
	}()
}

func (controller *Controller) Stop() {
	controller.mu.Lock()
	cancel := controller.cancel
	for _, entry := range controller.running {
		entry.StopRequested = true
		entry.StopReason = "shutdown"
		entry.Cancel()
	}
	controller.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (controller *Controller) RequestRefresh() RefreshResponse {
	requestedAt := time.Now().UTC()
	queued := controller.triggerPoll("manual_refresh")
	return RefreshResponse{
		Queued:      true,
		Coalesced:   !queued,
		RequestedAt: requestedAt,
		Operations:  []string{"poll", "reconcile"},
	}
}

func (controller *Controller) HandleJiraWebhook(secret string) (RefreshResponse, error) {
	configuredSecret := controller.cfg.Tracker.WebhookSecret
	if configuredSecret != "" && secret != configuredSecret {
		return RefreshResponse{}, fmt.Errorf("invalid webhook secret")
	}
	return controller.RequestRefresh(), nil
}

func (controller *Controller) Snapshot() Snapshot {
	controller.mu.RLock()
	defer controller.mu.RUnlock()
	now := time.Now().UTC()
	running := make([]RunningSnapshot, 0, len(controller.running))
	for issueID, entry := range controller.running {
		runtimeSeconds := int(now.Sub(entry.StartedAt).Seconds())
		running = append(running, RunningSnapshot{
			IssueID:        issueID,
			Identifier:     entry.Identifier,
			State:          entry.Issue.State,
			WorkspacePath:  entry.WorkspacePath,
			SessionID:      entry.SessionID,
			CodexPID:       entry.CodexPID,
			Turns:          entry.Turns,
			RetryAttempt:   entry.RetryAttempt,
			StartedAt:      entry.StartedAt,
			RuntimeSeconds: runtimeSeconds,
			LastEvent:      entry.LastEvent,
			LastMessage:    entry.LastMessage,
			LastTimestamp:  entry.LastTimestamp,
			Usage:          entry.Usage,
			StopRequested:  entry.StopRequested,
			StopReason:     entry.StopReason,
		})
	}
	retrying := make([]RetryingSnapshot, 0, len(controller.retries))
	for _, entry := range controller.retries {
		retrying = append(retrying, RetryingSnapshot{
			IssueID:      entry.IssueID,
			Identifier:   entry.Identifier,
			Attempt:      entry.Attempt,
			DueAt:        entry.DueAt,
			DueInMs:      maxInt64(0, entry.DueAt.Sub(now).Milliseconds()),
			Error:        entry.Error,
			Continuation: entry.Continuation,
		})
	}
	return Snapshot{
		Service:  ServiceSnapshot{Name: "symphony-go", Version: "dev", StartedAt: controller.startedAt, Uptime: now.Sub(controller.startedAt).Round(time.Second).String()},
		Workflow: WorkflowSnapshot{Path: controller.workflow.Path, LoadedAt: controller.startedAt, HasFrontMatter: controller.workflow.HasFrontMatter, PromptLength: len(controller.workflow.Prompt)},
		Config:   controller.cfg.Summary(),
		Polling:  PollingSnapshot{Checking: controller.pollInProgress, PollIntervalMs: controller.cfg.Orchestrator.PollIntervalMs, NextPollAt: controller.nextPollAt, LastPollAt: controller.lastPollAt, LastError: controller.lastError},
		Running:  running,
		Retrying: retrying,
	}
}

func (controller *Controller) IssueSnapshot(identifier string) (RunningSnapshot, bool) {
	controller.mu.RLock()
	defer controller.mu.RUnlock()
	for issueID, entry := range controller.running {
		if entry.Identifier == identifier || issueID == identifier {
			now := time.Now().UTC()
			return RunningSnapshot{
				IssueID:        issueID,
				Identifier:     entry.Identifier,
				State:          entry.Issue.State,
				WorkspacePath:  entry.WorkspacePath,
				SessionID:      entry.SessionID,
				CodexPID:       entry.CodexPID,
				Turns:          entry.Turns,
				RetryAttempt:   entry.RetryAttempt,
				StartedAt:      entry.StartedAt,
				RuntimeSeconds: int(now.Sub(entry.StartedAt).Seconds()),
				LastEvent:      entry.LastEvent,
				LastMessage:    entry.LastMessage,
				LastTimestamp:  entry.LastTimestamp,
				Usage:          entry.Usage,
				StopRequested:  entry.StopRequested,
				StopReason:     entry.StopReason,
			}, true
		}
	}
	return RunningSnapshot{}, false
}

func (controller *Controller) triggerPoll(reason string) bool {
	controller.mu.Lock()
	if controller.pollInProgress {
		controller.refreshQueued = true
		controller.mu.Unlock()
		return false
	}
	controller.pollInProgress = true
	controller.mu.Unlock()
	go controller.runPollCycle(reason)
	return true
}

func (controller *Controller) runPollCycle(reason string) {
	now := time.Now().UTC()
	controller.mu.Lock()
	controller.lastPollAt = &now
	controller.lastError = ""
	controller.mu.Unlock()

	ctx := context.Background()
	controller.logger.Info("starting poll cycle", slog.String("reason", reason))

	if err := controller.reconcileRunningIssues(ctx); err != nil {
		controller.setLastError(err)
	}
	issues, err := controller.tracker.FetchCandidateIssues(ctx)
	if err != nil {
		controller.setLastError(err)
	} else {
		domain.SortIssues(issues)
		controller.dispatchIssues(ctx, issues)
	}

	controller.mu.Lock()
	controller.pollInProgress = false
	queued := controller.refreshQueued
	controller.refreshQueued = false
	next := time.Now().UTC().Add(time.Duration(controller.cfg.Orchestrator.PollIntervalMs) * time.Millisecond)
	controller.nextPollAt = &next
	controller.mu.Unlock()

	if queued {
		controller.triggerPoll("queued_refresh")
	}
}

func (controller *Controller) cleanupStartup(ctx context.Context) {
	issues, err := controller.tracker.FetchIssuesByStates(ctx, controller.cfg.Tracker.TerminalStates)
	if err != nil {
		controller.logger.Warn("startup cleanup skipped", slog.Any("error", err))
		return
	}
	for _, issue := range issues {
		if err := controller.workspace.RemoveIssueWorkspaces(ctx, issue.Identifier); err != nil {
			controller.logger.Warn("failed to remove terminal workspace", slog.String("issue_identifier", issue.Identifier), slog.Any("error", err))
		}
	}
}

func (controller *Controller) reconcileRunningIssues(ctx context.Context) error {
	controller.mu.RLock()
	ids := make([]string, 0, len(controller.running))
	for issueID := range controller.running {
		ids = append(ids, issueID)
	}
	controller.mu.RUnlock()
	if len(ids) == 0 {
		return nil
	}
	issues, err := controller.tracker.FetchIssuesByIDs(ctx, ids)
	if err != nil {
		return err
	}
	stateByID := map[string]domain.Issue{}
	for _, issue := range issues {
		stateByID[issue.ID] = issue
	}
	controller.mu.Lock()
	defer controller.mu.Unlock()
	for issueID, entry := range controller.running {
		issue, ok := stateByID[issueID]
		if ok {
			entry.Issue = issue
		}
		if ok && controller.cfg.IsTerminalState(issue.State) && !entry.StopRequested {
			entry.StopRequested = true
			entry.StopReason = "terminal_state"
			entry.Cancel()
		}
	}
	return nil
}

func (controller *Controller) dispatchIssues(ctx context.Context, issues []domain.Issue) {
	for _, issue := range issues {
		if !controller.cfg.IsActiveState(issue.State) {
			continue
		}
		if !controller.canDispatch(issue) {
			continue
		}
		controller.startIssueRun(ctx, issue, 0)
	}
}

func (controller *Controller) canDispatch(issue domain.Issue) bool {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	if len(controller.running) >= controller.cfg.Orchestrator.MaxConcurrentAgents {
		return false
	}
	if _, claimed := controller.claimed[issue.ID]; claimed {
		return false
	}
	controller.claimed[issue.ID] = struct{}{}
	return true
}

func (controller *Controller) startIssueRun(ctx context.Context, issue domain.Issue, attempt int) {
	runCtx, cancel := context.WithCancel(ctx)
	entry := &runningEntry{Issue: issue, Identifier: issue.Identifier, WorkspacePath: workspacePathForIssue(controller.cfg.Workspace.Root, issue.Identifier), StartedAt: time.Now().UTC(), RetryAttempt: attempt, Cancel: cancel}
	controller.mu.Lock()
	controller.running[issue.ID] = entry
	controller.mu.Unlock()
	go func() {
		result, err := controller.runner.Run(runCtx, issue, attempt, func(event agent.Event) {
			controller.onAgentEvent(issue.ID, event)
		})
		controller.onRunFinished(issue, result, err)
	}()
}

func (controller *Controller) onAgentEvent(issueID string, event agent.Event) {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	entry := controller.running[issueID]
	if entry == nil {
		return
	}
	entry.LastEvent = event.Type
	entry.LastMessage = event.Message
	entry.Usage = event.Usage
	entry.SessionID = event.SessionID
	entry.CodexPID = event.CodexAppServerPID
	if event.Type == "session_started" {
		entry.Turns++
	}
	if !event.Timestamp.IsZero() {
		timestamp := event.Timestamp
		entry.LastTimestamp = &timestamp
	}
}

func (controller *Controller) onRunFinished(issue domain.Issue, result runner.Result, err error) {
	controller.mu.Lock()
	entry := controller.running[issue.ID]
	delete(controller.running, issue.ID)
	controller.mu.Unlock()
	if entry == nil {
		return
	}

	stopRequested := entry.StopRequested
	stopReason := entry.StopReason
	if stopRequested {
		controller.releaseClaim(issue.ID)
		if stopReason == "terminal_state" {
			_ = controller.workspace.RemoveIssueWorkspaces(context.Background(), issue.Identifier)
		}
		return
	}

	if err != nil {
		controller.scheduleRetry(issue, entry.RetryAttempt+1, false, err.Error())
		return
	}

	if result.Continuation {
		controller.scheduleRetry(issue, entry.RetryAttempt+1, true, "continuation requested")
		return
	}

	controller.releaseClaim(issue.ID)
}

func (controller *Controller) scheduleRetry(issue domain.Issue, attempt int, continuation bool, reason string) {
	delay := retryDelay(controller.cfg, attempt, continuation)
	dueAt := time.Now().UTC().Add(delay)
	controller.mu.Lock()
	if old := controller.retries[issue.ID]; old != nil && old.Timer != nil {
		old.Timer.Stop()
	}
	entry := &retryEntry{IssueID: issue.ID, Identifier: issue.Identifier, Attempt: attempt, DueAt: dueAt, Error: reason, Continuation: continuation}
	entry.Timer = time.AfterFunc(delay, func() { controller.handleRetryDue(issue, attempt, continuation, reason) })
	controller.retries[issue.ID] = entry
	controller.mu.Unlock()
}

func (controller *Controller) handleRetryDue(issue domain.Issue, attempt int, continuation bool, reason string) {
	controller.mu.Lock()
	delete(controller.retries, issue.ID)
	if len(controller.running) >= controller.cfg.Orchestrator.MaxConcurrentAgents {
		controller.mu.Unlock()
		controller.scheduleRetry(issue, attempt, continuation, "retry deferred: no available slots")
		return
	}
	controller.mu.Unlock()

	issues, err := controller.tracker.FetchIssuesByIDs(context.Background(), []string{issue.ID})
	if err != nil || len(issues) == 0 {
		controller.scheduleRetry(issue, attempt+1, false, fmt.Sprintf("retry refresh failed: %v", err))
		return
	}
	refreshed := issues[0]
	if controller.cfg.IsTerminalState(refreshed.State) {
		controller.releaseClaim(issue.ID)
		_ = controller.workspace.RemoveIssueWorkspaces(context.Background(), refreshed.Identifier)
		return
	}
	if !controller.cfg.IsActiveState(refreshed.State) {
		controller.releaseClaim(issue.ID)
		return
	}
	controller.startIssueRun(context.Background(), refreshed, attempt)
}

func (controller *Controller) releaseClaim(issueID string) {
	controller.mu.Lock()
	if retry := controller.retries[issueID]; retry != nil && retry.Timer != nil {
		retry.Timer.Stop()
	}
	delete(controller.claimed, issueID)
	delete(controller.retries, issueID)
	controller.mu.Unlock()
}

func (controller *Controller) setLastError(err error) {
	if err == nil {
		return
	}
	controller.logger.Warn("poll cycle error", slog.Any("error", err))
	controller.mu.Lock()
	controller.lastError = err.Error()
	controller.mu.Unlock()
}

func retryDelay(cfg config.Config, attempt int, continuation bool) time.Duration {
	if continuation {
		return continuationRetryDelay
	}
	if attempt < 1 {
		attempt = 1
	}
	delay := failureRetryBaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
	}
	maxDelay := time.Duration(cfg.Orchestrator.MaxRetryBackoffMs) * time.Millisecond
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func workspacePathForIssue(root string, identifier string) string {
	builder := strings.Builder{}
	trimmed := strings.TrimSpace(identifier)
	if trimmed == "" {
		trimmed = "issue"
	}
	for _, char := range trimmed {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case strings.ContainsRune("._-", char):
			builder.WriteRune(char)
		default:
			builder.WriteRune('_')
		}
	}
	return filepath.Join(root, builder.String())
}
