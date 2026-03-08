package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
)

var ErrWorkspaceOutsideRoot = errors.New("workspace path is outside workspace root")

type Workspace struct {
	Path       string
	Key        string
	CreatedNow bool
}

type Manager struct {
	cfg    config.Config
	logger *slog.Logger
}

func New(cfg config.Config, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{cfg: cfg, logger: logger}
}

func (manager *Manager) CreateForIssue(ctx context.Context, issue domain.Issue) (Workspace, error) {
	if err := os.MkdirAll(manager.cfg.Workspace.Root, 0o755); err != nil {
		return Workspace{}, fmt.Errorf("create workspace root: %w", err)
	}

	workspaceKey := safeIdentifier(issue.Identifier)
	workspacePath := filepath.Join(manager.cfg.Workspace.Root, workspaceKey)
	if err := manager.validatePath(workspacePath); err != nil {
		return Workspace{}, err
	}

	createdNow, err := manager.ensureWorkspace(workspacePath)
	if err != nil {
		return Workspace{}, err
	}

	workspace := Workspace{Path: workspacePath, Key: workspaceKey, CreatedNow: createdNow}
	if createdNow {
		if err := manager.maybeRunHook(ctx, manager.cfg.Hooks.AfterCreate, "after_create", workspacePath, issue, false); err != nil {
			return Workspace{}, err
		}
	}

	return workspace, nil
}

func (manager *Manager) Remove(ctx context.Context, workspacePath string) error {
	if _, err := os.Stat(workspacePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := manager.validatePath(workspacePath); err != nil {
		return err
	}
	identifier := filepath.Base(workspacePath)
	_ = manager.maybeRunHook(ctx, manager.cfg.Hooks.BeforeRemove, "before_remove", workspacePath, domain.Issue{Identifier: identifier}, true)
	return os.RemoveAll(workspacePath)
}

func (manager *Manager) RemoveIssueWorkspaces(ctx context.Context, identifier string) error {
	workspacePath := filepath.Join(manager.cfg.Workspace.Root, safeIdentifier(identifier))
	return manager.Remove(ctx, workspacePath)
}

func (manager *Manager) RunBeforeRun(ctx context.Context, workspacePath string, issue domain.Issue) error {
	return manager.maybeRunHook(ctx, manager.cfg.Hooks.BeforeRun, "before_run", workspacePath, issue, false)
}

func (manager *Manager) RunAfterRun(ctx context.Context, workspacePath string, issue domain.Issue) {
	_ = manager.maybeRunHook(ctx, manager.cfg.Hooks.AfterRun, "after_run", workspacePath, issue, true)
}

func (manager *Manager) ensureWorkspace(workspacePath string) (bool, error) {
	stat, err := os.Stat(workspacePath)
	switch {
	case err == nil && stat.IsDir():
		manager.cleanTemporaryArtifacts(workspacePath)
		return false, nil
	case err == nil:
		if err := os.RemoveAll(workspacePath); err != nil {
			return false, fmt.Errorf("remove non-directory workspace: %w", err)
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return false, fmt.Errorf("stat workspace: %w", err)
	}

	if err := os.RemoveAll(workspacePath); err != nil {
		return false, fmt.Errorf("reset workspace: %w", err)
	}
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return false, fmt.Errorf("create workspace: %w", err)
	}
	return true, nil
}

func (manager *Manager) maybeRunHook(ctx context.Context, command string, hookName string, workspacePath string, issue domain.Issue, ignoreFailure bool) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	manager.logger.Info("running workspace hook", slog.String("hook", hookName), slog.String("workspace", workspacePath), slog.String("issue_identifier", issue.Identifier))

	hookCtx, cancel := context.WithTimeout(ctx, time.Duration(manager.cfg.Hooks.TimeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(hookCtx, "sh", "-lc", command)
	cmd.Dir = workspacePath
	cmd.Env = append(os.Environ(),
		"SYMPHONY_ISSUE_ID="+issue.ID,
		"SYMPHONY_ISSUE_IDENTIFIER="+issue.Identifier,
		"SYMPHONY_ISSUE_TITLE="+issue.Title,
		"SYMPHONY_ISSUE_STATE="+issue.State,
		"SYMPHONY_WORKSPACE_PATH="+workspacePath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		wrapped := fmt.Errorf("workspace hook %s failed: %w output=%s", hookName, err, trimLogOutput(output))
		manager.logger.Warn("workspace hook failed", slog.String("hook", hookName), slog.Any("error", wrapped))
		if ignoreFailure {
			return nil
		}
		return wrapped
	}
	return nil
}

func (manager *Manager) cleanTemporaryArtifacts(workspacePath string) {
	for _, entry := range []string{"tmp"} {
		_ = os.RemoveAll(filepath.Join(workspacePath, entry))
	}
}

func (manager *Manager) validatePath(workspacePath string) error {
	expandedWorkspace := filepath.Clean(workspacePath)
	root := filepath.Clean(manager.cfg.Workspace.Root)
	if expandedWorkspace == root {
		return fmt.Errorf("workspace cannot equal workspace root: %w", ErrWorkspaceOutsideRoot)
	}
	relative, err := filepath.Rel(root, expandedWorkspace)
	if err != nil {
		return fmt.Errorf("rel workspace path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("workspace %q is outside root %q: %w", expandedWorkspace, root, ErrWorkspaceOutsideRoot)
	}
	current := root
	for _, segment := range strings.Split(relative, string(os.PathSeparator)) {
		if segment == "." || segment == "" {
			continue
		}
		current = filepath.Join(current, segment)
		if info, err := os.Lstat(current); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace path contains symlink: %s", current)
		}
	}
	return nil
}

func safeIdentifier(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		identifier = "issue"
	}
	builder := strings.Builder{}
	for _, char := range identifier {
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

func trimLogOutput(output []byte) string {
	text := strings.TrimSpace(string(output))
	if len(text) <= 2048 {
		return text
	}
	return text[:2048] + "... (truncated)"
}
