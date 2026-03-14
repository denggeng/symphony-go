package orchestrator

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
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
const maxRunHistoryEntries = 50
const maxRunEventsPerRun = 200
const maxSnapshotHistoryEntries = 12

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
	backlog        []BacklogSnapshot
	history        []*historyEntry
	historyByID    map[string]*historyEntry
	nextRunSeq     int64
	cancel         context.CancelFunc
	started        bool
}

type runningEntry struct {
	RunID         string
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
	Events        []RunEventSnapshot
}

type retryEntry struct {
	Issue        domain.Issue
	IssueID      string
	Identifier   string
	Attempt      int
	DueAt        time.Time
	Error        string
	Timer        *time.Timer
	Continuation bool
}

type historyEntry struct {
	Run    RunHistorySnapshot
	Events []RunEventSnapshot
}

type Snapshot struct {
	Service  ServiceSnapshot      `json:"service"`
	Workflow WorkflowSnapshot     `json:"workflow"`
	Config   config.Summary       `json:"config"`
	Polling  PollingSnapshot      `json:"polling"`
	Running  []RunningSnapshot    `json:"running"`
	Retrying []RetryingSnapshot   `json:"retrying"`
	Backlog  []BacklogSnapshot    `json:"backlog"`
	History  []RunHistorySnapshot `json:"history"`
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
	RunID          string      `json:"run_id"`
	IssueID        string      `json:"issue_id"`
	Identifier     string      `json:"identifier"`
	State          string      `json:"state"`
	Lane           string      `json:"lane,omitempty"`
	ReviewOf       string      `json:"review_of,omitempty"`
	Title          string      `json:"title,omitempty"`
	QueueStatus    string      `json:"queue_status"`
	Priority       *int        `json:"priority,omitempty"`
	Order          *int        `json:"order,omitempty"`
	Dependencies   []string    `json:"dependencies,omitempty"`
	BlockedBy      []string    `json:"blocked_by,omitempty"`
	UpdatedAt      *time.Time  `json:"updated_at,omitempty"`
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
	EventCount     int         `json:"event_count"`
}

type RetryingSnapshot struct {
	IssueID      string     `json:"issue_id"`
	Identifier   string     `json:"identifier"`
	Title        string     `json:"title,omitempty"`
	State        string     `json:"state,omitempty"`
	Lane         string     `json:"lane,omitempty"`
	ReviewOf     string     `json:"review_of,omitempty"`
	QueueStatus  string     `json:"queue_status"`
	Priority     *int       `json:"priority,omitempty"`
	Order        *int       `json:"order,omitempty"`
	Dependencies []string   `json:"dependencies,omitempty"`
	BlockedBy    []string   `json:"blocked_by,omitempty"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
	Attempt      int        `json:"attempt"`
	DueAt        time.Time  `json:"due_at"`
	DueInMs      int64      `json:"due_in_ms"`
	Error        string     `json:"error,omitempty"`
	Continuation bool       `json:"continuation"`
}

type BacklogSnapshot struct {
	IssueID      string     `json:"issue_id"`
	Identifier   string     `json:"identifier"`
	Title        string     `json:"title,omitempty"`
	State        string     `json:"state"`
	Lane         string     `json:"lane,omitempty"`
	ReviewOf     string     `json:"review_of,omitempty"`
	QueueStatus  string     `json:"queue_status"`
	Priority     *int       `json:"priority,omitempty"`
	Order        *int       `json:"order,omitempty"`
	Dependencies []string   `json:"dependencies,omitempty"`
	BlockedBy    []string   `json:"blocked_by,omitempty"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
}

