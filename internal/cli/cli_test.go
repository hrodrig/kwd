package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// runCmd executes a cobra command with args and captures stdout+stderr.
func runCmd(t *testing.T, cmd *cobra.Command, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "kwd.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVersionCommand(t *testing.T) {
	root := NewRootCmd()
	out, _, err := runCmd(t, root, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if out == "" {
		t.Fatal("version output empty")
	}
}

func TestAnalyzeCommand(t *testing.T) {
	cfgPath := writeConfig(t, `
cluster:
  name: test-cluster
resources:
  - deployment.default/app
  - statefulset.default/db
`)

	root := NewRootCmd()
	out, _, err := runCmd(t, root, "analyze", "--config", cfgPath)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if out == "" {
		t.Fatal("analyze output empty")
	}
}

func TestAnalyzeRejectsBadConfig(t *testing.T) {
	cfgPath := writeConfig(t, "resources:\n  - daemonset.default/x\n")

	root := NewRootCmd()
	if _, _, err := runCmd(t, root, "analyze", "--config", cfgPath); err == nil {
		t.Fatal("expected analyze to reject unsupported kind")
	}
}

func TestTargetCommand(t *testing.T) {
	cfgPath := writeConfig(t, `
cluster:
  name: test-cluster
resources:
  - deployment.default/app
`)

	root := NewRootCmd()
	out, _, err := runCmd(t, root, "target", "--config", cfgPath)
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	if out == "" {
		t.Fatal("target output empty")
	}
}

func TestNotifyTestNotAvailableInV0(t *testing.T) {
	root := NewRootCmd()
	if _, _, err := runCmd(t, root, "notify", "test"); err == nil {
		t.Fatal("notify test should not be available in v0.1")
	}
}
