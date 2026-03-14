package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
)

func TestCreateForIssueSeedsWorkspace(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	baseline := t.TempDir()
	writeTestFile(t, filepath.Join(baseline, "seed.txt"), "seed")
	writeTestFile(t, filepath.Join(baseline, "nested", "child.txt"), "child")
	writeTestFile(t, filepath.Join(baseline, ".git", "config"), "ignored")
	writeTestFile(t, filepath.Join(baseline, "tmp", "cache.txt"), "ignored")

	manager := New(config.Config{
		Workspace: config.WorkspaceConfig{
			Root: workspaceRoot,
			Seed: config.WorkspaceSeedConfig{Path: baseline, Excludes: []string{"tmp"}},
		},
		Hooks: config.HooksConfig{TimeoutMs: 1_000},
	}, nil)

	workspace, err := manager.CreateForIssue(context.Background(), domain.Issue{ID: "TASK-1", Identifier: "TASK-1"})
	if err != nil {
		t.Fatalf("CreateForIssue: %v", err)
	}
	assertFileContent(t, filepath.Join(workspace.Path, "seed.txt"), "seed")
	assertFileContent(t, filepath.Join(workspace.Path, "nested", "child.txt"), "child")
	assertMissingPath(t, filepath.Join(workspace.Path, ".git", "config"))
	assertMissingPath(t, filepath.Join(workspace.Path, "tmp", "cache.txt"))
}

func TestSyncBackCopiesFilesAndSkipsExcludedEntries(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	baseline := t.TempDir()
	manager := New(config.Config{
		Workspace: config.WorkspaceConfig{
			Root: workspaceRoot,
			SyncBack: config.WorkspaceSyncBackConfig{
				Path:     baseline,
				OnStates: []string{"Done"},
				Excludes: []string{"tmp"},
			},
		},
		Hooks: config.HooksConfig{TimeoutMs: 1_000},
	}, nil)

	workspace, err := manager.CreateForIssue(context.Background(), domain.Issue{ID: "TASK-2", Identifier: "TASK-2"})
	if err != nil {
		t.Fatalf("CreateForIssue: %v", err)
	}
	writeTestFile(t, filepath.Join(workspace.Path, "result.txt"), "done")
	writeTestFile(t, filepath.Join(workspace.Path, ".git", "HEAD"), "ignored")
	writeTestFile(t, filepath.Join(workspace.Path, "tmp", "cache.txt"), "ignored")

	if err := manager.SyncBack(context.Background(), workspace.Path, domain.Issue{ID: "TASK-2", Identifier: "TASK-2", State: "Done"}); err != nil {
		t.Fatalf("SyncBack: %v", err)
	}
	assertFileContent(t, filepath.Join(baseline, "result.txt"), "done")
	assertMissingPath(t, filepath.Join(baseline, ".git", "HEAD"))
	assertMissingPath(t, filepath.Join(baseline, "tmp", "cache.txt"))
}

func TestSyncBackSkipsUnconfiguredState(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	baseline := t.TempDir()
	manager := New(config.Config{
		Workspace: config.WorkspaceConfig{
			Root:     workspaceRoot,
			SyncBack: config.WorkspaceSyncBackConfig{Path: baseline, OnStates: []string{"Done"}},
		},
		Hooks: config.HooksConfig{TimeoutMs: 1_000},
	}, nil)

	workspace, err := manager.CreateForIssue(context.Background(), domain.Issue{ID: "TASK-3", Identifier: "TASK-3"})
	if err != nil {
		t.Fatalf("CreateForIssue: %v", err)
	}
	writeTestFile(t, filepath.Join(workspace.Path, "result.txt"), "blocked")

	if err := manager.SyncBack(context.Background(), workspace.Path, domain.Issue{ID: "TASK-3", Identifier: "TASK-3", State: "Blocked"}); err != nil {
		t.Fatalf("SyncBack: %v", err)
	}
	assertMissingPath(t, filepath.Join(baseline, "result.txt"))
}

func TestValidatePathRejectsOutsideRoot(t *testing.T) {
	t.Parallel()
	manager := New(config.Config{Workspace: config.WorkspaceConfig{Root: t.TempDir()}}, nil)
	if err := manager.validatePath("/tmp"); err == nil {
		t.Fatalf("expected validation error")
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if got := string(payload); got != want {
		t.Fatalf("unexpected file content for %q: got %q want %q", path, got, want)
	}
}

func assertMissingPath(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %q to be absent", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("Stat(%q): %v", path, err)
	}
}
