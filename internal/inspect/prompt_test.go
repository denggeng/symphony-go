package inspect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderLocalTaskPromptByTaskID(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	workflowPath := writePromptFixture(t, root)
	writeTaskFixture(t, filepath.Join(root, "local_tasks", "inbox", "demo-task.md"), `---
id: demo-task
title: Demo task
state: To Do
---
Implement the demo endpoint.
`)

	text, err := RenderLocalTaskPrompt(PromptRenderOptions{WorkflowPath: workflowPath, TaskID: "demo-task"})
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}
	if !strings.Contains(text, "Identifier: demo-task") {
		t.Fatalf("missing task id in prompt: %q", text)
	}
	if !strings.Contains(text, "State: To Do") {
		t.Fatalf("missing task state in prompt: %q", text)
	}
	if !strings.Contains(text, "Implement the demo endpoint.") {
		t.Fatalf("missing task body in prompt: %q", text)
	}
}

func TestRenderLocalTaskPromptByWorkspacePathAndContinuationTurn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	workflowPath := writePromptFixture(t, root)
	writeTaskFixture(t, filepath.Join(root, "local_tasks", "inbox", "demo-task.md"), `---
id: demo-task
title: Demo task
state: In Progress
---
Implement the demo endpoint.
`)
	workspacePath := filepath.Join(root, "workspaces", "demo-task")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	text, err := RenderLocalTaskPrompt(PromptRenderOptions{WorkflowPath: workflowPath, WorkspacePath: workspacePath, Turn: 2, Attempt: 3})
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}
	if !strings.Contains(text, "continuation turn #2 of 7") {
		t.Fatalf("expected continuation prompt, got: %q", text)
	}
	if strings.Contains(text, "Implement the demo endpoint.") {
		t.Fatalf("continuation prompt should not repeat first-turn body: %q", text)
	}
}

func writePromptFixture(t *testing.T, root string) string {
	t.Helper()
	for _, dir := range []string{
		filepath.Join(root, "local_tasks", "inbox"),
		filepath.Join(root, "local_tasks", "state"),
		filepath.Join(root, "local_tasks", "archive"),
		filepath.Join(root, "local_tasks", "results"),
		filepath.Join(root, "workspaces"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create fixture dir %s: %v", dir, err)
		}
	}
	workflowPath := filepath.Join(root, "WORKFLOW.md")
	workflowContent := fmt.Sprintf(`---
tracker:
  kind: local
local:
  inbox_dir: %s
  state_dir: %s
  archive_dir: %s
  results_dir: %s
  active_states:
    - To Do
    - In Progress
  terminal_states:
    - Done
    - Blocked
workspace:
  root: %s
agent:
  max_turns: 7
---
Identifier: {{ issue.identifier }}
State: {{ issue.state }}

Body:
{{ issue.description }}
`, filepath.ToSlash(filepath.Join(root, "local_tasks", "inbox")), filepath.ToSlash(filepath.Join(root, "local_tasks", "state")), filepath.ToSlash(filepath.Join(root, "local_tasks", "archive")), filepath.ToSlash(filepath.Join(root, "local_tasks", "results")), filepath.ToSlash(filepath.Join(root, "workspaces")))
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0o600); err != nil {
		t.Fatalf("write workflow fixture: %v", err)
	}
	return workflowPath
}

func writeTaskFixture(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("write task fixture: %v", err)
	}
}
