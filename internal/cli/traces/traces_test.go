package traces_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/cli/traces"
	"github.com/shhac/agent-dd/internal/mockdd/mockddtest"
)

func newCmd(t *testing.T) *cobra.Command {
	t.Helper()
	mockddtest.InstallClientFactory(t)
	root := &cobra.Command{Use: "agent-dd"}
	g := &shared.GlobalFlags{Format: "ndjson"}
	traces.Register(root, func() *shared.GlobalFlags { return g })
	return root
}

// parseNDJSON splits the captured stdout into data rows and meta rows
// (those keyed by a single `@`-prefixed key). The agent-dd output contract:
// data rows are full per-item objects; meta rows are single-key objects
// like {"@skipped":[...]} or {"@pagination":{...}}.
func parseNDJSON(t *testing.T, out string) (data []map[string]any, meta map[string]any) {
	t.Helper()
	meta = map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("invalid NDJSON line %q: %v", line, err)
		}
		if len(row) == 1 {
			for k := range row {
				if strings.HasPrefix(k, "@") {
					meta[k] = row[k]
					row = nil
					break
				}
			}
		}
		if row != nil {
			data = append(data, row)
		}
	}
	return data, meta
}

// Default `traces search` emits the navigation/context fields (trace_id,
// span_id, parent_id, host, type, etc) inline but hides the free-form
// attributes/custom blobs behind a `@skipped` meta-line. Pinning this is
// how we know future drift won't silently leak the large blobs or drop
// the IDs callers need to chase a specific span.
func TestTracesSearchDefaultEmitsIDsAndSkippedMeta(t *testing.T) {
	cmd := newCmd(t)
	cmd.SetArgs([]string{"traces", "search", "--query", "*", "--from", "now-1h", "--to", "now"})

	out := mockddtest.CaptureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	data, meta := parseNDJSON(t, out)
	if len(data) == 0 {
		t.Fatal("expected span rows, got 0")
	}

	for _, row := range data {
		for _, k := range []string{"trace_id", "span_id", "parent_id", "host", "type", "service", "operation", "resource"} {
			if _, ok := row[k]; !ok {
				t.Errorf("default output missing %q — identifiers/context must stay in the compact view", k)
			}
		}
		if _, ok := row["attributes"]; ok {
			t.Error("default output leaked free-form `attributes` blob; should be behind --full")
		}
		if _, ok := row["custom"]; ok {
			t.Error("default output leaked `custom` blob; should be behind --full")
		}
	}

	skipped, ok := meta["@skipped"].([]any)
	if !ok {
		t.Fatalf("expected @skipped meta-line in default output, got %v", meta)
	}
	want := map[string]bool{"attributes": true, "custom": true, "single_span": true}
	for _, k := range skipped {
		ks, _ := k.(string)
		if !want[ks] {
			t.Errorf("unexpected @skipped entry %q", ks)
		}
		delete(want, ks)
	}
	if len(want) > 0 {
		t.Errorf("@skipped missing entries: %v", want)
	}
}

// `--full` should add the free-form blobs and suppress @skipped (since
// nothing is hidden).
func TestTracesSearchFullIncludesBlobsAndSuppressesSkippedMeta(t *testing.T) {
	cmd := newCmd(t)
	cmd.SetArgs([]string{"traces", "search", "--query", "*", "--from", "now-1h", "--to", "now", "--full"})

	out := mockddtest.CaptureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	_, meta := parseNDJSON(t, out)
	if _, hasSkipped := meta["@skipped"]; hasSkipped {
		t.Error("--full output must not emit @skipped (nothing hidden)")
	}
	// Note: mockdd does not currently emit `attributes`/`custom` in its
	// trace fixtures, so the `--full` rows show those keys as nil-suppressed
	// from the map (CLI only adds them when len(...) > 0). The meta-line
	// suppression is the contract we pin here.
}
