package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/denggeng/symphony-go/internal/workflow"
)

func TestFromWorkflowAppliesDefaults(t *testing.T) {
	t.Parallel()
	definition := workflow.Definition{Config: map[string]any{}}
	cfg, err := FromWorkflow(definition)
	if err != nil {
		t.Fatalf("from workflow: %v", err)
	}
	if cfg.Orchestrator.PollIntervalMs != 30_000 {
		t.Fatalf("unexpected poll interval: %d", cfg.Orchestrator.PollIntervalMs)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("unexpected server port: %d", cfg.Server.Port)
	}
	if cfg.Agent.MaxTurns != 20 {
		t.Fatalf("unexpected agent max turns: %d", cfg.Agent.MaxTurns)
	}
	if cfg.Agent.PersistPromptsToResults {
		t.Fatalf("expected prompt persistence to default to false")
	}
	if cfg.Codex.ApprovalPolicy != "never" {
		t.Fatalf("unexpected approval policy: %#v", cfg.Codex.ApprovalPolicy)
	}
}

func TestFromWorkflowExpandsWorkspaceRoot(t *testing.T) {
	t.Setenv("SYMPHONY_WORKSPACE_ROOT", filepath.Join(t.TempDir(), "workspaces"))
	definition := workflow.Definition{Config: map[string]any{"workspace": map[string]any{"root": "$SYMPHONY_WORKSPACE_ROOT"}}}
	cfg, err := FromWorkflow(definition)
	if err != nil {
		t.Fatalf("from workflow: %v", err)
	}
	if want := os.Getenv("SYMPHONY_WORKSPACE_ROOT"); cfg.Workspace.Root != want {
		t.Fatalf("unexpected workspace root: got %q want %q", cfg.Workspace.Root, want)
	}
}