type IssueDetailSnapshot struct {
	IssueID        string      `json:"issue_id"`
	Identifier     string      `json:"identifier"`
	Title          string      `json:"title,omitempty"`
	State          string      `json:"state,omitempty"`
	Lane           string      `json:"lane,omitempty"`
	ReviewOf       string      `json:"review_of,omitempty"`
	QueueStatus    string      `json:"queue_status"`
	Priority       *int        `json:"priority,omitempty"`
	Order          *int        `json:"order,omitempty"`
	Dependencies   []string    `json:"dependencies,omitempty"`
	BlockedBy      []string    `json:"blocked_by,omitempty"`
	UpdatedAt      *time.Time  `json:"updated_at,omitempty"`
	RunID          string      `json:"run_id,omitempty"`
	WorkspacePath  string      `json:"workspace_path,omitempty"`
	SessionID      string      `json:"session_id,omitempty"`
	CodexPID       string      `json:"codex_app_server_pid,omitempty"`
	Turns          int         `json:"turns"`
	RetryAttempt   int         `json:"retry_attempt"`
	StartedAt      *time.Time  `json:"started_at,omitempty"`
	RuntimeSeconds int         `json:"runtime_seconds"`
	DueAt          *time.Time  `json:"due_at,omitempty"`
	DueInMs        int64       `json:"due_in_ms,omitempty"`
	LastEvent      string      `json:"last_event,omitempty"`
	LastMessage    string      `json:"last_message,omitempty"`
	LastTimestamp  *time.Time  `json:"last_timestamp,omitempty"`
	Usage          agent.Usage `json:"usage"`
	StopRequested  bool        `json:"stop_requested"`
	StopReason     string      `json:"stop_reason,omitempty"`
	EventCount     int         `json:"event_count"`
	Error          string      `json:"error,omitempty"`
	Continuation   bool        `json:"continuation"`
}

// RunEventSnapshot is a sanitized, UI-safe event emitted during one run.
type RunEventSnapshot struct {
	Timestamp         time.Time   `json:"timestamp"`
	Type              string      `json:"type"`
	SessionID         string      `json:"session_id,omitempty"`
	ThreadID          string      `json:"thread_id,omitempty"`
	TurnID            string      `json:"turn_id,omitempty"`
	CodexAppServerPID string      `json:"codex_app_server_pid,omitempty"`
	Message           string      `json:"message,omitempty"`
	Usage             agent.Usage `json:"usage"`
}

// RunHistorySnapshot is the summary of a completed or interrupted run.
type RunHistorySnapshot struct {
	RunID          string      `json:"run_id"`
	IssueID        string      `json:"issue_id"`
	Identifier     string      `json:"identifier"`
	Title          string      `json:"title,omitempty"`
	State          string      `json:"state,omitempty"`
	Lane           string      `json:"lane,omitempty"`
	ReviewOf       string      `json:"review_of,omitempty"`
	Status         string      `json:"status"`
	WorkspacePath  string      `json:"workspace_path"`
	SessionID      string      `json:"session_id,omitempty"`
	CodexPID       string      `json:"codex_app_server_pid,omitempty"`
	Turns          int         `json:"turns"`
	RetryAttempt   int         `json:"retry_attempt"`
	StartedAt      time.Time   `json:"started_at"`
	FinishedAt     time.Time   `json:"finished_at"`
	RuntimeSeconds int         `json:"runtime_seconds"`
	LastEvent      string      `json:"last_event,omitempty"`
	LastMessage    string      `json:"last_message,omitempty"`
	LastTimestamp  *time.Time  `json:"last_timestamp,omitempty"`
	Usage          agent.Usage `json:"usage"`
	Error          string      `json:"error,omitempty"`
	Continuation   bool        `json:"continuation"`
	StopRequested  bool        `json:"stop_requested"`
	StopReason     string      `json:"stop_reason,omitempty"`
	EventCount     int         `json:"event_count"`
}

