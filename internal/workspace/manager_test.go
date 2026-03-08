package workspace

import (
	"testing"

	"github.com/denggeng/symphony-go/internal/config"
)

func TestValidatePathRejectsOutsideRoot(t *testing.T) {
	t.Parallel()
	manager := New(config.Config{Workspace: config.WorkspaceConfig{Root: t.TempDir()}}, nil)
	if err := manager.validatePath("/tmp"); err == nil {
		t.Fatalf("expected validation error")
	}
}
