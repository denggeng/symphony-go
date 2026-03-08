package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesFrontMatterAndPrompt(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "WORKFLOW.md")
	content := `---
tracker:
  kind: jira
server:
  port: 9090
---
Hello from workflow.
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	definition, err := Load(path)
	if err != nil {
		t.Fatalf("load workflow: %v", err)
	}
	if definition.Path != path {
		t.Fatalf("unexpected path: got %q want %q", definition.Path, path)
	}
	if !definition.HasFrontMatter {
		t.Fatalf("expected front matter flag to be true")
	}
	if definition.Prompt != "Hello from workflow." {
		t.Fatalf("unexpected prompt: %q", definition.Prompt)
	}
	tracker, ok := definition.Config["tracker"].(map[string]any)
	if !ok {
		t.Fatalf("expected tracker config map, got %#v", definition.Config["tracker"])
	}
	if tracker["kind"] != "jira" {
		t.Fatalf("unexpected tracker kind: %#v", tracker["kind"])
	}
}

func TestParseWithoutFrontMatter(t *testing.T) {
	t.Parallel()
	definition, err := Parse("/tmp/WORKFLOW.md", "just the prompt")
	if err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	if definition.HasFrontMatter {
		t.Fatalf("did not expect front matter")
	}
	if len(definition.Config) != 0 {
		t.Fatalf("expected empty config, got %#v", definition.Config)
	}
	if definition.Prompt != "just the prompt" {
		t.Fatalf("unexpected prompt: %q", definition.Prompt)
	}
}
