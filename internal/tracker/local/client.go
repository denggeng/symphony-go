package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/tracker"
	"github.com/denggeng/symphony-go/internal/workflow"
)

var errUnsupportedRawAPI = errors.New("local tracker does not support raw api requests")

// Client implements local Markdown task tracking backed by inbox/archive folders.
type Client struct {
	cfg    config.Config
	logger *slog.Logger
}

type taskRecord struct {
	Issue       domain.Issue
	SourcePath  string
	ArchivePath string
	Summary     string
	Comments    []taskComment
}

type taskState struct {
	ID          string        `json:"id"`
	Title       string        `json:"title,omitempty"`
	State       string        `json:"state,omitempty"`
	Summary     string        `json:"summary,omitempty"`
	SourcePath  string        `json:"source_path,omitempty"`
	ArchivePath string        `json:"archive_path,omitempty"`
	UpdatedAt   time.Time     `json:"updated_at,omitempty"`
	Comments    []taskComment `json:"comments,omitempty"`
}

type taskComment struct {
	Timestamp time.Time `json:"timestamp"`
	Body      string    `json:"body"`
}

type taskFrontMatter struct {
	ID    string
	Title string
	State string
}

// New creates a local tracker client from runtime config.
func New(cfg config.Config, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{cfg: cfg, logger: logger}
}

func (client *Client) Kind() string { return "local" }

func (client *Client) FetchCandidateIssues(_ context.Context) ([]domain.Issue, error) {
	records, err := client.scanTasks(false)
	if err != nil {
		return nil, err
	}
	issues := make([]domain.Issue, 0, len(records))
	for _, record := range records {
		if client.cfg.IsActiveState(record.Issue.State) {
			issues = append(issues, record.Issue)
		}
	}
	domain.SortIssues(issues)
	return issues, nil
}

func (client *Client) FetchIssuesByIDs(_ context.Context, ids []string) ([]domain.Issue, error) {
	records, err := client.scanTasks(true)
	if err != nil {
		return nil, err
	}
	issues := make([]domain.Issue, 0, len(ids))
	for _, id := range ids {
		record, ok := records[strings.TrimSpace(id)]
		if ok {
			issues = append(issues, record.Issue)
		}
	}
	return issues, nil
}

func (client *Client) FetchIssuesByStates(_ context.Context, states []string) ([]domain.Issue, error) {
	if len(states) == 0 {
		return nil, nil
	}
	records, err := client.scanTasks(true)
	if err != nil {
		return nil, err
	}
	issues := make([]domain.Issue, 0, len(records))
	for _, record := range records {
		if record.Issue.MatchesState(states) {
			issues = append(issues, record.Issue)
		}
	}
	domain.SortIssues(issues)
	return issues, nil
}

func (client *Client) CreateComment(_ context.Context, issueIdentifier string, body string) error {
	record, err := client.taskByIdentifier(issueIdentifier)
	if err != nil {
		return err
	}
	comment := taskComment{Timestamp: time.Now().UTC(), Body: strings.TrimSpace(body)}
	if comment.Body == "" {
		return fmt.Errorf("comment body must not be empty")
	}
	state, err := client.loadTaskState(record.Issue.Identifier)
	if err != nil {
		return err
	}
	state.ID = record.Issue.Identifier
	state.Title = record.Issue.Title
	state.State = firstNonEmpty(state.State, record.Issue.State)
	state.SourcePath = firstNonEmpty(state.SourcePath, record.SourcePath)
	state.ArchivePath = firstNonEmpty(state.ArchivePath, record.ArchivePath)
	state.UpdatedAt = comment.Timestamp
	state.Comments = append(state.Comments, comment)
	if err := client.writeTaskState(state); err != nil {
		return err
	}
	return client.writeComments(record.Issue.Identifier, state.Comments)
}

func (client *Client) UpdateIssueState(ctx context.Context, issueIdentifier string, stateName string) error {
	return client.UpdateTask(ctx, issueIdentifier, tracker.TaskUpdate{State: stateName})
}

