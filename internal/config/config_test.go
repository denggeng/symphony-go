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
