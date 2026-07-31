package api

import (
	"reflect"
	"testing"
)

// The read-modify-write path exists so `monitors update` can change one field
// without destroying the ~85% of a real monitor that this CLI does not model.
// These tests are the guard on that promise, and they run over plain maps with
// no HTTP anywhere — wire correctness is mockdd's job, merge correctness is
// this file's.

func TestStripReadOnlyMonitorFields(t *testing.T) {
	monitor := map[string]any{
		"id":                 1001,
		"name":               "High CPU",
		"query":              "avg(last_5m):avg:system.cpu.user{*} > 90",
		"overall_state":      "alert",
		"created":            "2025-11-01T08:00:00Z",
		"modified":           "2026-03-15T14:30:00Z",
		"last_triggered_ts":  1711893600,
		"creator":            map[string]any{"name": "Alice"},
		"org_id":             12345,
		"matching_downtimes": []any{},
		"deleted":            nil,
		"state":              map[string]any{"groups": map[string]any{}},
		"overall_state_modified": "2026-03-15T14:30:00Z",
		"restricted_roles":       []any{"role-abc"},
		"options":                map[string]any{"thresholds": map[string]any{"critical": 90}},
	}

	got := stripReadOnlyMonitorFields(monitor)

	for _, removed := range []string{
		"overall_state", "created", "modified", "last_triggered_ts",
		"creator", "org_id", "matching_downtimes", "deleted", "state",
		"overall_state_modified",
	} {
		if _, present := got[removed]; present {
			t.Errorf("read-only field %q survived the strip", removed)
		}
	}

	// Fields the CLI doesn't model must survive — that's the whole point.
	for _, kept := range []string{"id", "name", "query", "restricted_roles", "options"} {
		if _, present := got[kept]; !present {
			t.Errorf("field %q was stripped but should have been preserved", kept)
		}
	}
}

func TestStripReadOnlyMonitorFieldsDoesNotMutateInput(t *testing.T) {
	monitor := map[string]any{"id": 1, "overall_state": "alert"}

	stripReadOnlyMonitorFields(monitor)

	if _, present := monitor["overall_state"]; !present {
		t.Error("stripReadOnlyMonitorFields mutated its input; callers rely on holding the original for diffing")
	}
}

func TestApplyMonitorUpdatesSetsTopLevelFields(t *testing.T) {
	existing := map[string]any{"name": "old", "query": "q", "priority": 3}

	got := applyMonitorUpdates(existing, map[string]any{"name": "new", "priority": 1})

	if got["name"] != "new" {
		t.Errorf("name = %v, want new", got["name"])
	}
	if got["priority"] != 1 {
		t.Errorf("priority = %v, want 1", got["priority"])
	}
	if got["query"] != "q" {
		t.Errorf("query = %v, want q (untouched fields must survive)", got["query"])
	}
}

func TestApplyMonitorUpdatesMergesOptionsInsteadOfReplacing(t *testing.T) {
	existing := map[string]any{
		"name": "mon",
		"options": map[string]any{
			"thresholds":        map[string]any{"critical": 90.0, "warning": 80.0},
			"notify_no_data":    true,
			"evaluation_delay":  300,
			"renotify_interval": 60,
		},
	}

	got := applyMonitorUpdates(existing, map[string]any{
		"options": map[string]any{"renotify_interval": 30},
	})

	opts, ok := got["options"].(map[string]any)
	if !ok {
		t.Fatalf("options is %T, want map[string]any", got["options"])
	}
	if opts["renotify_interval"] != 30 {
		t.Errorf("renotify_interval = %v, want 30", opts["renotify_interval"])
	}
	// The bug this whole design exists to prevent.
	if opts["notify_no_data"] != true {
		t.Error("notify_no_data was destroyed by a partial options update")
	}
	if opts["evaluation_delay"] != 300 {
		t.Error("evaluation_delay was destroyed by a partial options update")
	}
	thresholds, ok := opts["thresholds"].(map[string]any)
	if !ok {
		t.Fatalf("thresholds is %T, want map[string]any", opts["thresholds"])
	}
	if thresholds["critical"] != 90.0 || thresholds["warning"] != 80.0 {
		t.Errorf("thresholds were destroyed by an unrelated options update: %v", thresholds)
	}
}

