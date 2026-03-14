package local

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/tracker"
)

func testConfig(root string) config.Config {
	return config.Config{
		Tracker: config.TrackerConfig{Kind: "local"},
		Local: config.LocalConfig{
			InboxDir:       filepath.Join(root, "inbox"),
			StateDir:       filepath.Join(root, "state"),
			ArchiveDir:     filepath.Join(root, "archive"),
			ResultsDir:     filepath.Join(root, "results"),
			ActiveStates:   []string{"To Do", "In Progress"},
			TerminalStates: []string{"Done", "Blocked"},
		},
	}
}

func writeTaskFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
}

func issueIdentifiers(issues []domain.Issue) []string {
	identifiers := make([]string, 0, len(issues))
	for _, issue := range issues {
		identifiers = append(identifiers, issue.Identifier)
	}
	return identifiers
}

func testConfigWithReviewed(root string) config.Config {
	cfg := testConfig(root)
	cfg.Local.TerminalStates = []string{"Done", "Reviewed", "Blocked"}
	return cfg
}

func TestFetchCandidateIssuesReadsInboxMarkdown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	client := New(testConfig(root), slog.Default())
	writeTaskFile(t, filepath.Join(root, "inbox", "hello-endpoint.md"), `---
title: Add hello endpoint
state: To Do
---
Add a /hello endpoint and test it.
`)

	issues, err := client.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("fetch candidate issues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("unexpected candidate count: %d", len(issues))
	}
	if issues[0].Identifier != "hello-endpoint" {
		t.Fatalf("unexpected identifier: %q", issues[0].Identifier)
	}
	if issues[0].Title != "Add hello endpoint" {
		t.Fatalf("unexpected title: %q", issues[0].Title)
	}
	if issues[0].Description != "Add a /hello endpoint and test it." {
		t.Fatalf("unexpected description: %q", issues[0].Description)
	}
}

func TestFetchCandidateIssuesReadsLaneAndReviewOf(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	client := New(testConfig(root), slog.Default())
	writeTaskFile(t, filepath.Join(root, "inbox", "review-api.md"), `---
id: review-api
state: To Do
lane: review
review_of: impl-api
---
Review the API implementation slice.
`)

	issues, err := client.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("fetch candidate issues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("unexpected candidate count: %d", len(issues))
	}
	if issues[0].Lane != "review" {
		t.Fatalf("unexpected lane: %q", issues[0].Lane)
	}
	if issues[0].ReviewOf != "impl-api" {
		t.Fatalf("unexpected review_of: %q", issues[0].ReviewOf)
	}
}

func TestFetchCandidateIssuesOrdersByPriorityThenOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	client := New(testConfig(root), slog.Default())

	writeTaskFile(t, filepath.Join(root, "inbox", "fallback.md"), `---
title: Fallback cleanup
state: To Do
---
Tidy up deferred work.
`)
	writeTaskFile(t, filepath.Join(root, "inbox", "backend.md"), `---
id: backend
state: To Do
priority: 2
order: 20
---
Implement the backend path.
`)
	writeTaskFile(t, filepath.Join(root, "inbox", "bootstrap.md"), `---
id: bootstrap
state: To Do
priority: 1
order: 30
---
Bootstrap the service.
`)
	writeTaskFile(t, filepath.Join(root, "inbox", "schema.md"), `---
id: schema
state: To Do
priority: 1
order: 10
---
Define the schema first.
`)

	issues, err := client.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("fetch candidate issues: %v", err)
	}

	want := []string{"schema", "bootstrap", "backend", "fallback"}
	if got := issueIdentifiers(issues); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected issue order: got %v want %v", got, want)
	}
}

func TestFetchCandidateIssuesBlocksUntilDependenciesAreDone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	client := New(testConfig(root), slog.Default())

	writeTaskFile(t, filepath.Join(root, "inbox", "setup-db.md"), `---
id: setup-db
state: To Do
priority: 1
order: 10
---
Prepare database migrations.
`)
	writeTaskFile(t, filepath.Join(root, "inbox", "api.md"), `---
id: api
title: Build API layer
state: To Do
priority: 1
order: 20
depends_on:
  - setup-db
---
Build the API after the database task is complete.
`)

	issues, err := client.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("fetch candidate issues: %v", err)
	}
	if got, want := issueIdentifiers(issues), []string{"setup-db"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected ready issue set before dependency completion: got %v want %v", got, want)
	}

	err = client.UpdateTask(context.Background(), "setup-db", tracker.TaskUpdate{State: "Done", Summary: "Database setup complete."})
	if err != nil {
		t.Fatalf("mark dependency done: %v", err)
	}

	issues, err = client.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("fetch candidate issues after dependency completion: %v", err)
	}
	if got, want := issueIdentifiers(issues), []string{"api"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected ready issue set after dependency completion: got %v want %v", got, want)
	}
}

