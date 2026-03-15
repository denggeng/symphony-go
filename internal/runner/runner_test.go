package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/denggeng/symphony-go/internal/agent"
	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/prompt"
	"github.com/denggeng/symphony-go/internal/tracker"
	"github.com/denggeng/symphony-go/internal/workflow"
	"github.com/denggeng/symphony-go/internal/workspace"
)

func TestRunSeedsWorkspaceAndSyncsBackOnConfiguredTerminalState(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	baseline := t.TempDir()
	writeRunnerFile(t, filepath.Join(baseline, "seed.txt"), "seed")

	cfg := testRunnerConfig(workspaceRoot, baseline, baseline)
	backend := &stubBackend{
		onRunTurn: func(workspacePath string, issue domain.Issue) error {
			payload, err := os.ReadFile(filepath.Join(workspacePath, "seed.txt"))
			if err != nil {
				return err
			}
			if string(payload) != "seed" {
				return errors.New("seed file missing from workspace")
			}
			writeRunnerFile(t, filepath.Join(workspacePath, "result.txt"), "done")
			writeRunnerFile(t, filepath.Join(workspacePath, ".git", "HEAD"), "ignored")
			writeRunnerFile(t, filepath.Join(workspacePath, "tmp", "cache.txt"), "ignored")
			return nil
		},
	}
	tr := &stubTracker{issues: []domain.Issue{{ID: "TASK-1", Identifier: "TASK-1", State: "Done"}}}
	runner := New(cfg, nil, tr, workspace.New(cfg, nil), backend, prompt.New(workflow.Definition{}))

	result, err := runner.Run(context.Background(), domain.Issue{ID: "TASK-1", Identifier: "TASK-1", Title: "slice", State: "To Do"}, 0, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Continuation {
		t.Fatalf("expected run to finish without continuation")
	}
	assertRunnerFileContent(t, filepath.Join(baseline, "result.txt"), "done")
	assertRunnerMissingPath(t, filepath.Join(baseline, ".git", "HEAD"))
	assertRunnerMissingPath(t, filepath.Join(baseline, "tmp", "cache.txt"))
}

func TestRunDoesNotSyncBackForTerminalStatesOutsideOnStates(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	baseline := t.TempDir()
	cfg := testRunnerConfig(workspaceRoot, "", baseline)
	backend := &stubBackend{
		onRunTurn: func(workspacePath string, issue domain.Issue) error {
			writeRunnerFile(t, filepath.Join(workspacePath, "result.txt"), "blocked")
			return nil
		},
	}
	tr := &stubTracker{issues: []domain.Issue{{ID: "TASK-2", Identifier: "TASK-2", State: "Blocked"}}}
	runner := New(cfg, nil, tr, workspace.New(cfg, nil), backend, prompt.New(workflow.Definition{}))

	result, err := runner.Run(context.Background(), domain.Issue{ID: "TASK-2", Identifier: "TASK-2", Title: "slice", State: "To Do"}, 0, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Continuation {
		t.Fatalf("expected run to finish without continuation")
	}
	assertRunnerMissingPath(t, filepath.Join(baseline, "result.txt"))
}

func TestRunReturnsSyncBackFailure(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	syncBackTarget := filepath.Join(t.TempDir(), "baseline.txt")
	writeRunnerFile(t, syncBackTarget, "not-a-directory")

	cfg := testRunnerConfig(workspaceRoot, "", syncBackTarget)
	backend := &stubBackend{
		onRunTurn: func(workspacePath string, issue domain.Issue) error {
			writeRunnerFile(t, filepath.Join(workspacePath, "result.txt"), "done")
			return nil
		},
	}
	tr := &stubTracker{issues: []domain.Issue{{ID: "TASK-3", Identifier: "TASK-3", State: "Done"}}}
	runner := New(cfg, nil, tr, workspace.New(cfg, nil), backend, prompt.New(workflow.Definition{}))

	_, err := runner.Run(context.Background(), domain.Issue{ID: "TASK-3", Identifier: "TASK-3", Title: "slice", State: "To Do"}, 0, nil)
	if !errors.Is(err, workspace.ErrWorkspaceSyncBackFailed) {
		t.Fatalf("expected sync-back failure, got %v", err)
	}
}

func TestRunPersistsPromptsToLocalResultsWhenEnabled(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	resultsRoot := t.TempDir()
	cfg := testRunnerConfig(workspaceRoot, "", "")
	cfg.Local.ResultsDir = resultsRoot
	cfg.Agent.MaxTurns = 2
	cfg.Agent.PersistPromptsToResults = true

	backend := &stubBackend{}
	tr := &stubTracker{issueSequences: [][]domain.Issue{
		{{ID: "TASK-4", Identifier: "TASK-4", State: "In Progress"}},
		{{ID: "TASK-4", Identifier: "TASK-4", State: "Done"}},
	}}
	renderer := prompt.New(workflow.Definition{PromptTemplate: `Task {{ issue.identifier }} attempt {{ attempt }}`})
	runner := New(cfg, nil, tr, workspace.New(cfg, nil), backend, renderer)

	result, err := runner.Run(context.Background(), domain.Issue{ID: "TASK-4", Identifier: "TASK-4", Title: "slice", State: "To Do"}, 2, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Continuation {
		t.Fatalf("expected run to finish without continuation")
	}
	resultDir := filepath.Join(resultsRoot, "TASK-4")
	assertRunnerFileContent(t, filepath.Join(resultDir, "prompt.turn1.md"), "Task TASK-4 attempt 2")
	assertRunnerFileContent(t, filepath.Join(resultDir, "prompt.attempt2.turn1.md"), "Task TASK-4 attempt 2")
	assertRunnerFileContent(t, filepath.Join(resultDir, "prompt.turn2.md"), prompt.ContinuationPrompt(2, 2))
	assertRunnerFileContent(t, filepath.Join(resultDir, "prompt.attempt2.turn2.md"), prompt.ContinuationPrompt(2, 2))
}

type stubTracker struct {
	issues         []domain.Issue
	issueSequences [][]domain.Issue
	fetchByIDCalls int
}

func (stub *stubTracker) Kind() string {
	return "local"
}

func (stub *stubTracker) FetchCandidateIssues(context.Context) ([]domain.Issue, error) {
	return nil, nil
}

func (stub *stubTracker) FetchIssuesByIDs(context.Context, []string) ([]domain.Issue, error) {
	if len(stub.issueSequences) > 0 {
		index := stub.fetchByIDCalls
		if index >= len(stub.issueSequences) {
			index = len(stub.issueSequences) - 1
		}
		stub.fetchByIDCalls++
		return append([]domain.Issue(nil), stub.issueSequences[index]...), nil
	}
	return append([]domain.Issue(nil), stub.issues...), nil
}

func (stub *stubTracker) FetchIssuesByStates(context.Context, []string) ([]domain.Issue, error) {
	return nil, nil
}

func (stub *stubTracker) CreateComment(context.Context, string, string) error {
	return nil
}

func (stub *stubTracker) UpdateIssueState(context.Context, string, string) error {
	return nil
}

func (stub *stubTracker) RawAPI(context.Context, tracker.RawRequest) (tracker.RawResponse, error) {
	return tracker.RawResponse{}, nil
}

type stubBackend struct {
	workspacePath string
	onRunTurn     func(workspacePath string, issue domain.Issue) error
}

func (stub *stubBackend) StartSession(_ context.Context, workspacePath string) (agent.Session, error) {
	stub.workspacePath = workspacePath
	return "session", nil
}

func (stub *stubBackend) RunTurn(ctx context.Context, session agent.Session, issue domain.Issue, prompt string, onEvent agent.EventHandler) (agent.TurnResult, error) {
	if stub.onRunTurn != nil {
		if err := stub.onRunTurn(stub.workspacePath, issue); err != nil {
			return agent.TurnResult{}, err
		}
	}
	return agent.TurnResult{SessionID: "session"}, nil
}

func (stub *stubBackend) StopSession(context.Context, agent.Session) error {
	return nil
}

func testRunnerConfig(workspaceRoot string, seedPath string, syncBackPath string) config.Config {
	return config.Config{
		Tracker: config.TrackerConfig{Kind: "local"},
		Local: config.LocalConfig{
			ActiveStates:   []string{"To Do", "In Progress"},
			TerminalStates: []string{"Done", "Blocked"},
		},
		Workspace: config.WorkspaceConfig{
			Root: workspaceRoot,
			Seed: config.WorkspaceSeedConfig{Path: seedPath, Excludes: []string{"tmp"}},
			SyncBack: config.WorkspaceSyncBackConfig{
				Path:     syncBackPath,
				OnStates: []string{"Done"},
				Excludes: []string{"tmp"},
			},
		},
		Hooks: config.HooksConfig{TimeoutMs: 1_000},
		Agent: config.AgentConfig{MaxTurns: 1},
	}
}

func writeRunnerFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func assertRunnerFileContent(t *testing.T, path string, want string) {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if got := string(payload); got != want {
		t.Fatalf("unexpected file content for %q: got %q want %q", path, got, want)
	}
}

func assertRunnerMissingPath(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %q to be absent", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("Stat(%q): %v", path, err)
	}
}
