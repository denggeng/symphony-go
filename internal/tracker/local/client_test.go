package local

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/denggeng/symphony-go/internal/config"
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
