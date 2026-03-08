package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/denggeng/symphony-go/internal/agent/codexappserver"
	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/orchestrator"
	"github.com/denggeng/symphony-go/internal/prompt"
	"github.com/denggeng/symphony-go/internal/runner"
	"github.com/denggeng/symphony-go/internal/server"
	"github.com/denggeng/symphony-go/internal/tools"
	"github.com/denggeng/symphony-go/internal/tracker"
	jira "github.com/denggeng/symphony-go/internal/tracker/jira"
	"github.com/denggeng/symphony-go/internal/workflow"
	"github.com/denggeng/symphony-go/internal/workspace"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	workflowPath := flag.String("workflow", workflow.DefaultPath(), "Path to WORKFLOW.md")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(*logLevel)}))
	loadedWorkflow, err := workflow.Load(*workflowPath)
	if err != nil {
		return err
	}
	runtimeConfig, err := config.FromWorkflow(loadedWorkflow)
	if err != nil {
		return err
	}

	var issueTracker tracker.Tracker
	switch runtimeConfig.Tracker.Kind {
	case "jira":
		issueTracker = jira.New(runtimeConfig, logger)
	default:
		return fmt.Errorf("unsupported tracker kind: %s", runtimeConfig.Tracker.Kind)
	}

	workspaceManager := workspace.New(runtimeConfig, logger)
	toolExecutor := tools.New(issueTracker)
	backend := codexappserver.New(runtimeConfig, logger, toolExecutor)
	renderer := prompt.New(loadedWorkflow)
	runner := runner.New(runtimeConfig, logger, issueTracker, workspaceManager, backend, renderer)
	controller := orchestrator.New(orchestrator.Options{Logger: logger, Workflow: loadedWorkflow, Config: runtimeConfig, Tracker: issueTracker, Workspace: workspaceManager, Runner: runner})
	apiServer := server.New(server.Options{Logger: logger, Host: runtimeConfig.Server.Host, Port: runtimeConfig.Server.Port, Controller: controller})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	controller.Start(ctx)
	serverErrCh := make(chan error, 1)
	go func() { serverErrCh <- apiServer.Start() }()

	logger.Info("symphony-go started", slog.String("workflow_path", loadedWorkflow.Path), slog.String("tracker_kind", runtimeConfig.Tracker.Kind), slog.String("listen_addr", apiServer.Addr()))

	select {
	case err := <-serverErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}

	controller.Stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Info("symphony-go shutting down")
	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return nil
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
