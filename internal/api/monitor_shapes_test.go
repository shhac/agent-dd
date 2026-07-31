package api

import (
	"encoding/json"
	"testing"
)

// /v1/monitor and /v1/monitor/search return the same entity in two different
// shapes. Decoding both into one Monitor struct only worked for the first:
//
//	list/get         search
//	overall_state    status                (different key entirely)
//	created  string  created  epoch int    (different type)
//	modified string  modified epoch int
//
// So `monitors search` failed outright with "cannot unmarshal number into
// Go struct field Monitor.monitors.created of type string", and had it
// decoded, Status would have been empty and --status would have matched
// nothing. The mock returned list-shaped objects from its search endpoint,
// so nothing in the suite ever saw the difference.
//
// Both fixtures below are real shapes, reduced to the fields we model and
// stripped of anything identifying.

const searchShapedMonitor = `{
  "id": 12345,
  "name": "Example monitor",
  "type": "query alert",
  "query": "sum(last_1h):sum:example.metric{*}.as_count() > 0",
  "message": "notify someone",
  "tags": ["service:example"],
  "status": "No Data",
  "created": 1776777117,
  "modified": 1776777120,
  "overall_state_modified": 1776777120,
  "priority": 1,
  "classification": "metric",
  "restricted_roles": []
}`

const listShapedMonitor = `{
  "id": 12345,
  "name": "Example monitor",
  "type": "query alert",
  "query": "sum(last_1h):sum:example.metric{*}.as_count() > 0",
  "message": "notify someone",
  "tags": ["service:example"],
  "overall_state": "No Data",
  "created": "2026-04-21T13:11:18.896593+00:00",
  "modified": "2026-04-21T13:11:21.905499+00:00",
  "priority": 1,
  "options": {"thresholds": {"critical": 0}}
}`

func TestMonitorDecodesSearchShape(t *testing.T) {
	var m Monitor
	if err := json.Unmarshal([]byte(searchShapedMonitor), &m); err != nil {
		t.Fatalf("decoding a search-shaped monitor failed: %v", err)
	}

	if m.ID != 12345 {
		t.Errorf("id = %d, want 12345", m.ID)
	}
	// The key difference: search names this `status`, not `overall_state`.
	if m.Status != "No Data" {
		t.Errorf("status = %q, want \"No Data\" — search names it `status`", m.Status)
	}
	if m.Created == "" {
		t.Error("created was dropped; search sends it as an epoch number")
	}
	if m.Modified == "" {
		t.Error("modified was dropped")
	}
}

func TestMonitorDecodesListShape(t *testing.T) {
	var m Monitor
	if err := json.Unmarshal([]byte(listShapedMonitor), &m); err != nil {
		t.Fatalf("decoding a list-shaped monitor failed: %v", err)
	}

	if m.Status != "No Data" {
		t.Errorf("status = %q, want \"No Data\"", m.Status)
	}
	if m.Created != "2026-04-21T13:11:18.896593+00:00" {
		t.Errorf("created = %q, want the RFC3339 string untouched", m.Created)
	}
	if m.Options == nil {
		t.Error("options were dropped")
	}
}

// Both shapes must yield the same normalised timestamp format, so a caller
// can't tell which endpoint a monitor came from.
func TestMonitorEpochTimestampsBecomeRFC3339(t *testing.T) {
	var m Monitor
	if err := json.Unmarshal([]byte(searchShapedMonitor), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// 1776777117 -> 2026-04-21T... in UTC
	if got := m.Created; len(got) < 20 || got[:4] != "2026" {
		t.Errorf("created = %q, want an RFC3339 timestamp in 2026", got)
	}
	if got := m.Modified; len(got) < 20 || got[:4] != "2026" {
		t.Errorf("modified = %q, want an RFC3339 timestamp", got)
	}
}

// A search response filtered by --status must actually filter, which requires
// Status to be populated from the search spelling.
func TestSearchShapedMonitorsAreFilterable(t *testing.T) {
	var resp MonitorSearchResponse
	raw := `{"monitors": [` + searchShapedMonitor + `], "counts": {"muted": [{"name": false, "count": 1}]}}`
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := filterMonitorsByStatus(resp.Monitors, "no_data"); len(got) != 1 {
		t.Errorf("--status no_data matched %d of a search response, want 1", len(got))
	}
	if got := filterMonitorsByStatus(resp.Monitors, "alert"); len(got) != 0 {
		t.Errorf("--status alert matched %d, want 0", len(got))
	}
}
