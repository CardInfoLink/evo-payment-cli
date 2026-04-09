package completion

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestRoot creates a minimal root command with the completion subcommand attached.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "evo-cli"}
	root.AddCommand(NewCmdCompletion())
	return root
}

func TestCompletionBash(t *testing.T) {
	root := newTestRoot()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"completion", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion bash failed: %v", err)
	}
	// bash completion v2 contains this marker
	if !strings.Contains(buf.String(), "bash") && len(buf.String()) == 0 {
		t.Error("expected non-empty bash completion output")
	}
}

func TestCompletionZsh(t *testing.T) {
	root := newTestRoot()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"completion", "zsh"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion zsh failed: %v", err)
	}
	if len(buf.String()) == 0 {
		t.Error("expected non-empty zsh completion output")
	}
}

func TestCompletionFish(t *testing.T) {
	root := newTestRoot()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"completion", "fish"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion fish failed: %v", err)
	}
	if !strings.Contains(buf.String(), "complete") {
		t.Error("expected fish completion output to contain 'complete'")
	}
}

func TestCompletionPowershell(t *testing.T) {
	root := newTestRoot()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"completion", "powershell"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion powershell failed: %v", err)
	}
	if len(buf.String()) == 0 {
		t.Error("expected non-empty powershell completion output")
	}
}

func TestCompletionInvalidShell(t *testing.T) {
	root := newTestRoot()
	root.SetArgs([]string{"completion", "tcsh"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error for unsupported shell 'tcsh'")
	}
}

func TestCompletionNoArgs(t *testing.T) {
	root := newTestRoot()
	root.SetArgs([]string{"completion"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no shell argument provided")
	}
}

func TestCompletionValidArgs(t *testing.T) {
	cmd := NewCmdCompletion()
	want := []string{"bash", "zsh", "fish", "powershell"}
	if len(cmd.ValidArgs) != len(want) {
		t.Fatalf("ValidArgs length = %d, want %d", len(cmd.ValidArgs), len(want))
	}
	for i, v := range want {
		if cmd.ValidArgs[i] != v {
			t.Errorf("ValidArgs[%d] = %q, want %q", i, cmd.ValidArgs[i], v)
		}
	}
}
