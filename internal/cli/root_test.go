package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/shhac/agent-dd/internal/output"
)

// captureStderr swaps os.Stderr for the duration of fn and returns what was
// written. The root sink renders to the real os.Stderr, so it can only be
// exercised by capturing the pipe this way.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	return buf.String()
}

// libcli.Run is the single error sink: a cobra usage error (one cobra produces
// itself, not one a command pre-renders) must reach stderr as a structured
// {error,fixable_by} row exactly once, never as plain text. Run itself calls
// os.Exit, so this drives the same root command + output.WriteError it does,
// minus the exit, to assert the render shape.
func TestRunRendersUsageErrorStructuredOnce(t *testing.T) {
	root := newRootCmd("test")
	root.SetArgs([]string{"definitely-not-a-command"})

	var execErr error
	stderr := captureStderr(t, func() {
		execErr = root.Execute()
		if execErr != nil {
			output.WriteError(os.Stderr, execErr)
		}
	})

	if execErr == nil {
		t.Fatal("expected the root command to return the usage error")
	}

	lines := nonEmptyLines(stderr)
	if len(lines) != 1 {
		t.Fatalf("expected exactly one stderr line, got %d: %q", len(lines), stderr)
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatalf("expected structured JSON error, got %q (%v)", lines[0], err)
	}
	if row["fixable_by"] != "agent" {
		t.Errorf("expected fixable_by=agent for a usage error, got %v", row["fixable_by"])
	}
	if msg, _ := row["error"].(string); !strings.Contains(msg, "definitely-not-a-command") {
		t.Errorf("expected error naming the bad command, got %q", msg)
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}
