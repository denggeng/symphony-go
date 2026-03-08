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

func TestFromWorkflowRejectsMissingExpandedWorkspaceRoot(t *testing.T) {
	t.Parallel()
	definition := workflow.Definition{Config: map[string]any{"workspace": map[string]any{"root": "$SYMPHONY_MISSING_WORKSPACE_ROOT"}}}
	if _, err := FromWorkflow(definition); err == nil {
		t.Fatalf("expected validation error for missing expanded workspace root")
	}
}