func TestFromWorkflowRejectsUnknownTrackerKind(t *testing.T) {
	t.Parallel()
	definition := workflow.Definition{Config: map[string]any{"tracker": map[string]any{"kind": "trello"}}}
	if _, err := FromWorkflow(definition); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestFromWorkflowExpandsServerAuth(t *testing.T) {
	t.Setenv("SYMPHONY_SERVER_AUTH_USERNAME", "admin")
	t.Setenv("SYMPHONY_SERVER_AUTH_PASSWORD", "secret")
	definition := workflow.Definition{Config: map[string]any{"server": map[string]any{"username": "$SYMPHONY_SERVER_AUTH_USERNAME", "password": "$SYMPHONY_SERVER_AUTH_PASSWORD"}}}
	cfg, err := FromWorkflow(definition)
	if err != nil {
		t.Fatalf("from workflow: %v", err)
	}
	if cfg.Server.Username != "admin" || cfg.Server.Password != "secret" {
		t.Fatalf("unexpected server auth values: %#v", cfg.Server)
	}
	if !cfg.Summary().Server.AuthEnabled {
		t.Fatalf("expected auth to be enabled in summary")
	}
}

func TestFromWorkflowRejectsPartialServerAuth(t *testing.T) {
	t.Parallel()
	definition := workflow.Definition{Config: map[string]any{"server": map[string]any{"username": "admin"}}}
	if _, err := FromWorkflow(definition); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestFromWorkflowAppliesLocalDefaults(t *testing.T) {
	t.Parallel()
	definition := workflow.Definition{Config: map[string]any{"tracker": map[string]any{"kind": "local"}}}
	cfg, err := FromWorkflow(definition)
	if err != nil {
		t.Fatalf("from workflow: %v", err)
	}
	if cfg.Local.InboxDir == "" || cfg.Local.StateDir == "" || cfg.Local.ArchiveDir == "" || cfg.Local.ResultsDir == "" {
		t.Fatalf("expected local directories to be defaulted: %#v", cfg.Local)
	}
	if !cfg.IsActiveState("To Do") || !cfg.IsTerminalState("Done") {
		t.Fatalf("expected local state defaults to be active/terminal")
	}
	summary := cfg.Summary()
	if summary.Tracker.Kind != "local" {
		t.Fatalf("expected local tracker summary")
	}
	if summary.Local == nil || summary.Local.InboxDir == "" || summary.Local.ResultsDir == "" {
		t.Fatalf("expected local summary directories: %#v", summary.Local)
	}
}

func TestFromWorkflowNormalizesConcurrencyLimits(t *testing.T) {
	t.Parallel()
	definition := workflow.Definition{Config: map[string]any{
		"tracker": map[string]any{"kind": "local"},
		"orchestrator": map[string]any{
			"max_concurrent_agents": 2,
			"concurrency_limits":    map[string]any{" Review ": 1, "default": 1},
		},
	}}
	cfg, err := FromWorkflow(definition)
	if err != nil {
		t.Fatalf("from workflow: %v", err)
	}
	if got, want := cfg.Orchestrator.ConcurrencyLimits["review"], 1; got != want {
		t.Fatalf("unexpected review concurrency limit: got %d want %d", got, want)
	}
	if got, want := cfg.Orchestrator.ConcurrencyLimits["default"], 1; got != want {
		t.Fatalf("unexpected default concurrency limit: got %d want %d", got, want)
	}
	summary := cfg.Summary()
	if got, want := summary.Orchestrator.ConcurrencyLimits["review"], 1; got != want {
		t.Fatalf("unexpected summary review concurrency limit: got %d want %d", got, want)
	}
}

func TestFromWorkflowRejectsInvalidConcurrencyLimit(t *testing.T) {
	t.Parallel()
	definition := workflow.Definition{Config: map[string]any{
		"tracker": map[string]any{"kind": "local"},
		"orchestrator": map[string]any{
			"concurrency_limits": map[string]any{"review": 0},
		},
	}}
	if _, err := FromWorkflow(definition); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestFromWorkflowRejectsMissingExpandedWorkspaceRoot(t *testing.T) {
	t.Parallel()
	definition := workflow.Definition{Config: map[string]any{"workspace": map[string]any{"root": "$SYMPHONY_MISSING_WORKSPACE_ROOT"}}}
	if _, err := FromWorkflow(definition); err == nil {
		t.Fatalf("expected validation error for missing expanded workspace root")
	}
}

func TestFromWorkflowAppliesWorkspaceSyncBackDefaults(t *testing.T) {
	t.Parallel()

	definition := workflow.Definition{Config: map[string]any{
		"tracker": map[string]any{"kind": "local"},
		"workspace": map[string]any{
			"sync_back": map[string]any{"path": filepath.Join(t.TempDir(), "baseline")},
		},
	}}
	cfg, err := FromWorkflow(definition)
	if err != nil {
		t.Fatalf("from workflow: %v", err)
	}
	if got, want := cfg.Workspace.SyncBack.OnStates, []string{"Done"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("unexpected sync_back.on_states: got %v want %v", got, want)
	}
	if !cfg.ShouldSyncBackState("Done") {
		t.Fatalf("expected Done to be sync-back eligible")
	}
	if cfg.ShouldSyncBackState("Blocked") {
		t.Fatalf("expected Blocked to skip sync-back")
	}
	if got := cfg.Workspace.SyncBack.Excludes; len(got) < 2 || got[0] != ".git" || got[1] != "tmp" {
		t.Fatalf("unexpected sync_back excludes: %v", got)
	}
}

func TestFromWorkflowExpandsWorkspaceSeedAndSyncBackPaths(t *testing.T) {
	t.Setenv("SYMPHONY_WORKSPACE_SEED_DIR", filepath.Join(t.TempDir(), "seed"))
	t.Setenv("SYMPHONY_WORKSPACE_SYNC_DIR", filepath.Join(t.TempDir(), "sync"))

	definition := workflow.Definition{Config: map[string]any{
		"tracker": map[string]any{"kind": "local"},
		"workspace": map[string]any{
			"seed": map[string]any{
				"path":     "$SYMPHONY_WORKSPACE_SEED_DIR",
				"excludes": []any{"dist"},
			},
			"sync_back": map[string]any{
				"path":      "$SYMPHONY_WORKSPACE_SYNC_DIR",
				"on_states": []any{"Done", "Closed"},
				"excludes":  []any{"build"},
			},
		},
	}}
	cfg, err := FromWorkflow(definition)
	if err != nil {
		t.Fatalf("from workflow: %v", err)
	}
	if want := os.Getenv("SYMPHONY_WORKSPACE_SEED_DIR"); cfg.Workspace.Seed.Path != want {
		t.Fatalf("unexpected seed path: got %q want %q", cfg.Workspace.Seed.Path, want)
	}
	if want := os.Getenv("SYMPHONY_WORKSPACE_SYNC_DIR"); cfg.Workspace.SyncBack.Path != want {
		t.Fatalf("unexpected sync_back path: got %q want %q", cfg.Workspace.SyncBack.Path, want)
	}
	if got := cfg.Workspace.Seed.Excludes; len(got) != 3 || got[0] != ".git" || got[1] != "tmp" || got[2] != "dist" {
		t.Fatalf("unexpected seed excludes: %v", got)
	}
	if got := cfg.Workspace.SyncBack.Excludes; len(got) != 3 || got[0] != ".git" || got[1] != "tmp" || got[2] != "build" {
		t.Fatalf("unexpected sync_back excludes: %v", got)
	}
	summary := cfg.Summary()
	if summary.Workspace.Seed == nil || summary.Workspace.Seed.Path != cfg.Workspace.Seed.Path {
		t.Fatalf("expected workspace seed summary: %#v", summary.Workspace)
	}
	if summary.Workspace.SyncBack == nil || summary.Workspace.SyncBack.Path != cfg.Workspace.SyncBack.Path {
		t.Fatalf("expected workspace sync_back summary: %#v", summary.Workspace)
	}
}

func TestFromWorkflowRejectsWorkspaceSyncBackStatesWithoutPath(t *testing.T) {
	t.Parallel()

	definition := workflow.Definition{Config: map[string]any{
		"tracker": map[string]any{"kind": "local"},
		"workspace": map[string]any{
			"sync_back": map[string]any{
				"on_states": []any{"Done"},
			},
		},
	}}
	if _, err := FromWorkflow(definition); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestFromWorkflowEnablesPromptPersistence(t *testing.T) {
	t.Parallel()
	definition := workflow.Definition{Config: map[string]any{
		"tracker": map[string]any{"kind": "local"},
		"agent":   map[string]any{"persist_prompts_to_results": true},
	}}
	cfg, err := FromWorkflow(definition)
	if err != nil {
		t.Fatalf("from workflow: %v", err)
	}
	if !cfg.Agent.PersistPromptsToResults {
		t.Fatalf("expected prompt persistence to be enabled")
	}
	if !cfg.Summary().Agent.PersistPromptsToResults {
		t.Fatalf("expected prompt persistence in summary")
	}
}