func (client *Client) UpdateTask(_ context.Context, issueIdentifier string, update tracker.TaskUpdate) error {
	identifier := strings.TrimSpace(issueIdentifier)
	if identifier == "" {
		return fmt.Errorf("task identifier must not be empty")
	}
	stateName := strings.TrimSpace(update.State)
	if stateName == "" {
		return fmt.Errorf("task state must not be empty")
	}
	if !client.cfg.IsActiveState(stateName) && !client.cfg.IsTerminalState(stateName) {
		return fmt.Errorf("unsupported task state %q", stateName)
	}

	record, err := client.taskByIdentifier(identifier)
	if err != nil {
		return err
	}
	currentPath := record.SourcePath
	if currentPath == "" {
		currentPath = record.ArchivePath
	}
	if currentPath == "" {
		return fmt.Errorf("task %q has no source file", identifier)
	}

	state, err := client.loadTaskState(record.Issue.Identifier)
	if err != nil {
		return err
	}
	state.ID = record.Issue.Identifier
	state.Title = record.Issue.Title
	state.State = stateName
	if summary := strings.TrimSpace(update.Summary); summary != "" {
		state.Summary = summary
	}
	state.UpdatedAt = time.Now().UTC()
	state.Comments = append([]taskComment(nil), state.Comments...)

	targetPath := currentPath
	baseName := filepath.Base(currentPath)
	if baseName == "." || baseName == string(os.PathSeparator) || baseName == "" {
		baseName = safeIdentifier(record.Issue.Identifier) + ".md"
	}
	if client.cfg.IsTerminalState(stateName) {
		targetPath = filepath.Join(client.cfg.Local.ArchiveDir, baseName)
		if currentPath != targetPath {
			if err := moveFile(currentPath, targetPath); err != nil {
				return err
			}
		}
		state.SourcePath = ""
		state.ArchivePath = targetPath
	} else {
		targetPath = filepath.Join(client.cfg.Local.InboxDir, baseName)
		if currentPath != targetPath {
			if err := moveFile(currentPath, targetPath); err != nil {
				return err
			}
		}
		state.SourcePath = targetPath
		state.ArchivePath = ""
	}

	if err := client.writeTaskState(state); err != nil {
		return err
	}
	if err := client.writeTaskMetadata(state); err != nil {
		return err
	}
	if strings.TrimSpace(state.Summary) != "" {
		if err := client.writeTaskSummary(record.Issue.Identifier, record.Issue.Title, state); err != nil {
			return err
		}
	}
	if len(state.Comments) > 0 {
		if err := client.writeComments(record.Issue.Identifier, state.Comments); err != nil {
			return err
		}
	}
	return nil
}

func (client *Client) RawAPI(context.Context, tracker.RawRequest) (tracker.RawResponse, error) {
	return tracker.RawResponse{}, errUnsupportedRawAPI
}

func (client *Client) taskByIdentifier(identifier string) (*taskRecord, error) {
	records, err := client.scanTasks(true)
	if err != nil {
		return nil, err
	}
	record, ok := records[strings.TrimSpace(identifier)]
	if !ok {
		return nil, fmt.Errorf("local task %q not found", identifier)
	}
	return record, nil
}

func (client *Client) scanTasks(includeArchive bool) (map[string]*taskRecord, error) {
	if err := client.ensureDirs(); err != nil {
		return nil, err
	}
	records := map[string]*taskRecord{}
	if err := client.readTaskDir(client.cfg.Local.InboxDir, false, records); err != nil {
		return nil, err
	}
	if includeArchive {
		if err := client.readTaskDir(client.cfg.Local.ArchiveDir, true, records); err != nil {
			return nil, err
		}
	}
	return records, nil
}

func (client *Client) readTaskDir(dir string, archived bool, records map[string]*taskRecord) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read local task dir %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".md" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		record, err := client.parseTaskFile(path, archived)
		if err != nil {
			client.logger.Warn("skip invalid local task", slog.String("path", path), slog.Any("error", err))
			continue
		}
		existing := records[record.Issue.Identifier]
		if existing == nil || (existing.SourcePath == "" && record.SourcePath != "") {
			records[record.Issue.Identifier] = record
		}
	}
	return nil
}

func (client *Client) parseTaskFile(path string, archived bool) (*taskRecord, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read local task file: %w", err)
	}
	definition, err := workflow.Parse(path, string(content))
	if err != nil {
		return nil, err
	}
	metadata := decodeFrontMatter(definition.Config)
	identifier := strings.TrimSpace(metadata.ID)
	if identifier == "" {
		identifier = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if identifier == "" {
		identifier = "task"
	}
	title := firstNonEmpty(strings.TrimSpace(metadata.Title), firstContentLine(definition.Prompt), identifier)
	stateName := firstNonEmpty(strings.TrimSpace(metadata.State), firstConfiguredActiveState(client.cfg), "To Do")
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat local task file: %w", err)
	}
	updatedAt := info.ModTime().UTC()
	state, err := client.loadTaskState(identifier)
	if err != nil {
		return nil, err
	}
	if state.Title != "" {
		title = state.Title
	}
	if state.State != "" {
		stateName = state.State
	}
	if !state.UpdatedAt.IsZero() {
		updatedAt = state.UpdatedAt
	}
	createdAt := info.ModTime().UTC()
	issue := domain.Issue{
		ID:          identifier,
		Identifier:  identifier,
		Title:       title,
		Description: strings.TrimSpace(definition.Prompt),
		State:       stateName,
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
	}
	record := &taskRecord{Issue: issue, Summary: state.Summary, Comments: append([]taskComment(nil), state.Comments...)}
	if archived {
		record.ArchivePath = path
	} else {
		record.SourcePath = path
	}
	if state.SourcePath != "" && record.SourcePath == "" && !archived {
		record.SourcePath = state.SourcePath
	}
	if state.ArchivePath != "" && record.ArchivePath == "" && archived {
		record.ArchivePath = state.ArchivePath
	}
	return record, nil
}

