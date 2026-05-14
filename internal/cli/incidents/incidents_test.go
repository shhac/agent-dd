package incidents_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/incidents"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
)

func newIncidentsCmd(t *testing.T) *cobra.Command {
	t.Helper()
	mockddtest.InstallClientFactory(t)
	root := &cobra.Command{Use: "agent-dd"}
	g := &shared.GlobalFlags{Format: "ndjson"}
	incidents.Register(root, func() *shared.GlobalFlags { return g })
	return root
}

// `incidents list` resolves the commander handle from the JSON:API
// `included` array and inlines it as `commander` on each row. Pinning this
// means the rendering path stays connected to the resolver in api/incidents.go.
func TestIncidentsListSurfacesResolvedCommander(t *testing.T) {
	cmd := newIncidentsCmd(t)
	cmd.SetArgs([]string{"incidents", "list"})

	out := mockddtest.CaptureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		t.Fatal("expected incident rows, got 0")
	}

	var sawCommander bool
	for _, line := range lines {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("invalid NDJSON %q: %v", line, err)
		}
		if c, ok := row["commander"].(string); ok && c != "" {
			sawCommander = true
		}
		// The legacy `status` field must not appear — the v2 API renamed it
		// to `state` and the audit fixed the rendering.
		if _, hasStatus := row["status"]; hasStatus {
			t.Errorf("legacy `status` field leaked into output; expected `state`: %v", row)
		}
	}
	if !sawCommander {
		t.Error("expected at least one incident row to surface `commander` from JSON:API included")
	}
}
