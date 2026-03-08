package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIfExistsLoadsMissingVariablesOnly(t *testing.T) {
	t.Setenv("EXISTING_VALUE", "from-env")
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	payload := "# comment\nNEW_VALUE=loaded\nexport QUOTED=\"hello world\"\nEXISTING_VALUE=from-file\n"
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	if err := LoadIfExists(path); err != nil {
		t.Fatalf("load env file: %v", err)
	}
	if got := os.Getenv("NEW_VALUE"); got != "loaded" {
		t.Fatalf("unexpected NEW_VALUE: %q", got)
	}
	if got := os.Getenv("QUOTED"); got != "hello world" {
		t.Fatalf("unexpected QUOTED: %q", got)
	}
	if got := os.Getenv("EXISTING_VALUE"); got != "from-env" {
		t.Fatalf("expected existing env to win, got %q", got)
	}
}

func TestLoadIfExistsIgnoresMissingFile(t *testing.T) {
	t.Parallel()
	if err := LoadIfExists(filepath.Join(t.TempDir(), ".env")); err != nil {
		t.Fatalf("expected missing env file to be ignored: %v", err)
	}
}

func TestLoadIfExistsRejectsInvalidLine(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("INVALID_LINE\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	if err := LoadIfExists(path); err == nil {
		t.Fatalf("expected parse error")
	}
}
