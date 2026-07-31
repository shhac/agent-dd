package monitors_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
)

// Every monitor command must name the monitor's state the same way. Datadog
// itself does not — /v1/monitor calls it `overall_state`, /v1/monitor/search
// calls it `status` — and that inconsistency leaked straight through: `list`
// emitted `status` while `list --full`, `get` and `create` emitted
// `overall_state`, so an agent that listed and then fetched saw the key change
// underneath it. The CLI documents `status`, so `status` is the contract.

// stateKeysIn walks the whole decoded document, so it works for the pretty
// printed `--format json` envelope as well as NDJSON, and finds the key
// wherever it sits (update nests the monitor under "monitor").
func stateKeysIn(t *testing.T, out string) (hasStatus, hasOverallState bool) {
	t.Helper()

	var doc any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		// NDJSON: one document per line.
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			var row any
			if json.Unmarshal([]byte(line), &row) == nil {
				s, o := walkForStateKeys(row)
				hasStatus, hasOverallState = hasStatus || s, hasOverallState || o
			}
		}
		return hasStatus, hasOverallState
	}
	return walkForStateKeys(doc)
}

// looksLikeMonitor distinguishes a monitor object from an operation envelope.
// Both use `status`, but for different things: `{"status":"updated"}` is the
// outcome of a command, `{"id":…,"status":"Alert"}` is a monitor's state. Only
// the latter is what this test is about.
func looksLikeMonitor(m map[string]any) bool {
	if _, hasID := m["id"]; !hasID {
		return false
	}
	_, hasName := m["name"]
	_, hasQuery := m["query"]
	return hasName || hasQuery
}

func walkForStateKeys(node any) (hasStatus, hasOverallState bool) {
	switch typed := node.(type) {
	case map[string]any:
		if looksLikeMonitor(typed) {
			if _, ok := typed["status"]; ok {
				hasStatus = true
			}
			if _, ok := typed["overall_state"]; ok {
				hasOverallState = true
			}
		}
		for _, child := range typed {
			s, o := walkForStateKeys(child)
			hasStatus, hasOverallState = hasStatus || s, hasOverallState || o
		}
	case []any:
		for _, child := range typed {
			s, o := walkForStateKeys(child)
			hasStatus, hasOverallState = hasStatus || s, hasOverallState || o
		}
	}
	return hasStatus, hasOverallState
}

func TestMonitorCommandsAllNameTheStateStatus(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"list", []string{"list"}},
		{"list --full", []string{"list", "--full"}},
		{"get", []string{"get", "1001"}},
		{"search", []string{"search", "--query", "CPU"}},
		{"create", []string{"create", "--type", "metric alert", "--query", "avg(last_5m):avg:x{*} > 1", "--name", "t"}},
		{"update", []string{"update", "1001", "--name", "renamed"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockddtest.InstallClientFactory(t)

			out, err := runMonitors(t, "", tc.args...)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}

			hasStatus, hasOverallState := stateKeysIn(t, out)
			if hasOverallState {
				t.Errorf("%s emits `overall_state`; every command must use `status`", tc.name)
			}
			if !hasStatus {
				t.Errorf("%s emits no state key at all", tc.name)
			}
		})
	}
}
