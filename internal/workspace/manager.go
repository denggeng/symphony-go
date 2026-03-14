package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

// ErrWorkspaceSyncBackFailed indicates that a configured workspace sync-back could not complete successfully.
var ErrWorkspaceSyncBackFailed = errors.New("workspace sync-back failed")

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
		if err := manager.seedWorkspace(ctx, workspacePath, issue); err != nil {
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

// SyncBack copies workspace files into the configured baseline directory when the issue state is eligible.
func (manager *Manager) SyncBack(ctx context.Context, workspacePath string, issue domain.Issue) error {
	if err := manager.validatePath(workspacePath); err != nil {
		return err
	}
	if !manager.cfg.ShouldSyncBackState(issue.State) {
		return nil
	}
	destination := strings.TrimSpace(manager.cfg.Workspace.SyncBack.Path)
	if destination == "" {
		return nil
	}
	if err := manager.copyTree(ctx, workspacePath, destination, manager.cfg.Workspace.SyncBack.Excludes, "sync_back", issue); err != nil {
		return fmt.Errorf("%w: %v", ErrWorkspaceSyncBackFailed, err)
	}
	return nil
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

func (manager *Manager) seedWorkspace(ctx context.Context, workspacePath string, issue domain.Issue) error {
	source := strings.TrimSpace(manager.cfg.Workspace.Seed.Path)
	if source == "" {
		return nil
	}
	if err := manager.validatePath(workspacePath); err != nil {
		return err
	}
	return manager.copyTree(ctx, source, workspacePath, manager.cfg.Workspace.Seed.Excludes, "seed", issue)
}

func (manager *Manager) copyTree(ctx context.Context, sourceRoot string, destinationRoot string, excludes []string, operation string, issue domain.Issue) error {
	absoluteSource, err := resolveExistingPath(strings.TrimSpace(sourceRoot))
	if err != nil {
		return fmt.Errorf("resolve %s source: %w", operation, err)
	}
	absoluteDestination, err := resolveTargetPath(strings.TrimSpace(destinationRoot))
	if err != nil {
		return fmt.Errorf("resolve %s destination: %w", operation, err)
	}
	if absoluteSource == absoluteDestination {
		return fmt.Errorf("workspace %s source and destination must differ: %s", operation, absoluteSource)
	}
	sourceInfo, err := os.Stat(absoluteSource)
	if err != nil {
		return fmt.Errorf("workspace %s source path: %w", operation, err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("workspace %s source must be a directory: %s", operation, absoluteSource)
	}
	if err := os.MkdirAll(absoluteDestination, 0o755); err != nil {
		return fmt.Errorf("workspace %s destination path: %w", operation, err)
	}
	if info, err := os.Stat(absoluteDestination); err != nil {
		return fmt.Errorf("workspace %s destination path: %w", operation, err)
	} else if !info.IsDir() {
		return fmt.Errorf("workspace %s destination must be a directory: %s", operation, absoluteDestination)
	}

	manager.logger.Info("syncing workspace files",
		slog.String("operation", operation),
		slog.String("source", absoluteSource),
		slog.String("destination", absoluteDestination),
		slog.String("issue_identifier", issue.Identifier),
	)

	skipRelative := relativeIfWithin(absoluteSource, absoluteDestination)
	return filepath.WalkDir(absoluteSource, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if sourcePath == absoluteSource {
			return nil
		}

		relativePath, err := filepath.Rel(absoluteSource, sourcePath)
		if err != nil {
			return fmt.Errorf("workspace %s relative path: %w", operation, err)
		}
		relativePath = filepath.Clean(relativePath)
		if shouldSkipCopyEntry(relativePath, entry, skipRelative, excludes) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace %s does not support symlinks: %s", operation, sourcePath)
		}

		destinationPath := filepath.Join(absoluteDestination, relativePath)
		if err := ensurePathWithinRoot(absoluteDestination, destinationPath); err != nil {
			return fmt.Errorf("workspace %s destination path: %w", operation, err)
		}

		if info.IsDir() {
			return ensureDirectoryUnderRoot(absoluteDestination, destinationPath, directoryMode(info.Mode().Perm()))
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("workspace %s only supports regular files: %s", operation, sourcePath)
		}
		return copyRegularFile(absoluteDestination, sourcePath, destinationPath, info.Mode().Perm())
	})
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

func relativeIfWithin(root string, candidate string) string {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return ""
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return ""
	}
	return filepath.Clean(relative)
}

func shouldSkipCopyEntry(relativePath string, entry fs.DirEntry, skipRelative string, excludes []string) bool {
	normalizedRelative := filepath.ToSlash(filepath.Clean(relativePath))
	if normalizedRelative == "." {
		return false
	}
	if skipRelative != "" {
		normalizedSkip := filepath.ToSlash(filepath.Clean(skipRelative))
		if normalizedRelative == normalizedSkip || strings.HasPrefix(normalizedRelative, normalizedSkip+"/") {
			return true
		}
	}
	if entry.Name() == ".git" {
		return true
	}
	for _, exclude := range excludes {
		normalizedExclude := normalizeExcludePattern(exclude)
		if normalizedExclude == "" {
			continue
		}
		if normalizedRelative == normalizedExclude || strings.HasPrefix(normalizedRelative, normalizedExclude+"/") {
			return true
		}
	}
	return false
}

func normalizeExcludePattern(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Trim(trimmed, "/\\")
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return ""
	}
	return filepath.ToSlash(cleaned)
}

