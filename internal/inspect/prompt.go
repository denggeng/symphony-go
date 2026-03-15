package inspect

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/envfile"
	"github.com/denggeng/symphony-go/internal/prompt"
	localtracker "github.com/denggeng/symphony-go/internal/tracker/local"
	"github.com/denggeng/symphony-go/internal/workflow"
)

// PromptRenderOptions describes which local task prompt to reconstruct.
type PromptRenderOptions struct {
	WorkflowPath  string
	TaskID        string
	WorkspacePath string
	Turn          int
	Attempt       int
}

// RenderLocalTaskPrompt rebuilds the exact prompt Symphony would send to Codex
// for a local Markdown task and a specific turn.
func RenderLocalTaskPrompt(opts PromptRenderOptions) (string, error) {
	workflowPath := strings.TrimSpace(opts.WorkflowPath)
	if workflowPath == "" {
		workflowPath = workflow.DefaultPath()
	}
	if err := envfile.LoadIfExists(filepath.Join(filepath.Dir(workflowPath), ".env")); err != nil {
		return "", err
	}

	loadedWorkflow, err := workflow.Load(workflowPath)
	if err != nil {
		return "", err
	}
	cfg, err := config.FromWorkflow(loadedWorkflow)
	if err != nil {
		return "", err
	}
	if cfg.Tracker.Kind != "local" {
		return "", fmt.Errorf("render-prompt currently supports tracker.kind=local workflows only")
	}

	identifier, err := resolveTaskIdentifier(opts.TaskID, opts.WorkspacePath)
	if err != nil {
		return "", err
	}
	issue, err := loadLocalIssue(cfg, identifier)
	if err != nil {
		return "", err
	}

	turn := opts.Turn
	if turn == 0 {
		turn = 1
	}
	if turn < 1 {
		return "", fmt.Errorf("turn must be greater than 0")
	}
	attempt := opts.Attempt
	if attempt == 0 {
		attempt = 1
	}
	if attempt < 1 {
		return "", fmt.Errorf("attempt must be greater than 0")
	}

	renderer := prompt.New(loadedWorkflow)
	return prompt.TurnPrompt(renderer, issue, attempt, turn, cfg.Agent.MaxTurns), nil
}

func loadLocalIssue(cfg config.Config, identifier string) (domain.Issue, error) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tracker := localtracker.New(cfg, logger)
	issues, err := tracker.FetchIssuesByIDs(context.Background(), []string{identifier})
	if err != nil {
		return domain.Issue{}, err
	}
	if len(issues) == 0 {
		return domain.Issue{}, fmt.Errorf("local task %q not found in %s or %s", identifier, cfg.Local.InboxDir, cfg.Local.ArchiveDir)
	}
	return issues[0], nil
}

func resolveTaskIdentifier(taskID string, workspacePath string) (string, error) {
	if trimmed := strings.TrimSpace(taskID); trimmed != "" {
		return trimmed, nil
	}
	trimmedWorkspace := strings.TrimSpace(workspacePath)
	if trimmedWorkspace == "" {
		return "", fmt.Errorf("provide either a task id or a workspace path")
	}
	identifier := strings.TrimSpace(filepath.Base(filepath.Clean(trimmedWorkspace)))
	if identifier == "" || identifier == "." || identifier == string(filepath.Separator) {
		return "", fmt.Errorf("cannot derive task id from workspace path %q", workspacePath)
	}
	return identifier, nil
}
