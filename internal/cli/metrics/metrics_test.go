package metrics_test

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/metrics"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
)

func newMetricsCmd(t *testing.T) *cobra.Command {
	t.Helper()
	mockddtest.InstallClientFactory(t)
	root := &cobra.Command{Use: "agent-dd"}
	g := &shared.GlobalFlags{Format: "ndjson"}
	metrics.Register(root, func() *shared.GlobalFlags { return g })
	return root
}

// `metrics query` with a known-bad query must surface the parse error
// returned by Datadog as a non-nil RunE error so the CLI exits non-zero
// and the user sees the hint instead of an empty result set. The
// `fail-query` sentinel triggers the mockdd error path.
func TestMetricsQuerySurfacesParseError(t *testing.T) {
	cmd := newMetricsCmd(t)
	cmd.SetArgs([]string{"metrics", "query", "--query", "fail-query", "--from", "now-1h", "--to", "now"})

	var execErr error
	_ = mockddtest.CaptureStdout(t, func() {
		execErr = cmd.Execute()
	})

	if execErr == nil {
		t.Fatal("expected metrics query to return an error for fail-query, got nil")
	}
	if !strings.Contains(execErr.Error(), "query parse error") {
		t.Errorf("error %q should mention the parse error message from DD", execErr.Error())
	}
}
