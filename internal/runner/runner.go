package runner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/denggeng/symphony-go/internal/agent"
	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/prompt"
	"github.com/denggeng/symphony-go/internal/tracker"
	"github.com/denggeng/symphony-go/internal/workspace"
)

type Result struct {
	WorkspacePath string
	Continuation  bool
	Turns         int
}

type Runner struct {
	cfg       config.Config
	logger    *slog.Logger
	tracker   tracker.Tracker
	workspace *workspace.Manager
	backend   agent.Backend
	renderer  *prompt.Renderer
}

func New(cfg config.Config, logger *slog.Logger, tr tracker.Tracker, workspaceManager *workspace.Manager, backend agent.Backend, renderer *prompt.Renderer) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{cfg: cfg, logger: logger, tracker: tr, workspace: workspaceManager, backend: backend, renderer: renderer}
}

func (runner *Runner) Run(ctx context.Context, issue domain.Issue, attempt int, onEvent agent.EventHandler) (Result, error) {
	ws, err := runner.workspace.CreateForIssue(ctx, issue)
	if err != nil {
		return Result{}, err
	}
	defer runner.workspace.RunAfterRun(context.Background(), ws.Path, issue)

	if err := runner.workspace.RunBeforeRun(ctx, ws.Path, issue); err != nil {
		return Result{}, err
	}

	session, err := runner.backend.StartSession(ctx, ws.Path)
	if err != nil {
		return Result{}, err
	}
	defer runner.backend.StopSession(context.Background(), session)

	maxTurns := runner.cfg.Agent.MaxTurns
	for turnNumber := 1; turnNumber <= maxTurns; turnNumber++ {
		turnPrompt := runner.buildPrompt(issue, attempt, turnNumber, maxTurns)
		runner.persistPrompt(issue, attempt, turnNumber, turnPrompt)
		if _, err := runner.backend.RunTurn(ctx, session, issue, turnPrompt, onEvent); err != nil {
			return Result{WorkspacePath: ws.Path, Turns: turnNumber}, err
		}

		refreshedIssue, continueIssue, err := runner.shouldContinue(ctx, issue)
		if err != nil {
			return Result{WorkspacePath: ws.Path, Turns: turnNumber}, err
		}
		issue = refreshedIssue
		if !continueIssue {
			if err := runner.workspace.SyncBack(context.Background(), ws.Path, refreshedIssue); err != nil {
				return Result{WorkspacePath: ws.Path, Turns: turnNumber, Continuation: false}, err
			}
			return Result{WorkspacePath: ws.Path, Turns: turnNumber, Continuation: false}, nil
		}
		if turnNumber == maxTurns {
			return Result{WorkspacePath: ws.Path, Turns: turnNumber, Continuation: true}, nil
		}
	}

	return Result{WorkspacePath: ws.Path, Turns: maxTurns, Continuation: true}, nil
}

func (runner *Runner) buildPrompt(issue domain.Issue, attempt int, turnNumber int, maxTurns int) string {
	return prompt.TurnPrompt(runner.renderer, issue, attempt, turnNumber, maxTurns)
}

func (runner *Runner) shouldContinue(ctx context.Context, issue domain.Issue) (domain.Issue, bool, error) {
	issues, err := runner.tracker.FetchIssuesByIDs(ctx, []string{issue.ID})
	if err != nil {
		return issue, false, err
	}
	if len(issues) == 0 {
		return issue, false, fmt.Errorf("tracker could not refresh issue %s", issue.ID)
	}
	refreshed := issues[0]
	return refreshed, runner.cfg.IsActiveState(refreshed.State), nil
}