func ensurePathWithinRoot(root string, candidate string) error {
	cleanedRoot := filepath.Clean(root)
	cleanedCandidate := filepath.Clean(candidate)
	relative, err := filepath.Rel(cleanedRoot, cleanedCandidate)
	if err != nil {
		return fmt.Errorf("relative path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path %q escapes root %q", cleanedCandidate, cleanedRoot)
	}
	return nil
}

func resolveExistingPath(path string) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolutePath)
}

func resolveTargetPath(path string) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := absolutePath
	missing := make([]string, 0, 4)
	for {
		info, statErr := os.Lstat(current)
		if statErr == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("path component is not a directory: %s", current)
			}
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return resolved, nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", statErr
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func ensureDirectoryUnderRoot(root string, path string, mode fs.FileMode) error {
	if err := ensurePathWithinRoot(root, path); err != nil {
		return err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == "." {
		return nil
	}
	current := root
	for _, segment := range strings.Split(relative, string(os.PathSeparator)) {
		if segment == "" || segment == "." {
			continue
		}
		current = filepath.Join(current, segment)
		info, statErr := os.Lstat(current)
		if statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("path contains symlink: %s", current)
			}
			if !info.IsDir() {
				return fmt.Errorf("path component is not a directory: %s", current)
			}
			continue
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		if err := os.Mkdir(current, directoryMode(mode)); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	return nil
}

func directoryMode(mode fs.FileMode) fs.FileMode {
	if mode == 0 {
		return 0o755
	}
	return mode
}

func fileMode(mode fs.FileMode) fs.FileMode {
	if mode == 0 {
		return 0o644
	}
	return mode
}

func copyRegularFile(destinationRoot string, sourcePath string, destinationPath string, mode fs.FileMode) error {
	if err := ensureDirectoryUnderRoot(destinationRoot, filepath.Dir(destinationPath), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(destinationPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination path contains symlink: %s", destinationPath)
		}
		if info.IsDir() {
			return fmt.Errorf("destination path is a directory: %s", destinationPath)
		}
		if err := os.Remove(destinationPath); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	temporaryFile, err := os.CreateTemp(filepath.Dir(destinationPath), ".symphony-copy-*")
	if err != nil {
		return err
	}
	temporaryPath := temporaryFile.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if _, err := io.Copy(temporaryFile, sourceFile); err != nil {
		_ = temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Chmod(fileMode(mode)); err != nil {
		_ = temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destinationPath); err != nil {
		return err
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