func TestApplyMonitorUpdatesMergesNestedThresholds(t *testing.T) {
	existing := map[string]any{
		"options": map[string]any{
			"thresholds": map[string]any{"critical": 90.0, "warning": 80.0},
		},
	}

	got := applyMonitorUpdates(existing, map[string]any{
		"options": map[string]any{"thresholds": map[string]any{"critical": 95.0}},
	})

	opts := got["options"].(map[string]any)
	thresholds := opts["thresholds"].(map[string]any)
	if thresholds["critical"] != 95.0 {
		t.Errorf("critical = %v, want 95", thresholds["critical"])
	}
	if thresholds["warning"] != 80.0 {
		t.Errorf("warning = %v, want 80 — raising critical must not clear warning", thresholds["warning"])
	}
}

func TestApplyMonitorUpdatesAddsOptionsWhenAbsent(t *testing.T) {
	existing := map[string]any{"name": "mon"}

	got := applyMonitorUpdates(existing, map[string]any{
		"options": map[string]any{"renotify_interval": 30},
	})

	opts, ok := got["options"].(map[string]any)
	if !ok {
		t.Fatalf("options is %T, want map[string]any", got["options"])
	}
	if opts["renotify_interval"] != 30 {
		t.Errorf("renotify_interval = %v, want 30", opts["renotify_interval"])
	}
}

func TestApplyMonitorUpdatesDoesNotMutateInput(t *testing.T) {
	existing := map[string]any{
		"name":    "old",
		"options": map[string]any{"renotify_interval": 60},
	}

	applyMonitorUpdates(existing, map[string]any{
		"name":    "new",
		"options": map[string]any{"renotify_interval": 30},
	})

	if existing["name"] != "old" {
		t.Error("applyMonitorUpdates mutated the input's top-level fields")
	}
	opts := existing["options"].(map[string]any)
	if opts["renotify_interval"] != 60 {
		t.Error("applyMonitorUpdates mutated the input's nested options; the caller needs the original to render a before/after diff")
	}
}

func TestApplyMonitorUpdatesReplacesListsWholesale(t *testing.T) {
	// Tags are set-like: merging them would make removal impossible.
	existing := map[string]any{"tags": []string{"env:prod", "team:platform"}}

	got := applyMonitorUpdates(existing, map[string]any{"tags": []string{"env:staging"}})

	want := []string{"env:staging"}
	if !reflect.DeepEqual(got["tags"], want) {
		t.Errorf("tags = %v, want %v — lists replace, they don't merge", got["tags"], want)
	}
}

func TestDiffMonitorFields(t *testing.T) {
	before := map[string]any{
		"name":     "old name",
		"priority": 3,
		"query":    "unchanged",
		"options":  map[string]any{"renotify_interval": 60, "notify_no_data": true},
	}
	after := map[string]any{
		"name":     "new name",
		"priority": 3,
		"query":    "unchanged",
		"options":  map[string]any{"renotify_interval": 30, "notify_no_data": true},
	}

	got := diffMonitorFields(before, after)

	name, ok := got["name"]
	if !ok {
		t.Fatal("expected name in the diff")
	}
	if name.From != "old name" || name.To != "new name" {
		t.Errorf("name diff = %+v, want old name -> new name", name)
	}

	if _, present := got["priority"]; present {
		t.Error("priority is unchanged and must not appear in the diff")
	}
	if _, present := got["query"]; present {
		t.Error("query is unchanged and must not appear in the diff")
	}

	opts, ok := got["options.renotify_interval"]
	if !ok {
		t.Fatalf("expected a dotted path for the changed nested option, got keys %v", keysOf(got))
	}
	if opts.From != 60 || opts.To != 30 {
		t.Errorf("options.renotify_interval diff = %+v, want 60 -> 30", opts)
	}
	if _, present := got["options.notify_no_data"]; present {
		t.Error("unchanged nested option must not appear in the diff")
	}
}

func TestDiffMonitorFieldsReportsAddedAndRemoved(t *testing.T) {
	before := map[string]any{"name": "mon", "message": "old message"}
	after := map[string]any{"name": "mon", "priority": 2}

	got := diffMonitorFields(before, after)

	removed, ok := got["message"]
	if !ok {
		t.Fatal("expected removed field in the diff")
	}
	if removed.From != "old message" || removed.To != nil {
		t.Errorf("message diff = %+v, want old message -> nil", removed)
	}

	added, ok := got["priority"]
	if !ok {
		t.Fatal("expected added field in the diff")
	}
	if added.From != nil || added.To != 2 {
		t.Errorf("priority diff = %+v, want nil -> 2", added)
	}
}

func keysOf(m map[string]FieldChange) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