func (client *Client) ensureDirs() error {
	for _, dir := range []string{client.cfg.Local.InboxDir, client.cfg.Local.StateDir, client.cfg.Local.ArchiveDir, client.cfg.Local.ResultsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create local task dir %s: %w", dir, err)
		}
	}
	return nil
}

func (client *Client) loadTaskState(identifier string) (taskState, error) {
	path := client.taskStatePath(identifier)
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return taskState{}, nil
		}
		return taskState{}, fmt.Errorf("read local task state: %w", err)
	}
	var state taskState
	if err := json.Unmarshal(payload, &state); err != nil {
		return taskState{}, fmt.Errorf("decode local task state: %w", err)
	}
	return state, nil
}

func (client *Client) writeTaskState(state taskState) error {
	if err := client.ensureDirs(); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode local task state: %w", err)
	}
	if err := os.WriteFile(client.taskStatePath(state.ID), payload, 0o600); err != nil {
		return fmt.Errorf("write local task state: %w", err)
	}
	return nil
}

func (client *Client) writeTaskMetadata(state taskState) error {
	metadataPath := filepath.Join(client.resultDir(state.ID), "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0o755); err != nil {
		return fmt.Errorf("create local task metadata dir: %w", err)
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode local task metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, payload, 0o600); err != nil {
		return fmt.Errorf("write local task metadata: %w", err)
	}
	return nil
}

func (client *Client) writeTaskSummary(identifier string, title string, state taskState) error {
	summaryPath := filepath.Join(client.resultDir(identifier), "summary.md")
	if err := os.MkdirAll(filepath.Dir(summaryPath), 0o755); err != nil {
		return fmt.Errorf("create local task summary dir: %w", err)
	}
	content := strings.TrimSpace(fmt.Sprintf(`# %s

- Task: %s
- State: %s
- Updated: %s

## Summary

%s
`, firstNonEmpty(title, identifier), identifier, state.State, state.UpdatedAt.Format(time.RFC3339), state.Summary))
	if err := os.WriteFile(summaryPath, []byte(content+"\n"), 0o600); err != nil {
		return fmt.Errorf("write local task summary: %w", err)
	}
	return nil
}

func (client *Client) writeComments(identifier string, comments []taskComment) error {
	commentsPath := filepath.Join(client.resultDir(identifier), "comments.md")
	if err := os.MkdirAll(filepath.Dir(commentsPath), 0o755); err != nil {
		return fmt.Errorf("create local task comments dir: %w", err)
	}
	builder := strings.Builder{}
	builder.WriteString("# Comments\n\n")
	for _, comment := range comments {
		builder.WriteString("## ")
		builder.WriteString(comment.Timestamp.Format(time.RFC3339))
		builder.WriteString("\n\n")
		builder.WriteString(strings.TrimSpace(comment.Body))
		builder.WriteString("\n\n")
	}
	if err := os.WriteFile(commentsPath, []byte(builder.String()), 0o600); err != nil {
		return fmt.Errorf("write local task comments: %w", err)
	}
	return nil
}

func (client *Client) taskStatePath(identifier string) string {
	return filepath.Join(client.cfg.Local.StateDir, safeIdentifier(identifier)+".json")
}

func (client *Client) resultDir(identifier string) string {
	return filepath.Join(client.cfg.Local.ResultsDir, safeIdentifier(identifier))
}

func decodeFrontMatter(values map[string]any) taskFrontMatter {
	frontMatter := taskFrontMatter{}
	if values == nil {
		return frontMatter
	}
	frontMatter.ID, _ = values["id"].(string)
	frontMatter.Title, _ = values["title"].(string)
	frontMatter.State, _ = values["state"].(string)
	return frontMatter
}

func firstConfiguredActiveState(cfg config.Config) string {
	states := cfg.ActiveStates()
	if len(states) == 0 {
		return ""
	}
	return states[0]
}

func firstContentLine(body string) string {
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func safeIdentifier(identifier string) string {
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

func moveFile(source string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create local task target dir: %w", err)
	}
	if source == target {
		return nil
	}
	if err := os.Rename(source, target); err != nil {
		return fmt.Errorf("move local task file %s -> %s: %w", source, target, err)
	}
	return nil
}

var _ tracker.Tracker = (*Client)(nil)
var _ tracker.TaskUpdater = (*Client)(nil)
