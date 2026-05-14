package monitors_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/monitors"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
)

func newMonitorsCmd(t *testing.T) *cobra.Command {
	t.Helper()
	mockddtest.InstallClientFactory(t)
	root := &cobra.Command{Use: "agent-dd"}
	g := &shared.GlobalFlags{Format: "ndjson"}
	monitors.Register(root, func() *shared.GlobalFlags { return g })
	return root
}

// `monitors search` emits the result rollups as a `@counts` meta-line ahead
// of the monitor rows. This pins the contract — losing @counts would mean
// the CLI silently drops the alert/warn/ok totals that the triage skill
// reference describes.
func TestMonitorsSearchEmitsCountsMetaLine(t *testing.T) {
	cmd := newMonitorsCmd(t)
	cmd.SetArgs([]string{"monitors", "search", "--query", "*"})

	out := mockddtest.CaptureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var sawCounts bool
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("invalid NDJSON %q: %v", line, err)
		}
		if c, ok := row["@counts"].(map[string]any); ok {
			sawCounts = true
			status, _ := c["status"].([]any)
			if len(status) == 0 {
				t.Error("@counts.status is empty — rollup not populated")
			}
		}
	}
	if !sawCounts {
		t.Fatal("expected @counts meta-line in monitors search output")
	}
}