func TestFetchCandidateIssuesTreatsReviewedDependenciesAsSatisfied(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	client := New(testConfigWithReviewed(root), slog.Default())

	writeTaskFile(t, filepath.Join(root, "inbox", "review.md"), `---
id: review
state: To Do
---
Review the provider wiring.
`)
	writeTaskFile(t, filepath.Join(root, "inbox", "fix.md"), `---
id: fix
state: To Do
depends_on: review
---
Apply the follow-up implementation after review is complete.
`)

	if err := client.UpdateTask(context.Background(), "review", tracker.TaskUpdate{State: "Reviewed", Summary: "Review complete with follow-up notes."}); err != nil {
		t.Fatalf("mark dependency reviewed: %v", err)
	}

	issues, err := client.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("fetch candidate issues after reviewed dependency: %v", err)
	}
	if got, want := issueIdentifiers(issues), []string{"fix"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected ready issue set after reviewed dependency: got %v want %v", got, want)
	}
}

func TestFetchCandidateIssuesKeepsDependentsBlockedOnBlockedDependencies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	client := New(testConfig(root), slog.Default())

	writeTaskFile(t, filepath.Join(root, "inbox", "setup-db.md"), `---
id: setup-db
state: To Do
---
Prepare database migrations.
`)
	writeTaskFile(t, filepath.Join(root, "inbox", "api.md"), `---
id: api
state: To Do
depends_on: setup-db
---
Build the API after the database task is complete.
`)

	err := client.UpdateTask(context.Background(), "setup-db", tracker.TaskUpdate{State: "Blocked", Summary: "Database migration is blocked."})
	if err != nil {
		t.Fatalf("mark dependency blocked: %v", err)
	}

	issues, err := client.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("fetch candidate issues: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no ready issues when dependency is blocked, got %v", issueIdentifiers(issues))
	}
}

func TestFetchIssuesByStatesIncludesBlockedDependencies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	client := New(testConfig(root), slog.Default())

	writeTaskFile(t, filepath.Join(root, "inbox", "setup-db.md"), `---
id: setup-db
state: To Do
---
Prepare database migrations.
`)
	writeTaskFile(t, filepath.Join(root, "inbox", "api.md"), `---
id: api
state: To Do
depends_on:
  - setup-db
  - auth
---
Build the API after the database task is complete.
`)
	writeTaskFile(t, filepath.Join(root, "archive", "auth.md"), `---
id: auth
state: Done
---
Bootstrap authentication.
`)

	issues, err := client.FetchIssuesByStates(context.Background(), []string{"To Do"})
	if err != nil {
		t.Fatalf("fetch issues by states: %v", err)
	}
	issueByID := make(map[string]domain.Issue, len(issues))
	for _, issue := range issues {
		issueByID[issue.Identifier] = issue
	}

	api := issueByID["api"]
	if got, want := api.Dependencies, []string{"setup-db", "auth"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected dependencies: got %v want %v", got, want)
	}
	if got, want := api.BlockedBy, []string{"setup-db"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected blocked_by: got %v want %v", got, want)
	}
	if blocked := issueByID["setup-db"].BlockedBy; len(blocked) != 0 {
		t.Fatalf("expected dependency task to have no blockers, got %v", blocked)
	}
}

func TestUpdateTaskArchivesTerminalTaskAndWritesResults(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	client := New(testConfig(root), slog.Default())
	inboxPath := filepath.Join(root, "inbox", "hello-endpoint.md")
	writeTaskFile(t, inboxPath, `---
id: hello-endpoint
title: Add hello endpoint
state: To Do
---
Implement the endpoint.
`)

	err := client.UpdateTask(context.Background(), "hello-endpoint", tracker.TaskUpdate{State: "Done", Summary: "Implemented endpoint and ran go test ./..."})
	if err != nil {
		t.Fatalf("update task: %v", err)
	}
	if _, err := os.Stat(inboxPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected task to leave inbox, stat err=%v", err)
	}
	archivePath := filepath.Join(root, "archive", "hello-endpoint.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archived task file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "state", "hello-endpoint.json")); err != nil {
		t.Fatalf("expected task state file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "results", "hello-endpoint", "summary.md")); err != nil {
		t.Fatalf("expected task summary file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "results", "hello-endpoint", "metadata.json")); err != nil {
		t.Fatalf("expected task metadata file: %v", err)
	}

	issues, err := client.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("fetch candidates after archive: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no active candidates after archive, got %d", len(issues))
	}

	refreshed, err := client.FetchIssuesByIDs(context.Background(), []string{"hello-endpoint"})
	if err != nil {
		t.Fatalf("fetch issues by id: %v", err)
	}
	if len(refreshed) != 1 || refreshed[0].State != "Done" {
		t.Fatalf("unexpected refreshed issue set: %#v", refreshed)
	}
}
