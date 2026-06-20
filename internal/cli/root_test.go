package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// captureStderr swaps os.Stderr for the duration of fn and returns what was
// written. Execute renders to the real os.Stderr, so the root sink can only be
// exercised through the package-level entry point this way.
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

// Execute is the single error sink: a cobra usage error (one cobra produces
// itself, not one a command pre-renders) must reach stderr as a structured
// {error,fixable_by} row exactly once, never as plain text.
func TestExecuteRendersUsageErrorStructuredOnce(t *testing.T) {
	origArgs := os.Args
	os.Args = []string{"agent-dd", "definitely-not-a-command"}
	defer func() { os.Args = origArgs }()

	var execErr error
	stderr := captureStderr(t, func() { execErr = Execute("test") })

	if execErr == nil {
		t.Fatal("expected Execute to return the usage error")
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
