package runner

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/denggeng/symphony-go/internal/domain"
)

func (runner *Runner) persistPrompt(issue domain.Issue, attempt int, turnNumber int, renderedPrompt string) {
	if !runner.cfg.Agent.PersistPromptsToResults {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(runner.cfg.Tracker.Kind), "local") {
		return
	}
	resultsRoot := strings.TrimSpace(runner.cfg.Local.ResultsDir)
	if resultsRoot == "" {
		return
	}
	resultDir := filepath.Join(resultsRoot, safePromptIdentifier(issue.Identifier))
	if err := os.MkdirAll(resultDir, 0o755); err != nil {
		runner.logger.Warn("persist prompt directory failed",
			slog.String("issue_identifier", issue.Identifier),
			slog.Int("attempt", normalizedPromptAttempt(attempt)),
			slog.Int("turn", turnNumber),
			slog.String("path", resultDir),
			slog.Any("error", err),
		)
		return
	}
	for _, path := range []string{
		filepath.Join(resultDir, fmt.Sprintf("prompt.turn%d.md", turnNumber)),
		filepath.Join(resultDir, fmt.Sprintf("prompt.attempt%d.turn%d.md", normalizedPromptAttempt(attempt), turnNumber)),
	} {
		if err := os.WriteFile(path, []byte(renderedPrompt), 0o600); err != nil {
			runner.logger.Warn("persist prompt file failed",
				slog.String("issue_identifier", issue.Identifier),
				slog.Int("attempt", normalizedPromptAttempt(attempt)),
				slog.Int("turn", turnNumber),
				slog.String("path", path),
				slog.Any("error", err),
			)
		}
	}
}

func normalizedPromptAttempt(attempt int) int {
	if attempt < 1 {
		return 1
	}
	return attempt
}

func safePromptIdentifier(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		identifier = "task"
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
