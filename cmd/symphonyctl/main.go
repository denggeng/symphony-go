package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/denggeng/symphony-go/internal/inspect"
	"github.com/denggeng/symphony-go/internal/workflow"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return flag.ErrHelp
	}
	if args[0] != "render-prompt" {
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
	return runRenderPrompt(args[1:], stdout, stderr)
}

func runRenderPrompt(args []string, stdout io.Writer, stderr io.Writer) error {
	flags := flag.NewFlagSet("render-prompt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	workflowPath := flags.String("workflow", workflow.DefaultPath(), "Path to WORKFLOW.md")
	taskID := flags.String("task-id", "", "Local task identifier")
	workspacePath := flags.String("workspace", "", "Workspace path; basename is used as the task identifier")
	turn := flags.Int("turn", 1, "Turn number to render")
	attempt := flags.Int("attempt", 1, "Attempt number to render")
	outputPath := flags.String("output", "", "Optional file path to write the rendered prompt")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*taskID) == "" && strings.TrimSpace(*workspacePath) == "" {
		return fmt.Errorf("provide -task-id or -workspace")
	}
	if strings.TrimSpace(*taskID) != "" && strings.TrimSpace(*workspacePath) != "" {
		return fmt.Errorf("use only one of -task-id or -workspace")
	}

	text, err := inspect.RenderLocalTaskPrompt(inspect.PromptRenderOptions{
		WorkflowPath:  *workflowPath,
		TaskID:        *taskID,
		WorkspacePath: *workspacePath,
		Turn:          *turn,
		Attempt:       *attempt,
	})
	if err != nil {
		return err
	}
	if target := strings.TrimSpace(*outputPath); target != "" {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
		if err := os.WriteFile(target, []byte(text+"\n"), 0o600); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
	_, err = fmt.Fprintln(stdout, text)
	return err
}

func printUsage(output io.Writer) {
	_, _ = fmt.Fprintf(output, "Usage:\n  %s render-prompt [flags]\n\n", filepath.Base(os.Args[0]))
	_, _ = fmt.Fprintln(output, "Commands:")
	_, _ = fmt.Fprintln(output, "  render-prompt   Reconstruct the prompt Symphony sends for a local task")
}