// RunHistoryDetail is the detailed history payload for one completed run.
type RunHistoryDetail struct {
	Run    RunHistorySnapshot `json:"run"`
	Events []RunEventSnapshot `json:"events"`
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
		logger:      logger,
		workflow:    opts.Workflow,
		cfg:         opts.Config,
		tracker:     opts.Tracker,
		workspace:   opts.Workspace,
		runner:      opts.Runner,
		startedAt:   now,
		running:     map[string]*runningEntry{},
		claimed:     map[string]struct{}{},
		retries:     map[string]*retryEntry{},
		historyByID: map[string]*historyEntry{},
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
			RunID:          entry.RunID,
			IssueID:        issueID,
			Identifier:     entry.Identifier,
			State:          entry.Issue.State,
			Lane:           entry.Issue.Lane,
			ReviewOf:       entry.Issue.ReviewOf,
			Title:          entry.Issue.Title,
			QueueStatus:    "running",
			Priority:       entry.Issue.Priority,
			Order:          entry.Issue.Order,
			Dependencies:   append([]string(nil), entry.Issue.Dependencies...),
			BlockedBy:      append([]string(nil), entry.Issue.BlockedBy...),
			UpdatedAt:      entry.Issue.UpdatedAt,
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
			EventCount:     len(entry.Events),
		})
	}
	retrying := make([]RetryingSnapshot, 0, len(controller.retries))
	for _, entry := range controller.retries {
		retrying = append(retrying, RetryingSnapshot{
			IssueID:      entry.IssueID,
			Identifier:   entry.Identifier,
			Title:        entry.Issue.Title,
			State:        entry.Issue.State,
			Lane:         entry.Issue.Lane,
			ReviewOf:     entry.Issue.ReviewOf,
			QueueStatus:  "retrying",
			Priority:     entry.Issue.Priority,
			Order:        entry.Issue.Order,
			Dependencies: append([]string(nil), entry.Issue.Dependencies...),
			BlockedBy:    append([]string(nil), entry.Issue.BlockedBy...),
			UpdatedAt:    entry.Issue.UpdatedAt,
			Attempt:      entry.Attempt,
			DueAt:        entry.DueAt,
			DueInMs:      maxInt64(0, entry.DueAt.Sub(now).Milliseconds()),
			Error:        entry.Error,
			Continuation: entry.Continuation,
		})
	}
	slices.SortFunc(running, func(left RunningSnapshot, right RunningSnapshot) int {
		if diff := cmp.Compare(left.Identifier, right.Identifier); diff != 0 {
			return diff
		}
		return cmp.Compare(left.IssueID, right.IssueID)
	})
	slices.SortFunc(retrying, func(left RetryingSnapshot, right RetryingSnapshot) int {
		if diff := cmp.Compare(left.DueAt.UnixMilli(), right.DueAt.UnixMilli()); diff != 0 {
			return diff
		}
		return cmp.Compare(left.Identifier, right.Identifier)
	})
	backlog := append([]BacklogSnapshot(nil), controller.backlog...)
	return Snapshot{
		Service:  ServiceSnapshot{Name: "symphony-go", Version: "dev", StartedAt: controller.startedAt, Uptime: now.Sub(controller.startedAt).Round(time.Second).String()},
		Workflow: WorkflowSnapshot{Path: controller.workflow.Path, LoadedAt: controller.startedAt, HasFrontMatter: controller.workflow.HasFrontMatter, PromptLength: len(controller.workflow.Prompt)},
		Config:   controller.cfg.Summary(),
		Polling:  PollingSnapshot{Checking: controller.pollInProgress, PollIntervalMs: controller.cfg.Orchestrator.PollIntervalMs, NextPollAt: controller.nextPollAt, LastPollAt: controller.lastPollAt, LastError: controller.lastError},
		Running:  running,
		Retrying: retrying,
		Backlog:  backlog,
		History:  controller.historySnapshotsLocked(maxSnapshotHistoryEntries),
	}
}

func (controller *Controller) IssueSnapshot(identifier string) (IssueDetailSnapshot, bool) {
	controller.mu.RLock()
	defer controller.mu.RUnlock()
	now := time.Now().UTC()
	for issueID, entry := range controller.running {
		if entry.Identifier == identifier || issueID == identifier {
			return issueDetailFromRunningEntry(issueID, entry, now), true
		}
	}
	for issueID, entry := range controller.retries {
		if entry.Identifier == identifier || issueID == identifier {
			return issueDetailFromRetryEntry(issueID, entry, now), true
		}
	}
	for _, entry := range controller.backlog {
		if entry.Identifier == identifier || entry.IssueID == identifier {
			return issueDetailFromBacklogSnapshot(entry), true
		}
	}
	return IssueDetailSnapshot{}, false
}

