package api

import (
	"maps"
	"reflect"
	"slices"
)

// A Datadog monitor has far more fields than this CLI models. `monitors update`
// therefore works over the raw decoded map rather than the Monitor struct: read
// the monitor, apply the caller's changes to the map, write the whole thing
// back. Anything unmodelled — restricted_roles, composite sub-monitors, the
// long tail of options — makes the round trip untouched, which a
// marshal-the-struct approach would silently drop.
//
// Everything in this file is a pure function over maps. Wire correctness is
// mockdd's job; these are the merge semantics, and they're tested without HTTP.

// monitorReadOnlyFields are server-owned. Datadog returns them on GET but they
// describe evaluation state rather than configuration, so echoing them back on
// PUT is at best meaningless and at worst rejected.
var monitorReadOnlyFields = []string{
	// Both spellings: reads normalise `overall_state` to `status`, and neither
	// may be written back. Stripping only the raw name would let the state
	// survive into the PUT body once normalisation moved into the client.
	"overall_state",
	"status",
	"overall_state_modified",
	"created",
	"created_at",
	"modified",
	"last_triggered_ts",
	"creator",
	"org_id",
	"matching_downtimes",
	"deleted",
	"state",
}

// stripReadOnlyMonitorFields returns a copy with the server-owned fields
// removed. The input is left intact — callers hold onto it to render the
// before/after diff.
func stripReadOnlyMonitorFields(monitor map[string]any) map[string]any {
	out := cloneMonitorMap(monitor)
	for _, field := range monitorReadOnlyFields {
		delete(out, field)
	}
	return out
}

// applyMonitorUpdates layers `changes` onto `existing` and returns the result,
// mutating neither.
//
// Nested maps merge key-by-key so that changing one option leaves its siblings
// alone: `--renotify-interval 30` must not clear a tuned threshold. Everything
// else — scalars, and notably lists like `tags` — replaces wholesale, because
// merging a list would make removal impossible.
func applyMonitorUpdates(existing, changes map[string]any) map[string]any {
	out := cloneMonitorMap(existing)
	for key, incoming := range changes {
		current, present := out[key]
		if !present {
			out[key] = cloneValue(incoming)
			continue
		}

		currentMap, currentIsMap := current.(map[string]any)
		incomingMap, incomingIsMap := incoming.(map[string]any)
		if currentIsMap && incomingIsMap {
			out[key] = applyMonitorUpdates(currentMap, incomingMap)
			continue
		}

		out[key] = cloneValue(incoming)
	}
	return out
}

// FieldChange is one before/after pair in an update's diff. Agents report these
// back to a human — "critical threshold 5 -> 20, nothing else moved" — so the
// diff is part of the command's contract, not debug output.
type FieldChange struct {
	From any `json:"from"`
	To   any `json:"to"`
}

// diffMonitorFields reports only what actually changed, descending into nested
// maps and naming those entries by dotted path (`options.renotify_interval`).
// A field present on one side only is reported against a nil counterpart.
func diffMonitorFields(before, after map[string]any) map[string]FieldChange {
	changes := map[string]FieldChange{}
	collectFieldChanges("", before, after, changes)
	return changes
}

func collectFieldChanges(prefix string, before, after map[string]any, changes map[string]FieldChange) {
	for _, key := range sortedUnionKeys(before, after) {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		oldValue, inBefore := before[key]
		newValue, inAfter := after[key]

		oldMap, oldIsMap := oldValue.(map[string]any)
		newMap, newIsMap := newValue.(map[string]any)
		if inBefore && inAfter && oldIsMap && newIsMap {
			collectFieldChanges(path, oldMap, newMap, changes)
			continue
		}

		if reflect.DeepEqual(oldValue, newValue) {
			continue
		}
		changes[path] = FieldChange{From: oldValue, To: newValue}
	}
}

func sortedUnionKeys(a, b map[string]any) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for key := range a {
		seen[key] = struct{}{}
	}
	for key := range b {
		seen[key] = struct{}{}
	}
	return slices.Sorted(maps.Keys(seen))
}

// cloneMonitorMap deep-copies the nested maps and slices so a merge can never
// write through into the caller's copy. Scalars are shared, which is safe —
// they're immutable in Go.
func cloneMonitorMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for key, value := range m {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMonitorMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneValue(item)
		}
		return out
	case []string:
		return slices.Clone(typed)
	default:
		return value
	}
}
