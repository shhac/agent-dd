package api

import "testing"

// Datadog returns overall_state in title case with spaces — "Alert", "OK",
// "No Data", "Warn". The CLI documents --status as alert|warn|ok|no_data.
// Those never matched, so `monitors list --status alert` silently returned
// nothing against a real org for every release that shipped it. The mock
// fixtures used the CLI's lowercase spelling rather than Datadog's, so the
// whole test suite agreed with the bug.
//
// Match on a normalised form so both spellings work in either direction.

func TestFilterMonitorsByStatusMatchesDatadogsSpelling(t *testing.T) {
	monitors := []Monitor{
		{ID: 1, Status: "Alert"},
		{ID: 2, Status: "OK"},
		{ID: 3, Status: "No Data"},
		{ID: 4, Status: "Warn"},
	}

	cases := []struct {
		filter string
		wantID int
	}{
		// What the CLI has always documented.
		{"alert", 1},
		{"ok", 2},
		{"no_data", 3},
		{"warn", 4},
		// What Datadog actually returns — a user copying a state back in.
		{"Alert", 1},
		{"OK", 2},
		{"No Data", 3},
		{"Warn", 4},
		// Plausible spellings of the same thing.
		{"ALERT", 1},
		{"no data", 3},
		{"No_Data", 3},
	}

	for _, tc := range cases {
		got := filterMonitorsByStatus(monitors, tc.filter)
		if len(got) != 1 {
			t.Errorf("--status %q matched %d monitors, want exactly 1", tc.filter, len(got))
			continue
		}
		if got[0].ID != tc.wantID {
			t.Errorf("--status %q matched monitor %d, want %d", tc.filter, got[0].ID, tc.wantID)
		}
	}
}

func TestFilterMonitorsByStatusEmptyReturnsAll(t *testing.T) {
	monitors := []Monitor{{ID: 1, Status: "Alert"}, {ID: 2, Status: "OK"}}

	if got := filterMonitorsByStatus(monitors, ""); len(got) != 2 {
		t.Errorf("empty --status returned %d monitors, want all 2", len(got))
	}
}

func TestFilterMonitorsByStatusUnknownMatchesNothing(t *testing.T) {
	monitors := []Monitor{{ID: 1, Status: "Alert"}}

	if got := filterMonitorsByStatus(monitors, "definitely-not-a-state"); len(got) != 0 {
		t.Errorf("unknown --status matched %d monitors, want 0", len(got))
	}
}