func (controller *Controller) History() []RunHistorySnapshot {
	controller.mu.RLock()
	defer controller.mu.RUnlock()
	return controller.historySnapshotsLocked(maxRunHistoryEntries)
}

func (controller *Controller) RunHistory(runID string) (RunHistoryDetail, bool) {
	controller.mu.RLock()
	defer controller.mu.RUnlock()
	entry := controller.historyByID[strings.TrimSpace(runID)]
	if entry == nil {
		return RunHistoryDetail{}, false
	}
	return RunHistoryDetail{Run: entry.Run, Events: cloneRunEvents(entry.Events)}, true
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

	trackerKind := ""
	if controller.tracker != nil {
		trackerKind = controller.tracker.Kind()
	}
	var err error
	activeLoaded := false
	activeIssues := []domain.Issue(nil)
	if trackerKind == "local" {
		activeIssues, err = controller.tracker.FetchIssuesByStates(ctx, controller.cfg.ActiveStates())
		if err != nil {
			controller.setLastError(err)
		} else {
			activeLoaded = true
		}
	} else {
		controller.clearBacklogSnapshot()
	}

	issues, err := controller.tracker.FetchCandidateIssues(ctx)
	if err != nil {
		controller.setLastError(err)
	} else {
		domain.SortIssues(issues)
		controller.dispatchIssues(ctx, issues)
	}
	if activeLoaded {
		controller.updateBacklogSnapshot(activeIssues)
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
	issues, err := controller.tracker.FetchIssuesByStates(ctx, controller.cfg.TerminalStates())
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

func (controller *Controller) clearBacklogSnapshot() {
	controller.mu.Lock()
	controller.backlog = nil
	controller.mu.Unlock()
}

func (controller *Controller) updateBacklogSnapshot(activeIssues []domain.Issue) {
	ready := make([]BacklogSnapshot, 0, len(activeIssues))
	blocked := make([]BacklogSnapshot, 0, len(activeIssues))

	controller.mu.Lock()
	defer controller.mu.Unlock()
	for _, issue := range activeIssues {
		if _, ok := controller.running[issue.ID]; ok {
			continue
		}
		if _, ok := controller.retries[issue.ID]; ok {
			continue
		}
		snapshot := backlogSnapshotFromIssue(issue)
		if len(snapshot.BlockedBy) > 0 {
			blocked = append(blocked, snapshot)
			continue
		}
		ready = append(ready, snapshot)
	}
	controller.backlog = append(ready, blocked...)
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
			entry.Identifier = issue.Identifier
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
	if !controller.hasLaneCapacityLocked(issue.Lane) {
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
	startedAt := time.Now().UTC()
	controller.mu.Lock()
	runID := controller.nextRunIDLocked(issue.Identifier, startedAt)
	entry := &runningEntry{RunID: runID, Issue: issue, Identifier: issue.Identifier, WorkspacePath: workspacePathForIssue(controller.cfg.Workspace.Root, issue.Identifier), StartedAt: startedAt, RetryAttempt: attempt, Cancel: cancel}
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
	entry.Events = appendRunEvent(entry.Events, runEventSnapshotFromAgent(event))
	if event.Type == "session_started" {
		entry.Turns++
	}
	if !event.Timestamp.IsZero() {
		timestamp := event.Timestamp
		entry.LastTimestamp = &timestamp
	}
}

func (controller *Controller) onRunFinished(issue domain.Issue, result runner.Result, err error) {
	finishedAt := time.Now().UTC()
	controller.mu.Lock()
	entry := controller.running[issue.ID]
	delete(controller.running, issue.ID)
	if entry != nil {
		controller.appendHistoryLocked(buildRunHistorySnapshot(entry, result, err, finishedAt), entry.Events)
	}
	controller.mu.Unlock()
	if entry == nil {
		return
	}

	stopRequested := entry.StopRequested
	stopReason := entry.StopReason
	if stopRequested {
		controller.releaseClaim(issue.ID)
		if stopReason == "terminal_state" {
			if syncErr := controller.syncBackWorkspace(entry.Issue, entry.WorkspacePath); syncErr == nil {
				_ = controller.workspace.RemoveIssueWorkspaces(context.Background(), entry.Identifier)
			}
		}
		return
	}

	if err != nil {
		if errors.Is(err, workspace.ErrWorkspaceSyncBackFailed) {
			controller.logger.Warn("workspace sync-back failed", slog.String("issue_identifier", entry.Identifier), slog.String("workspace", entry.WorkspacePath), slog.Any("error", err))
			controller.releaseClaim(issue.ID)
			return
		}
		controller.scheduleRetry(issue, entry.RetryAttempt+1, false, err.Error())
		return
	}

	if result.Continuation {
		controller.scheduleRetry(issue, entry.RetryAttempt+1, true, "continuation requested")
		return
	}

	controller.releaseClaim(issue.ID)
}

func (controller *Controller) syncBackWorkspace(issue domain.Issue, workspacePath string) error {
	if controller.workspace == nil {
		return nil
	}
	if err := controller.workspace.SyncBack(context.Background(), workspacePath, issue); err != nil {
		controller.logger.Warn("workspace sync-back failed", slog.String("issue_identifier", issue.Identifier), slog.String("workspace", workspacePath), slog.Any("error", err))
		return err
	}
	return nil
}

func (controller *Controller) scheduleRetry(issue domain.Issue, attempt int, continuation bool, reason string) {
	delay := retryDelay(controller.cfg, attempt, continuation)
	dueAt := time.Now().UTC().Add(delay)
	controller.mu.Lock()
	if old := controller.retries[issue.ID]; old != nil && old.Timer != nil {
		old.Timer.Stop()
	}
	entry := &retryEntry{Issue: issue, IssueID: issue.ID, Identifier: issue.Identifier, Attempt: attempt, DueAt: dueAt, Error: reason, Continuation: continuation}
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

func issueDetailFromRunningEntry(issueID string, entry *runningEntry, now time.Time) IssueDetailSnapshot {
	startedAt := entry.StartedAt
	return IssueDetailSnapshot{
		IssueID:        issueID,
		Identifier:     entry.Identifier,
		Title:          entry.Issue.Title,
		State:          entry.Issue.State,
		Lane:           entry.Issue.Lane,
		ReviewOf:       entry.Issue.ReviewOf,
		QueueStatus:    "running",
		Priority:       entry.Issue.Priority,
		Order:          entry.Issue.Order,
		Dependencies:   append([]string(nil), entry.Issue.Dependencies...),
		BlockedBy:      append([]string(nil), entry.Issue.BlockedBy...),
		UpdatedAt:      entry.Issue.UpdatedAt,
		RunID:          entry.RunID,
		WorkspacePath:  entry.WorkspacePath,
		SessionID:      entry.SessionID,
		CodexPID:       entry.CodexPID,
		Turns:          entry.Turns,
		RetryAttempt:   entry.RetryAttempt,
		StartedAt:      &startedAt,
		RuntimeSeconds: int(now.Sub(entry.StartedAt).Seconds()),
		LastEvent:      entry.LastEvent,
		LastMessage:    entry.LastMessage,
		LastTimestamp:  entry.LastTimestamp,
		Usage:          entry.Usage,
		StopRequested:  entry.StopRequested,
		StopReason:     entry.StopReason,
		EventCount:     len(entry.Events),
	}
}

func issueDetailFromRetryEntry(issueID string, entry *retryEntry, now time.Time) IssueDetailSnapshot {
	dueAt := entry.DueAt
	return IssueDetailSnapshot{
		IssueID:      issueID,
		Identifier:   entry.Identifier,
		Title:        entry.Issue.Title,
		State:        entry.Issue.State,
		Lane:         entry.Issue.Lane,
		ReviewOf:     entry.Issue.ReviewOf,
		QueueStatus:  "retrying",
		Priority:     entry.Issue.Priority,
		Order:        entry.Issue.Order,
		Dependencies: append([]string(nil), entry.Issue.Dependencies...),
		BlockedBy:    append([]string(nil), entry.Issue.BlockedBy...),
		UpdatedAt:    entry.Issue.UpdatedAt,
		RetryAttempt: entry.Attempt,
		DueAt:        &dueAt,
		DueInMs:      maxInt64(0, entry.DueAt.Sub(now).Milliseconds()),
		LastMessage:  entry.Error,
		Error:        entry.Error,
		Continuation: entry.Continuation,
	}
}

func issueDetailFromBacklogSnapshot(entry BacklogSnapshot) IssueDetailSnapshot {
	return IssueDetailSnapshot{
		IssueID:      entry.IssueID,
		Identifier:   entry.Identifier,
		Title:        entry.Title,
		State:        entry.State,
		Lane:         entry.Lane,
		ReviewOf:     entry.ReviewOf,
		QueueStatus:  entry.QueueStatus,
		Priority:     entry.Priority,
		Order:        entry.Order,
		Dependencies: append([]string(nil), entry.Dependencies...),
		BlockedBy:    append([]string(nil), entry.BlockedBy...),
		UpdatedAt:    entry.UpdatedAt,
	}
}

func backlogSnapshotFromIssue(issue domain.Issue) BacklogSnapshot {
	queueStatus := "ready"
	if len(issue.BlockedBy) > 0 {
		queueStatus = "blocked"
	}
	return BacklogSnapshot{
		IssueID:      issue.ID,
		Identifier:   issue.Identifier,
		Title:        issue.Title,
		State:        issue.State,
		Lane:         issue.Lane,
		ReviewOf:     issue.ReviewOf,
		QueueStatus:  queueStatus,
		Priority:     issue.Priority,
		Order:        issue.Order,
		Dependencies: append([]string(nil), issue.Dependencies...),
		BlockedBy:    append([]string(nil), issue.BlockedBy...),
		UpdatedAt:    issue.UpdatedAt,
	}
}

func (controller *Controller) hasLaneCapacityLocked(lane string) bool {
	limits := controller.cfg.Orchestrator.ConcurrencyLimits
	if len(limits) == 0 {
		return true
	}
	normalized := issueLaneKey(lane)
	limit, ok := limits[normalized]
	if !ok {
		return true
	}
	running := 0
	for _, entry := range controller.running {
		if issueLaneKey(entry.Issue.Lane) != normalized {
			continue
		}
		running++
		if running >= limit {
			return false
		}
	}
	return true
}

func issueLaneKey(lane string) string {
	trimmed := strings.ToLower(strings.TrimSpace(lane))
	if trimmed == "" {
		return "default"
	}
	return trimmed
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func buildRunHistorySnapshot(entry *runningEntry, result runner.Result, err error, finishedAt time.Time) RunHistorySnapshot {
	turns := entry.Turns
	if result.Turns > turns {
		turns = result.Turns
	}
	workspacePath := entry.WorkspacePath
	if workspacePath == "" {
		workspacePath = result.WorkspacePath
	}
	status := runStatus(entry.StopRequested, entry.StopReason, result.Continuation, err)
	runtimeSeconds := int(finishedAt.Sub(entry.StartedAt).Seconds())
	suppressError := shouldSuppressRunError(entry.StopRequested, entry.StopReason, err)
	message := entry.LastMessage
	if message == "" && err != nil && !suppressError {
		message = err.Error()
	}
	history := RunHistorySnapshot{
		RunID:          entry.RunID,
		IssueID:        entry.Issue.ID,
		Identifier:     entry.Identifier,
		Title:          entry.Issue.Title,
		State:          entry.Issue.State,
		Lane:           entry.Issue.Lane,
		ReviewOf:       entry.Issue.ReviewOf,
		Status:         status,
		WorkspacePath:  workspacePath,
		SessionID:      entry.SessionID,
		CodexPID:       entry.CodexPID,
		Turns:          turns,
		RetryAttempt:   entry.RetryAttempt,
		StartedAt:      entry.StartedAt,
		FinishedAt:     finishedAt,
		RuntimeSeconds: runtimeSeconds,
		LastEvent:      entry.LastEvent,
		LastMessage:    message,
		LastTimestamp:  entry.LastTimestamp,
		Usage:          entry.Usage,
		Continuation:   result.Continuation,
		StopRequested:  entry.StopRequested,
		StopReason:     entry.StopReason,
		EventCount:     len(entry.Events),
	}
	if err != nil && !suppressError {
		history.Error = err.Error()
	}
	return history
}

func shouldSuppressRunError(stopRequested bool, stopReason string, err error) bool {
	if err == nil {
		return false
	}
	if stopReason == "terminal_state" {
		return true
	}
	if stopRequested && errors.Is(err, context.Canceled) {
		return true
	}
	if stopRequested && strings.Contains(strings.ToLower(err.Error()), "context canceled") {
		return true
	}
	return false
}

func runStatus(stopRequested bool, stopReason string, continuation bool, err error) string {
	switch {
	case stopRequested && stopReason == "terminal_state":
		return "stopped_terminal"
	case stopRequested:
		return "stopped"
	case err != nil:
		return "failed"
	case continuation:
		return "continued"
	default:
		return "succeeded"
	}
}

func runEventSnapshotFromAgent(event agent.Event) RunEventSnapshot {
	return RunEventSnapshot{
		Timestamp:         event.Timestamp,
		Type:              event.Type,
		SessionID:         event.SessionID,
		ThreadID:          event.ThreadID,
		TurnID:            event.TurnID,
		CodexAppServerPID: event.CodexAppServerPID,
		Message:           event.Message,
		Usage:             event.Usage,
	}
}

func appendRunEvent(events []RunEventSnapshot, event RunEventSnapshot) []RunEventSnapshot {
	events = append(events, event)
	if len(events) <= maxRunEventsPerRun {
		return events
	}
	copy(events, events[len(events)-maxRunEventsPerRun:])
	return events[:maxRunEventsPerRun]
}

func (controller *Controller) historySnapshotsLocked(limit int) []RunHistorySnapshot {
	if limit <= 0 || limit > len(controller.history) {
		limit = len(controller.history)
	}
	history := make([]RunHistorySnapshot, 0, limit)
	for _, entry := range controller.history[:limit] {
		history = append(history, entry.Run)
	}
	return history
}

func (controller *Controller) appendHistoryLocked(run RunHistorySnapshot, events []RunEventSnapshot) {
	entry := &historyEntry{Run: run, Events: cloneRunEvents(events)}
	controller.history = append([]*historyEntry{entry}, controller.history...)
	controller.historyByID[run.RunID] = entry
	if len(controller.history) <= maxRunHistoryEntries {
		return
	}
	trimmed := controller.history[maxRunHistoryEntries:]
	controller.history = controller.history[:maxRunHistoryEntries]
	for _, old := range trimmed {
		delete(controller.historyByID, old.Run.RunID)
	}
}

func cloneRunEvents(events []RunEventSnapshot) []RunEventSnapshot {
	if len(events) == 0 {
		return nil
	}
	cloned := make([]RunEventSnapshot, len(events))
	copy(cloned, events)
	return cloned
}

func (controller *Controller) nextRunIDLocked(identifier string, startedAt time.Time) string {
	controller.nextRunSeq++
	return fmt.Sprintf("%s-%s-%03d", startedAt.Format("20060102T150405Z"), safeIdentifierValue(identifier), controller.nextRunSeq)
}

func workspacePathForIssue(root string, identifier string) string {
	return filepath.Join(root, safeIdentifierValue(identifier))
}

func safeIdentifierValue(identifier string) string {
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
	return builder.String()
}
